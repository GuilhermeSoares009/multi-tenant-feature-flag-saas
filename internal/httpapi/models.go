package httpapi

import (
    "errors"
    "strconv"
    "strings"
    "time"

    "github.com/GuilhermeSoares009/multi-tenant-feature-flag-saas/internal/flags"
)

type upsertFlagRequest struct {
    TenantID    string `json:"tenantId"`
    Key         string `json:"key"`
    Description string `json:"description"`
    Enabled     bool   `json:"enabled"`
    Rollout     int    `json:"rollout"`
}

type evaluateFlagRequest struct {
    TenantID  string `json:"tenantId"`
    FlagKey   string `json:"flagKey"`
    EntityKey string `json:"entityKey"`
}

type flagResponse struct {
	TenantID    string    `json:"tenantId"`
	Key         string    `json:"key"`
	Description string    `json:"description"`
	Enabled     bool      `json:"enabled"`
	Rollout     int       `json:"rollout"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type flagsResponse struct {
	Flags []flagResponse `json:"flags"`
}

type evaluateResponse struct {
    TenantID string `json:"tenantId"`
    FlagKey  string `json:"flagKey"`
    TraceID  string `json:"traceId"`
    Result   bool   `json:"result"`
    Reason   string `json:"reason"`
    Rollout  int    `json:"rollout"`
    HashSlot int    `json:"hashSlot"`
}

type auditResponse struct {
    Entries []flags.AuditEntry `json:"entries"`
}

type errorResponse struct {
    Error string `json:"error"`
}

func (req upsertFlagRequest) Validate() error {
    if strings.TrimSpace(req.TenantID) == "" {
        return errors.New("tenantId is required")
    }
    if strings.TrimSpace(req.Key) == "" {
        return errors.New("key is required")
    }
    if req.Rollout < 0 || req.Rollout > 100 {
        return errors.New("rollout must be between 0 and 100")
    }
    return nil
}

func (req evaluateFlagRequest) Validate() error {
    if strings.TrimSpace(req.TenantID) == "" {
        return errors.New("tenantId is required")
    }
    if strings.TrimSpace(req.FlagKey) == "" {
        return errors.New("flagKey is required")
    }
    if strings.TrimSpace(req.EntityKey) == "" {
        return errors.New("entityKey is required")
    }
    return nil
}

func toFlagResponse(tenantID string, flag flags.Flag) flagResponse {
    return flagResponse{
        TenantID:    tenantID,
        Key:         flag.Key,
        Description: flag.Description,
        Enabled:     flag.Enabled,
        Rollout:     flag.Rollout,
        UpdatedAt:   flag.UpdatedAt,
    }
}

func parseLimit(raw string, fallback int) int {
    if raw == "" {
        return fallback
    }
    value, err := strconv.Atoi(raw)
    if err != nil || value <= 0 {
        return fallback
    }
    return value
}
