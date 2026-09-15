package party

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	migrations "github.com/Mightyfin/party-unification-platform/internal/database"
	"github.com/jackc/pgx/v5"
)

// Everything, including the schema, rolls back. Never run against a live DB.
func TestLifecycleDatabaseRegression(t *testing.T) {
	dsn := os.Getenv("PARTY_LOOKUP_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("disposable PostgreSQL required")
	}
	ctx := context.Background()
	db, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close(ctx)
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	schema := pgx.Identifier{newID("party_test")}.Sanitize()
	if _, err = tx.Exec(ctx, "CREATE SCHEMA "+schema+"; SET LOCAL search_path TO "+schema+",public"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"00001_party_registry.sql", "00002_role_relationship_lifecycle.sql"} {
		b, err := migrations.Migrations.ReadFile("migrations/" + name)
		if err != nil {
			t.Fatal(err)
		}
		up := strings.Split(string(b), "-- +goose Down")[0]
		if _, err = tx.Exec(ctx, up); err != nil {
			t.Fatal(name, err)
		}
	}
	s := NewStore(tx, []byte("synthetic-test-only-key"))
	cmd := Command{ActorSubject: "operator", CorrelationID: "test"}
	scope := Scope{"green", "sandbox"}
	resolve := func(ref string) Resolution {
		r, err := s.Resolve(ctx, cmd, ResolutionRequest{TenantID: scope.TenantID, Environment: scope.Environment, SourceSystem: "synthetic", ExternalReference: ref, PartyType: "individual", DisplayName: "Synthetic participant", RoleType: "buyer", Product: "EFAAS", Purpose: "UAT"})
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	one, two := resolve("one"), resolve("two")
	if r := resolve("one"); r.PartyID != one.PartyID || r.Outcome != "matched" {
		t.Fatal("resolution retry changed identity")
	}
	roles, err := s.ListRoles(ctx, scope, one.PartyID)
	if err != nil || len(roles) != 1 {
		t.Fatal(roles, err)
	}
	role := roles[0]
	if role.OnboardingStatus != "PENDING" || role.VerificationLevel != "UNVERIFIED" {
		t.Fatal("identity resolution manufactured an approval", role.OnboardingStatus, role.VerificationLevel)
	}
	for _, action := range []string{"suspend", "activate"} {
		r, err := s.ChangeRole(ctx, cmd, scope, role.ID, LifecycleRequest{Action: action, Reason: "Synthetic lifecycle test"})
		if err != nil || r.ID != role.ID {
			t.Fatal(action, err)
		}
	}
	if _, err := s.ChangeRole(ctx, cmd, Scope{"foreign", "sandbox"}, role.ID, LifecycleRequest{Action: "end", Reason: "Forbidden cross tenant"}); !errors.Is(err, ErrNotFound) {
		t.Fatal("cross-tenant role change", err)
	}
	extra := RoleRequest{RoleType: "SUPPLIER", Product: "EFAAS", Classification: "TEST", Channel: "PARTNER", OnboardingStatus: "PENDING", VerificationLevel: "UNVERIFIED"}
	if _, err := s.AssignRole(ctx, cmd, scope, one.PartyID, extra); err != nil {
		t.Fatal("valid role assignment", err)
	}
	rel, err := s.CreateRelationship(ctx, cmd, scope, one.PartyID, RelationshipRequest{ToPartyID: two.PartyID, RelationshipType: "SUPPLIES", Classification: "TEST"})
	if err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"suspend", "activate", "end"} {
		if _, err := s.ChangeRelationship(ctx, cmd, scope, rel.ID, LifecycleRequest{Action: action, Reason: "Synthetic relation test"}); err != nil {
			t.Fatal(action, err)
		}
	}
	if _, err := s.ChangeRelationship(ctx, cmd, scope, rel.ID, LifecycleRequest{Action: "activate", Reason: "Cannot reopen ended relation"}); !errors.Is(err, ErrInvalidTransition) {
		t.Fatal("ended relationship reopened", err)
	}
	transfer := ParticipationTransitionRequest{SourceRoleID: role.ID, SourceTenantID: scope.TenantID, TargetProduct: "EMBEDDED_FINANCE", TargetClassification: "DIRECT_TEST", ConsentReference: "synthetic-consent", EligibilityReference: "synthetic-eligibility", Reason: "Authorised synthetic transfer"}
	cmd.ParticipationTransferAuthorized = true
	// Incorrect party must roll back the source-role change.
	if _, err := s.TransitionParticipation(ctx, cmd, Scope{"direct", "sandbox"}, two.PartyID, transfer); !errors.Is(err, ErrInvalidTransition) {
		t.Fatal("wrong borrower accepted", err)
	}
	result, err := s.TransitionParticipation(ctx, cmd, Scope{"direct", "sandbox"}, one.PartyID, transfer)
	if err != nil {
		t.Fatal(err)
	}
	if result.EndedRole.Status != "ended" || result.ActivatedRole.PartyID != one.PartyID || result.ActivatedRole.TenantID != "direct" {
		t.Fatal("incorrect transition", result)
	}
	if result.ActivatedRole.OnboardingStatus != "PENDING" || result.ActivatedRole.VerificationLevel != "UNVERIFIED" {
		t.Fatal("transfer manufactured compliance approval")
	}
	var histories, events int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM party_history`).Scan(&histories); err != nil {
		t.Fatal(err)
	}
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM outbox_events`).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if histories != events || events != 11 {
		t.Fatal("audit/outbox not atomic or denied action recorded", histories, events)
	}
}
