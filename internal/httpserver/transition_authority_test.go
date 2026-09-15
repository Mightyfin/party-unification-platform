package httpserver

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Mightyfin/party-unification-platform/internal/auth"
)

func TestCrossProgrammeTransitionRequiresExplicitAuthority(t *testing.T) {
	for _, mode := range []string{"ordinary", "tenant", "wrong-env", "missing-role", "missing-scope", "authorized"} {
		p := auth.Principal{Subject: "operator", Environment: "sandbox", Scopes: map[string]struct{}{"party.roles.transition": {}, "party.participation.transfer": {}}, Roles: map[string]struct{}{"party-participation-transfer": {}}}
		switch mode {
		case "ordinary":
			delete(p.Roles, "party-participation-transfer")
			delete(p.Scopes, "party.participation.transfer")
		case "tenant":
			p.TenantID = "green"
		case "wrong-env":
			p.Environment = "production"
		case "missing-role":
			delete(p.Roles, "party-participation-transfer")
		case "missing-scope":
			delete(p.Scopes, "party.participation.transfer")
		}
		mux := http.NewServeMux()
		registerLifecycleRoutes(mux, false, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
		// Invalid body proves an authorized actor reaches validation, not storage.
		r := httptest.NewRequest("POST", "/v1/parties/party/participation-transitions", strings.NewReader(`{"ParticipationTransferAuthorized":true}`))
		r.Header.Set("X-Acting-Tenant-Id", "green")
		r.Header.Set("X-Acting-Environment", "sandbox")
		r = r.WithContext(context.WithValue(r.Context(), principalKey{}, p))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		want := 403
		if mode == "authorized" {
			want = 400
		}
		if w.Code != want {
			t.Fatal(mode, w.Code, w.Body.String())
		}
	}
}
