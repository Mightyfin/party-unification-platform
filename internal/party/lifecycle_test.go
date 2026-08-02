package party

import "testing"

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
