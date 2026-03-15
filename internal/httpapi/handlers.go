package httpapi

import (
    "crypto/rand"
    "encoding/hex"
    "encoding/json"
    "errors"
    "io"
    "net"
    "net/http"
    "os"
    "strings"
    "time"

    "github.com/GuilhermeSoares009/multi-tenant-feature-flag-saas/internal/flags"
)

const (
    maxBodySize     = 1 << 20
    latencyBudgetMs = 100
)

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
    traceID := newID()
    writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
    logRequest(logEntry{
        Message:   "health check",
        TraceID:   traceID,
        TenantID:  "",
        FlagKey:   "",
        Action:    "health",
        Status:    http.StatusOK,
        Path:      r.URL.Path,
        Method:    r.Method,
        BudgetMs:  latencyBudgetMs,
        DurationMs: 0,
    })
}

func (s *Server) handleUpsert(w http.ResponseWriter, r *http.Request) {
    start := time.Now()
    traceID := newID()

    if r.Method != http.MethodPost {
        writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
        logRequest(logEntry{
            Message:   "method not allowed",
            TraceID:   traceID,
            TenantID:  "",
            FlagKey:   "",
            Action:    "upsert",
            Status:    http.StatusMethodNotAllowed,
            Path:      r.URL.Path,
            Method:    r.Method,
            BudgetMs:  latencyBudgetMs,
            DurationMs: 0,
        })
        return
    }

    var payload upsertFlagRequest
    if err := readJSON(r, &payload); err != nil {
        writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
        logRequest(logEntry{
            Message:   "invalid request",
            TraceID:   traceID,
            TenantID:  "",
            FlagKey:   "",
            Action:    "upsert",
            Status:    http.StatusBadRequest,
            Path:      r.URL.Path,
            Method:    r.Method,
            BudgetMs:  latencyBudgetMs,
            DurationMs: 0,
        })
        return
    }

    if err := payload.Validate(); err != nil {
        writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
        logRequest(logEntry{
            Message:   "validation failed",
            TraceID:   traceID,
            TenantID:  payload.TenantID,
            FlagKey:   payload.Key,
            Action:    "upsert",
            Status:    http.StatusBadRequest,
            Path:      r.URL.Path,
            Method:    r.Method,
            BudgetMs:  latencyBudgetMs,
            DurationMs: 0,
        })
        return
    }

    stored := s.store.UpsertFlag(payload.TenantID, flags.Flag{
        Key:         payload.Key,
        Description: payload.Description,
        Enabled:     payload.Enabled,
        Rollout:     payload.Rollout,
    })

    writeJSON(w, http.StatusOK, toFlagResponse(payload.TenantID, stored))

    durationMs := time.Since(start).Milliseconds()
    logRequest(logEntry{
        Message:        "flag upserted",
        TraceID:        traceID,
        TenantID:       payload.TenantID,
        FlagKey:        payload.Key,
        Action:         "upsert",
        Status:         http.StatusOK,
        Path:           r.URL.Path,
        Method:         r.Method,
        BudgetMs:       latencyBudgetMs,
        DurationMs:     durationMs,
        BudgetExceeded: durationMs > latencyBudgetMs,
    })
}

func (s *Server) handleEvaluate(w http.ResponseWriter, r *http.Request) {
    start := time.Now()
    traceID := newID()

    if r.Method != http.MethodPost {
        writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
        logRequest(logEntry{
            Message:   "method not allowed",
            TraceID:   traceID,
            TenantID:  "",
            FlagKey:   "",
            Action:    "evaluate",
            Status:    http.StatusMethodNotAllowed,
            Path:      r.URL.Path,
            Method:    r.Method,
            BudgetMs:  latencyBudgetMs,
            DurationMs: 0,
        })
        return
    }

    var payload evaluateFlagRequest
    if err := readJSON(r, &payload); err != nil {
        writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
        logRequest(logEntry{
            Message:   "invalid request",
            TraceID:   traceID,
            TenantID:  "",
            FlagKey:   "",
            Action:    "evaluate",
            Status:    http.StatusBadRequest,
            Path:      r.URL.Path,
            Method:    r.Method,
            BudgetMs:  latencyBudgetMs,
            DurationMs: 0,
        })
        return
    }

    if err := payload.Validate(); err != nil {
        writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
        logRequest(logEntry{
            Message:   "validation failed",
            TraceID:   traceID,
            TenantID:  payload.TenantID,
            FlagKey:   payload.FlagKey,
            Action:    "evaluate",
            Status:    http.StatusBadRequest,
            Path:      r.URL.Path,
            Method:    r.Method,
            BudgetMs:  latencyBudgetMs,
            DurationMs: 0,
        })
        return
    }

    evaluation, ok := s.store.Evaluate(payload.TenantID, payload.FlagKey, payload.EntityKey)
    if !ok {
        writeJSON(w, http.StatusNotFound, errorResponse{Error: "flag not found"})
        logRequest(logEntry{
            Message:   "flag not found",
            TraceID:   traceID,
            TenantID:  payload.TenantID,
            FlagKey:   payload.FlagKey,
            Action:    "evaluate",
            Status:    http.StatusNotFound,
            Path:      r.URL.Path,
            Method:    r.Method,
            BudgetMs:  latencyBudgetMs,
            DurationMs: 0,
        })
        return
    }

    writeJSON(w, http.StatusOK, evaluateResponse{
        TenantID: payload.TenantID,
        FlagKey:  payload.FlagKey,
        TraceID:  traceID,
        Result:   evaluation.Enabled,
        Reason:   evaluation.Reason,
        Rollout:  evaluation.Rollout,
        HashSlot: evaluation.HashSlot,
    })

    durationMs := time.Since(start).Milliseconds()
    logRequest(logEntry{
        Message:        "flag evaluated",
        TraceID:        traceID,
        TenantID:       payload.TenantID,
        FlagKey:        payload.FlagKey,
        Action:         "evaluate",
        Status:         http.StatusOK,
        Path:           r.URL.Path,
        Method:         r.Method,
        BudgetMs:       latencyBudgetMs,
        DurationMs:     durationMs,
        BudgetExceeded: durationMs > latencyBudgetMs,
    })
}

func (s *Server) handleFlags(w http.ResponseWriter, r *http.Request) {
	traceID := newID()
	tenantID := strings.TrimSpace(r.URL.Query().Get("tenantId"))
	if tenantID == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "tenantId is required"})
		logRequest(logEntry{
			Message:    "validation failed",
			TraceID:    traceID,
			TenantID:   "",
			FlagKey:    "",
			Action:     "flags",
			Status:     http.StatusBadRequest,
			Path:       r.URL.Path,
			Method:     r.Method,
			BudgetMs:   latencyBudgetMs,
			DurationMs: 0,
		})
		return
	}

	flagKey := strings.TrimSpace(r.URL.Query().Get("key"))
	switch r.Method {
	case http.MethodGet:
		if flagKey != "" {
			flag, ok := s.store.GetFlag(tenantID, flagKey)
			if !ok {
				writeJSON(w, http.StatusNotFound, errorResponse{Error: "flag not found"})
				return
			}
			writeJSON(w, http.StatusOK, toFlagResponse(tenantID, flag))
			return
		}
		flags := s.store.ListFlags(tenantID)
		response := flagsResponse{Flags: make([]flagResponse, 0, len(flags))}
		for _, current := range flags {
			response.Flags = append(response.Flags, toFlagResponse(tenantID, current))
		}
		writeJSON(w, http.StatusOK, response)
	case http.MethodDelete:
		if flagKey == "" {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "key is required"})
			return
		}
		if !s.store.DeleteFlag(tenantID, flagKey) {
			writeJSON(w, http.StatusNotFound, errorResponse{Error: "flag not found"})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
	}
}

func (s *Server) handleAudit(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
        return
    }

    tenantID := strings.TrimSpace(r.URL.Query().Get("tenantId"))
    limit := parseLimit(r.URL.Query().Get("limit"), 50)
	entries := s.store.Audit(tenantID, limit)
	writeJSON(w, http.StatusOK, auditResponse{Entries: entries})
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	writeJSON(w, http.StatusOK, s.metrics.Snapshot())
}

func readJSON(r *http.Request, dst any) error {
    decoder := json.NewDecoder(io.LimitReader(r.Body, maxBodySize))
    decoder.DisallowUnknownFields()
    if err := decoder.Decode(dst); err != nil {
        return err
    }
    if err := decoder.Decode(&struct{}{}); err != io.EOF {
        return errors.New("unexpected data after json body")
    }
    return nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    _ = json.NewEncoder(w).Encode(payload)
}

func clientIP(r *http.Request) string {
    if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
        parts := strings.Split(forwarded, ",")
        return strings.TrimSpace(parts[0])
    }
    if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
        return strings.TrimSpace(realIP)
    }
    host, _, err := net.SplitHostPort(r.RemoteAddr)
    if err == nil {
        return host
    }
    return r.RemoteAddr
}

func newID() string {
    bytes := make([]byte, 16)
    if _, err := rand.Read(bytes); err != nil {
        return hex.EncodeToString([]byte(time.Now().Format("20060102150405.000000")))
    }
    return hex.EncodeToString(bytes)
}

type logEntry struct {
    Message        string `json:"message"`
    TraceID        string `json:"traceId"`
    TenantID       string `json:"tenantId"`
    FlagKey        string `json:"flagKey"`
    Action         string `json:"action"`
    Status         int    `json:"status"`
    Path           string `json:"path"`
    Method         string `json:"method"`
    BudgetMs       int64  `json:"budgetMs"`
    DurationMs     int64  `json:"durationMs,omitempty"`
    BudgetExceeded bool   `json:"budgetExceeded,omitempty"`
}

func logRequest(entry logEntry) {
    data, err := json.Marshal(entry)
    if err != nil {
        return
    }
    _, _ = os.Stdout.Write(append(data, '\n'))
}
