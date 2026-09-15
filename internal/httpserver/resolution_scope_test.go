package httpserver

import (
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Mightyfin/party-unification-platform/internal/auth"
)

type scopeVerifier struct{ principal auth.Principal }

func (v scopeVerifier) Verify(context.Context, string) (auth.Principal, error) {
	return v.principal, nil
}

func TestPrimaryRoutesRejectContradictoryCredentialScope(t *testing.T) {
	for _, route := range []struct{ method, path string }{{"GET", "/v1/parties/invalid!"}, {"POST", "/v1/party-resolutions"}} {
		for _, mode := range []string{"foreign-tenant", "foreign-environment", "matching", "service-reader"} {
			t.Run(route.method+"/"+mode, func(t *testing.T) {
				p := auth.Principal{Subject: "synthetic", TenantID: "green", Environment: "sandbox", Scopes: map[string]struct{}{"party.read": {}, "party.resolve": {}}}
				switch mode {
				case "foreign-tenant":
					p.TenantID = "other"
				case "foreign-environment":
					p.Environment = "production"
				case "service-reader":
					p.TenantID = ""
				}
				srv := New("", "sandbox", false, scopeVerifier{p}, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
				r := httptest.NewRequest(route.method, route.path, strings.NewReader(`{}`))
				r.Header.Set("Authorization", "Bearer synthetic")
				r.Header.Set("X-Acting-Tenant-Id", "green")
				r.Header.Set("X-Acting-Environment", "sandbox")
				w := httptest.NewRecorder()
				srv.Handler.ServeHTTP(w, r)
				want := 403
				if mode == "matching" || mode == "service-reader" {
					want = 400 // Valid authority reaches input validation; no database calls.
				}
				if w.Code != want {
					t.Fatalf("got %d: %s", w.Code, w.Body.String())
				}
			})
		}
	}
}
