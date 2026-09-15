package auth

import (
	"encoding/json"
	"testing"
)

func TestCanonicalSubjectClaim(t *testing.T) {
	var c claims
	if err := json.Unmarshal([]byte(`{"sub":"service-account","scope":"party.read"}`), &c); err != nil || c.Subject != "service-account" || c.Scope != "party.read" {
		t.Fatal("deployed OIDC subject mapping lost", err)
	}
}
