package httpserver

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/Mightyfin/party-unification-platform/internal/auth"
	"github.com/Mightyfin/party-unification-platform/internal/party"
)

type readiness interface{ Ping(context.Context) error }

var safeValue = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,120}$`)

func New(address, environment string, authDisabled bool, verifier auth.Verifier, ready readiness, store *party.Store, logger *slog.Logger) *http.Server {
	mux := http.NewServeMux()
	registerLifecycleRoutes(mux, authDisabled, store, logger)
	mux.HandleFunc("GET /health/live", func(w http.ResponseWriter, r *http.Request) {
		write(w, 200, map[string]string{"status": "ok", "service": "party-platform", "environment": environment})
	})
	mux.HandleFunc("GET /health/ready", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if ready.Ping(ctx) != nil {
			problem(w, 503, "not_ready", "A required dependency is unavailable.")
			return
		}
		write(w, 200, map[string]string{"status": "ready"})
	})
	mux.HandleFunc("POST /v1/party-resolutions", func(w http.ResponseWriter, r *http.Request) {
		principal, ok := r.Context().Value(principalKey{}).(auth.Principal)
		if !ok || (!authDisabled && !principal.HasScope("party.resolve")) {
			problem(w, 403, "forbidden", "party.resolve scope is required.")
			return
		}
		var in party.ResolutionRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&in); err != nil {
			problem(w, 400, "invalid_json", "Request body is invalid.")
			return
		}
		in.TenantID = strings.TrimSpace(r.Header.Get("X-Acting-Tenant-Id"))
		in.Environment = strings.TrimSpace(r.Header.Get("X-Acting-Environment"))
		in.SourceSystem = strings.TrimSpace(in.SourceSystem)
		in.ExternalReference = strings.TrimSpace(in.ExternalReference)
		in.DisplayName = strings.TrimSpace(in.DisplayName)
		if !safeValue.MatchString(in.TenantID) || !safeValue.MatchString(in.ExternalReference) || (in.Environment != "sandbox" && in.Environment != "production") || (in.PartyType != "individual" && in.PartyType != "organization" && in.PartyType != "organizational_unit") || in.DisplayName == "" || len(in.DisplayName) > 200 || !safeValue.MatchString(in.SourceSystem) || !safeValue.MatchString(in.RoleType) || !safeValue.MatchString(in.Product) || in.Purpose == "" {
			problem(w, 400, "validation_failed", "Party resolution fields are invalid.")
			return
		}
		correlation := r.Header.Get("X-Correlation-Id")
		if correlation == "" {
			correlation = "cor_missing"
		}
		if principal.Subject == "" {
			principal.Subject = "local-development"
		}
		out, err := store.Resolve(r.Context(), party.Command{ActorSubject: principal.Subject, CorrelationID: correlation}, in)
		if err == party.ErrConflict {
			problem(w, 409, "alias_conflict", "The alias already resolves to a different party type.")
			return
		}
		if err == party.ErrInvalidRole {
			problem(w, 400, "invalid_role", "The role or product is not canonical.")
			return
		}
		if err != nil {
			logger.Error("party resolution failed", "error", err, "correlation_id", correlation)
			problem(w, 500, "internal_error", "Party resolution failed.")
			return
		}
		status := 201
		if out.Outcome == "matched" {
			status = 200
		}
		if out.Outcome == "review_required" {
			status = 202
		}
		write(w, status, out)
	})
	h := security(authenticate(authDisabled, verifier, mux))
	return &http.Server{Addr: address, Handler: h, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
}

type principalKey struct{}

func authenticate(disabled bool, v auth.Verifier, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/v1/") {
			next.ServeHTTP(w, r)
			return
		}
		if disabled {
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalKey{}, auth.Principal{Subject: "local-development", Scopes: map[string]struct{}{"party.resolve": {}, "party.roles.read": {}, "party.roles.write": {}, "party.roles.transition": {}, "party.relationships.read": {}, "party.relationships.write": {}}})))
			return
		}
		scheme, token, ok := strings.Cut(strings.TrimSpace(r.Header.Get("Authorization")), " ")
		if !ok || !strings.EqualFold(scheme, "Bearer") {
			problem(w, 401, "unauthorized", "A bearer token is required.")
			return
		}
		p, err := v.Verify(r.Context(), token)
		if err != nil {
			problem(w, 401, "unauthorized", "The bearer token is invalid.")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalKey{}, p)))
	})
}
func security(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}
func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func problem(w http.ResponseWriter, status int, code, message string) {
	write(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
