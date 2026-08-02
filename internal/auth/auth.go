package auth

import (
	"context"
	"fmt"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
)

type Principal struct { Subject string; Scopes map[string]struct{} }
func (p Principal) HasScope(s string) bool { _, ok := p.Scopes[s]; return ok }
type Verifier interface { Verify(context.Context, string) (Principal, error) }
type OIDCVerifier struct { verifier *oidc.IDTokenVerifier }
type claims struct { Subject, Scope string }
func New(ctx context.Context, issuer, audience string) (*OIDCVerifier, error) {
	p, err := oidc.NewProvider(ctx, issuer); if err != nil { return nil, err }
	return &OIDCVerifier{verifier:p.Verifier(&oidc.Config{ClientID:audience})}, nil
}
func (v *OIDCVerifier) Verify(ctx context.Context, raw string) (Principal, error) {
	t, err := v.verifier.Verify(ctx, raw); if err != nil { return Principal{}, err }
	var c claims; if err=t.Claims(&c); err != nil || c.Subject=="" { return Principal{}, fmt.Errorf("invalid claims") }
	p := Principal{Subject:c.Subject, Scopes:map[string]struct{}{}}
	for _, s := range strings.Fields(c.Scope) { p.Scopes[s]=struct{}{} }
	return p, nil
}
