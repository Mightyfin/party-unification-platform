package party

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestCanonicalLookupTenantEnvironmentAndStatus(t *testing.T) {
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
	_, err = db.Exec(ctx, `CREATE TEMP TABLE parties(id text,public_id text,party_type text,display_name text,status text);
CREATE TEMP TABLE party_aliases(party_id text,tenant_id text,environment text,status text);
CREATE TEMP TABLE party_roles(party_id text,tenant_id text,environment text,status text,effective_to timestamptz,effective_from timestamptz DEFAULT now());
INSERT INTO parties VALUES ('p','pty_test','individual','Synthetic owner','provisional');
INSERT INTO party_aliases VALUES ('p','green','sandbox','active');`)
	if err != nil {
		t.Fatal(err)
	}
	s := NewStore(db, nil)
	r, err := s.GetForTenant(ctx, "pty_test", "green", "sandbox")
	if err != nil || r.ID != "pty_test" || r.PartyID != r.ID || r.Status != "provisional" {
		t.Fatal(r, err)
	}
	for _, args := range [][3]string{{"pty_test", "other", "sandbox"}, {"pty_test", "green", "production"}, {"missing", "green", "sandbox"}} {
		if _, err := s.GetForTenant(ctx, args[0], args[1], args[2]); !errors.Is(err, ErrNotFound) {
			t.Fatal("scope leaked", args, err)
		}
	}
	_, err = db.Exec(ctx, `UPDATE party_aliases SET status='inactive'; INSERT INTO party_roles(party_id,tenant_id,environment,status,effective_to) VALUES('p','green','sandbox','active',now()-interval '1 day');`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetForTenant(ctx, "pty_test", "green", "sandbox"); !errors.Is(err, ErrNotFound) {
		t.Fatal("expired relationship accepted", err)
	}
	if _, err = db.Exec(ctx, `UPDATE party_roles SET effective_to=now()+interval '1 day'`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetForTenant(ctx, "pty_test", "green", "sandbox"); err != nil {
		t.Fatal("active relationship missing", err)
	}
	if _, err = db.Exec(ctx, `UPDATE party_roles SET effective_from=now()+interval '1 hour'`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetForTenant(ctx, "pty_test", "green", "sandbox"); !errors.Is(err, ErrNotFound) {
		t.Fatal("future relationship accepted", err)
	}
	if _, err = db.Exec(ctx, `UPDATE party_roles SET effective_from=now()`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `UPDATE parties SET status='closed'`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetForTenant(ctx, "pty_test", "green", "sandbox"); !errors.Is(err, ErrNotFound) {
		t.Fatal("closed party returned", err)
	}
}
