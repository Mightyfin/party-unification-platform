package httpserver

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/Mightyfin/party-unification-platform/internal/auth"
	"github.com/Mightyfin/party-unification-platform/internal/party"
)

func registerLifecycleRoutes(mux *http.ServeMux, authDisabled bool, store *party.Store, logger *slog.Logger) {
	mux.HandleFunc("GET /v1/parties/{party_id}/roles", func(w http.ResponseWriter, r *http.Request) {
		principal, scope, ok := authorizeLifecycle(w, r, authDisabled, "party.roles.read")
		if !ok {
			return
		}
		_ = principal
		items, err := store.ListRoles(r.Context(), scope, r.PathValue("party_id"))
		if lifecycleError(w, r, logger, err) {
			return
		}
		write(w, http.StatusOK, map[string]any{"data": items})
	})
	mux.HandleFunc("POST /v1/parties/{party_id}/roles", func(w http.ResponseWriter, r *http.Request) {
		principal, scope, ok := authorizeLifecycle(w, r, authDisabled, "party.roles.write")
		if !ok {
			return
		}
		var in party.RoleRequest
		if !decode(w, r, &in) || !party.ValidateRole(in) {
			problem(w, 400, "validation_failed", "Role assignment is invalid.")
			return
		}
		out, err := store.AssignRole(r.Context(), command(r, principal), scope, r.PathValue("party_id"), in)
		if lifecycleError(w, r, logger, err) {
			return
		}
		write(w, http.StatusCreated, out)
	})
	mux.HandleFunc("POST /v1/party-roles/{role_id}/lifecycle", func(w http.ResponseWriter, r *http.Request) {
		principal, scope, ok := authorizeLifecycle(w, r, authDisabled, "party.roles.write")
		if !ok {
			return
		}
		var in party.LifecycleRequest
		if !decode(w, r, &in) {
			return
		}
		in.Action = strings.ToLower(strings.TrimSpace(in.Action))
		in.Reason = strings.TrimSpace(in.Reason)
		out, err := store.ChangeRole(r.Context(), command(r, principal), scope, r.PathValue("role_id"), in)
		if lifecycleError(w, r, logger, err) {
			return
		}
		write(w, http.StatusOK, out)
	})
	mux.HandleFunc("POST /v1/parties/{party_id}/relationships", func(w http.ResponseWriter, r *http.Request) {
		principal, scope, ok := authorizeLifecycle(w, r, authDisabled, "party.relationships.write")
		if !ok {
			return
		}
		var in party.RelationshipRequest
		if !decode(w, r, &in) || !party.ValidateRelationship(in) {
			problem(w, 400, "validation_failed", "Party relationship is invalid.")
			return
		}
		out, err := store.CreateRelationship(r.Context(), command(r, principal), scope, r.PathValue("party_id"), in)
		if lifecycleError(w, r, logger, err) {
			return
		}
		write(w, http.StatusCreated, out)
	})
	mux.HandleFunc("GET /v1/parties/{party_id}/relationships", func(w http.ResponseWriter, r *http.Request) {
		_, scope, ok := authorizeLifecycle(w, r, authDisabled, "party.relationships.read")
		if !ok {
			return
		}
		items, err := store.ListRelationships(r.Context(), scope, r.PathValue("party_id"))
		if lifecycleError(w, r, logger, err) {
			return
		}
		write(w, http.StatusOK, map[string]any{"data": items})
	})
	mux.HandleFunc("POST /v1/party-relationships/{relationship_id}/lifecycle", func(w http.ResponseWriter, r *http.Request) {
		principal, scope, ok := authorizeLifecycle(w, r, authDisabled, "party.relationships.write")
		if !ok {
			return
		}
		var in party.LifecycleRequest
		if !decode(w, r, &in) {
			return
		}
		in.Action = strings.ToLower(strings.TrimSpace(in.Action))
		in.Reason = strings.TrimSpace(in.Reason)
		out, err := store.ChangeRelationship(r.Context(), command(r, principal), scope, r.PathValue("relationship_id"), in)
		if lifecycleError(w, r, logger, err) {
			return
		}
		write(w, http.StatusOK, out)
	})
	mux.HandleFunc("POST /v1/parties/{party_id}/participation-transitions", func(w http.ResponseWriter, r *http.Request) {
		principal, scope, ok := authorizeLifecycle(w, r, authDisabled, "party.roles.transition")
		if !ok {
			return
		}
		var in party.ParticipationTransitionRequest
		if !decode(w, r, &in) {
			return
		}
		out, err := store.TransitionParticipation(r.Context(), command(r, principal), scope, r.PathValue("party_id"), in)
		if lifecycleError(w, r, logger, err) {
			return
		}
		write(w, http.StatusCreated, out)
	})
}

func authorizeLifecycle(w http.ResponseWriter, r *http.Request, disabled bool, required string) (auth.Principal, party.Scope, bool) {
	p, ok := r.Context().Value(principalKey{}).(auth.Principal)
	if !ok || (!disabled && !p.HasScope(required)) {
		problem(w, 403, "forbidden", required+" scope is required.")
		return auth.Principal{}, party.Scope{}, false
	}
	s := party.Scope{TenantID: strings.TrimSpace(r.Header.Get("X-Acting-Tenant-Id")), Environment: strings.TrimSpace(r.Header.Get("X-Acting-Environment"))}
	if !safeValue.MatchString(s.TenantID) || (s.Environment != "sandbox" && s.Environment != "production") {
		problem(w, 400, "invalid_scope", "Acting tenant and environment are required.")
		return auth.Principal{}, party.Scope{}, false
	}
	return p, s, true
}
func command(r *http.Request, p auth.Principal) party.Command {
	if p.Subject == "" {
		p.Subject = "local-development"
	}
	cor := r.Header.Get("X-Correlation-Id")
	if cor == "" {
		cor = "cor_missing"
	}
	return party.Command{ActorSubject: p.Subject, CorrelationID: cor}
}
func decode(w http.ResponseWriter, r *http.Request, out any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(out); err != nil {
		problem(w, 400, "invalid_json", "Request body is invalid.")
		return false
	}
	return true
}
func lifecycleError(w http.ResponseWriter, r *http.Request, logger *slog.Logger, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, party.ErrNotFound):
		problem(w, 404, "not_found", "The party resource was not found in this scope.")
	case errors.Is(err, party.ErrConflict):
		problem(w, 409, "conflict", "An active role or relationship already exists in this scope.")
	case errors.Is(err, party.ErrInvalidTransition):
		problem(w, 409, "invalid_transition", "The requested lifecycle transition is not allowed.")
	default:
		logger.Error("party lifecycle operation failed", "error", err, "path", r.URL.Path)
		problem(w, 500, "internal_error", "The party lifecycle operation failed.")
	}
	return true
}
