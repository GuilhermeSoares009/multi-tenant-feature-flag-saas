package httpapi

import (
    "net/http"
    "time"

    "github.com/GuilhermeSoares009/multi-tenant-feature-flag-saas/internal/flags"
    "github.com/GuilhermeSoares009/multi-tenant-feature-flag-saas/internal/ratelimit"
)

type Server struct {
	store   *flags.Store
	limiter *ratelimit.Limiter
	metrics *metricsStore
	mux     *http.ServeMux
}

func NewServer(store *flags.Store, limiter *ratelimit.Limiter) *Server {
	server := &Server{
		store:   store,
		limiter: limiter,
		metrics: newMetricsStore(),
		mux:     http.NewServeMux(),
	}
    server.routes()
    return server
}

func (s *Server) routes() {
    s.mux.HandleFunc("/api/v1/health", s.handleHealth)
	s.mux.HandleFunc("/api/v1/flags/upsert", s.handleUpsert)
	s.mux.HandleFunc("/api/v1/flags", s.handleFlags)
	s.mux.HandleFunc("/api/v1/flags/evaluate", s.handleEvaluate)
	s.mux.HandleFunc("/api/v1/audit", s.handleAudit)
	s.mux.HandleFunc("/api/v1/metrics", s.handleMetrics)
}

func (s *Server) Handler() http.Handler {
	return s.withMetrics(s.withRateLimit(s.mux))
}

func (s *Server) withRateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.limiter.Allow(clientIP(r), time.Now()) {
			writeJSON(w, http.StatusTooManyRequests, errorResponse{Error: "rate limit exceeded"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) withMetrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		durationMs := time.Since(start).Milliseconds()
		s.metrics.Record(r.URL.Path, durationMs, durationMs > latencyBudgetMs)
	})
}
