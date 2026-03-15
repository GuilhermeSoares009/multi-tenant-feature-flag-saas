package flags

import (
	"hash/fnv"
	"sort"
	"sync"
	"time"
)

type Flag struct {
    Key         string
    Description string
    Enabled     bool
    Rollout     int
    UpdatedAt   time.Time
}

type Evaluation struct {
    Enabled  bool
    Reason   string
    Rollout  int
    HashSlot int
}

type AuditEntry struct {
    Timestamp time.Time
    TenantID  string
    FlagKey   string
    Action    string
    Detail    string
}

type Store struct {
	mu    sync.RWMutex
	flags map[string]map[string]Flag
	audit []AuditEntry
	cache *ConfigCache
}

func NewStore() *Store {
	return &Store{
		flags: make(map[string]map[string]Flag),
		audit: make([]AuditEntry, 0, 200),
		cache: NewConfigCache(30 * time.Second),
	}
}

func (s *Store) UpsertFlag(tenantID string, flag Flag) Flag {
    s.mu.Lock()
    defer s.mu.Unlock()

    if s.flags[tenantID] == nil {
        s.flags[tenantID] = make(map[string]Flag)
    }
	flag.UpdatedAt = time.Now().UTC()
	s.flags[tenantID][flag.Key] = flag
	if s.cache != nil {
		s.cache.Set(tenantID, flag, time.Now().UTC())
	}
	s.appendAudit(AuditEntry{
		Timestamp: flag.UpdatedAt,
        TenantID:  tenantID,
        FlagKey:   flag.Key,
        Action:    "upsert",
        Detail:    "flag updated",
    })
    return flag
}

func (s *Store) GetFlag(tenantID, flagKey string) (Flag, bool) {
	if s.cache != nil {
		if cached, ok := s.cache.Get(tenantID, flagKey, time.Now().UTC()); ok {
			return cached, true
		}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	tenantFlags := s.flags[tenantID]
	if tenantFlags == nil {
		return Flag{}, false
	}
	flag, ok := tenantFlags[flagKey]
	if ok && s.cache != nil {
		s.cache.Set(tenantID, flag, time.Now().UTC())
	}
	return flag, ok
}

func (s *Store) ListFlags(tenantID string) []Flag {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tenantFlags := s.flags[tenantID]
	if tenantFlags == nil {
		return []Flag{}
	}
	result := make([]Flag, 0, len(tenantFlags))
	for _, flag := range tenantFlags {
		result = append(result, flag)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Key < result[j].Key
	})
	return result
}

func (s *Store) DeleteFlag(tenantID, flagKey string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	tenantFlags := s.flags[tenantID]
	if tenantFlags == nil {
		return false
	}
	if _, ok := tenantFlags[flagKey]; !ok {
		return false
	}
	delete(tenantFlags, flagKey)
	if len(tenantFlags) == 0 {
		delete(s.flags, tenantID)
	}
	if s.cache != nil {
		s.cache.Delete(tenantID, flagKey)
	}
	s.appendAudit(AuditEntry{
		Timestamp: time.Now().UTC(),
		TenantID:  tenantID,
		FlagKey:   flagKey,
		Action:    "delete",
		Detail:    "flag deleted",
	})
	return true
}

func (s *Store) Evaluate(tenantID, flagKey, entityKey string) (Evaluation, bool) {
	flag, ok := s.GetFlag(tenantID, flagKey)
	if !ok {
		return Evaluation{}, false
	}

    evaluation := evaluateFlag(flag, tenantID, entityKey)
    s.mu.Lock()
    s.appendAudit(AuditEntry{
        Timestamp: time.Now().UTC(),
        TenantID:  tenantID,
        FlagKey:   flagKey,
        Action:    "evaluate",
        Detail:    evaluation.Reason,
    })
    s.mu.Unlock()
    return evaluation, true
}

func (s *Store) Audit(tenantID string, limit int) []AuditEntry {
    s.mu.RLock()
    defer s.mu.RUnlock()

    if limit <= 0 {
        limit = 50
    }
    result := make([]AuditEntry, 0, limit)
    for i := len(s.audit) - 1; i >= 0 && len(result) < limit; i-- {
        entry := s.audit[i]
        if tenantID == "" || entry.TenantID == tenantID {
            result = append(result, entry)
        }
    }
    return result
}

func (s *Store) getFlagLocked(tenantID, flagKey string) (Flag, bool) {
    tenantFlags := s.flags[tenantID]
    if tenantFlags == nil {
        return Flag{}, false
    }
    flag, ok := tenantFlags[flagKey]
    return flag, ok
}

func (s *Store) appendAudit(entry AuditEntry) {
    s.audit = append(s.audit, entry)
    if len(s.audit) > 1000 {
        s.audit = s.audit[len(s.audit)-1000:]
    }
}

func evaluateFlag(flag Flag, tenantID, entityKey string) Evaluation {
    if !flag.Enabled {
        return Evaluation{
            Enabled: false,
            Reason:  "flag-disabled",
            Rollout: flag.Rollout,
        }
    }
    if flag.Rollout >= 100 {
        return Evaluation{
            Enabled: true,
            Reason:  "rollout-100",
            Rollout: flag.Rollout,
            HashSlot: 0,
        }
    }
    if flag.Rollout <= 0 {
        return Evaluation{
            Enabled: false,
            Reason:  "rollout-0",
            Rollout: flag.Rollout,
            HashSlot: 0,
        }
    }

    slot := hashSlot(tenantID, flag.Key, entityKey)
    enabled := slot < flag.Rollout
    reason := "rollout-match"
    if !enabled {
        reason = "rollout-miss"
    }
    return Evaluation{
        Enabled:  enabled,
        Reason:   reason,
        Rollout:  flag.Rollout,
        HashSlot: slot,
    }
}

func hashSlot(tenantID, flagKey, entityKey string) int {
    h := fnv.New32a()
    _, _ = h.Write([]byte(tenantID))
    _, _ = h.Write([]byte("|"))
    _, _ = h.Write([]byte(flagKey))
    _, _ = h.Write([]byte("|"))
    _, _ = h.Write([]byte(entityKey))
    return int(h.Sum32() % 100)
}
