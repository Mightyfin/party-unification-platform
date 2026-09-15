package party

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestMembershipAndCrossTenantWrites(t *testing.T) {
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
	_, err = db.Exec(ctx, `CREATE TEMP TABLE parties(id text,public_id text,status text);
CREATE TEMP TABLE party_aliases(party_id text,tenant_id text,environment text,status text);
CREATE TEMP TABLE party_roles(party_id text,tenant_id text,environment text,status text,effective_from timestamptz,effective_to timestamptz);
INSERT INTO parties VALUES('a','pty_a','active'),('b','pty_b','active'),('c','pty_c','restricted');
INSERT INTO party_aliases VALUES('a','green','sandbox','active'),('b','other','sandbox','active'),('c','green','sandbox','active');`)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		tenant, env, id string
		allowed         bool
	}{
		{"green", "sandbox", "pty_a", true}, {"green", "production", "pty_a", false}, {"other", "sandbox", "pty_a", false}, {"green", "sandbox", "pty_b", false}, {"green", "sandbox", "pty_c", false},
	} {
		tx, err := db.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		err = requireMembership(ctx, tx, Scope{tc.tenant, tc.env}, tc.id)
		tx.Rollback(ctx)
		if (err == nil) != tc.allowed {
			t.Fatal(tc, err)
		}
	}
	s := NewStore(db, nil)
	cmd := Command{ActorSubject: "operator"}
	scope := Scope{"green", "sandbox"}
	in := RoleRequest{RoleType: "CUSTOMER", Product: "EFAAS", Classification: "BUYER", Channel: "PARTNER", OnboardingStatus: "PENDING", VerificationLevel: "UNVERIFIED"}
	if _, err := s.AssignRole(ctx, cmd, scope, "pty_b", in); !errors.Is(err, ErrNotFound) {
		t.Fatal("foreign role write not stopped", err)
	}
	if _, err := s.CreateRelationship(ctx, cmd, scope, "pty_a", RelationshipRequest{ToPartyID: "pty_b", RelationshipType: "SUPPLIES", Classification: "TEST"}); !errors.Is(err, ErrNotFound) {
		t.Fatal("foreign relationship not stopped", err)
	}
	_, err = db.Exec(ctx, `UPDATE party_aliases SET status='inactive' WHERE party_id='a'; INSERT INTO party_roles VALUES('a','green','sandbox','active',now()-interval '1 day',now()+interval '1 day');`)
	if err != nil {
		t.Fatal(err)
	}
	tx, _ := db.Begin(ctx)
	err = requireMembership(ctx, tx, scope, "pty_a")
	tx.Rollback(ctx)
	if err != nil {
		t.Fatal("current role not accepted", err)
	}
	_, err = db.Exec(ctx, `UPDATE party_roles SET effective_from=now()+interval '1 hour'`)
	if err != nil {
		t.Fatal(err)
	}
	tx, _ = db.Begin(ctx)
	err = requireMembership(ctx, tx, scope, "pty_a")
	tx.Rollback(ctx)
	if !errors.Is(err, ErrNotFound) {
		t.Fatal("future role accepted", err)
	}
}
