package party

import (
	"context"
	"errors"
	"testing"
)

func TestRoleAssignmentCannotSelfAttestCompliance(t *testing.T) {
	for _, status := range []string{"PENDING", "UNDER_REVIEW", "APPROVED", "REJECTED"} {
		for _, level := range []string{"UNVERIFIED", "IDENTITY_VERIFIED", "PRODUCT_VERIFIED", "ENHANCED_DUE_DILIGENCE"} {
			if status == "PENDING" && level == "UNVERIFIED" {
				continue // Positive database path is covered by the lifecycle regression.
			}
			in := RoleRequest{RoleType: "CUSTOMER", Product: "EFAAS", Classification: "BUYER", Channel: "PARTNER", OnboardingStatus: status, VerificationLevel: level, ConsentReference: "claimed-consent", EligibilityReference: "claimed-approval"}
			_, err := NewStore(nil, nil).AssignRole(context.Background(), Command{ActorSubject: "role-writer"}, Scope{"green", "sandbox"}, "party", in)
			if !errors.Is(err, ErrVerificationEvidenceRequired) {
				t.Fatalf("%s/%s: %v", status, level, err)
			}
		}
	}
}

func TestCanonicalRoleValidation(t *testing.T) {
	valid := RoleRequest{RoleType: "CUSTOMER", Product: "EMBEDDED_FINANCE", Classification: "DIRECT_CUSTOMER", Channel: "DIRECT", OnboardingStatus: "APPROVED", VerificationLevel: "PRODUCT_VERIFIED"}
	if !ValidateRole(valid) {
		t.Fatal("canonical customer role rejected")
	}
	valid.RoleType = "ADMIN_STAFF"
	if ValidateRole(valid) {
		t.Fatal("authorization role accepted as a business role")
	}
}

func TestRelationshipValidation(t *testing.T) {
	if !ValidateRelationship(RelationshipRequest{ToPartyID: "pty_target", RelationshipType: "SUPPLIES", Classification: "AGRICULTURAL_OUTPUT"}) {
		t.Fatal("canonical relationship rejected")
	}
	if ValidateRelationship(RelationshipRequest{ToPartyID: "pty_target", RelationshipType: "ADMINISTERS", Classification: "SYSTEM"}) {
		t.Fatal("authorization relationship accepted")
	}
}
