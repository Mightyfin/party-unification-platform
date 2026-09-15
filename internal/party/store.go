package party

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

var (
	ErrConflict = errors.New("party alias conflicts with request")
	ErrNotFound = errors.New("party not found")
)

type database interface {
	Begin(context.Context) (pgx.Tx, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}
type Store struct {
	db  database
	key []byte
}

func NewStore(db database, key []byte) *Store { return &Store{db: db, key: key} }

type Identifier struct {
	Scheme             string `json:"scheme"`
	Value              string `json:"value"`
	VerificationStatus string `json:"verification_status"`
}
type ResolutionRequest struct {
	TenantID          string       `json:"tenant_id"`
	Environment       string       `json:"environment"`
	SourceSystem      string       `json:"source_system"`
	ExternalReference string       `json:"external_reference"`
	PartyType         string       `json:"party_type"`
	DisplayName       string       `json:"display_name"`
	RoleType          string       `json:"role_type"`
	Product           string       `json:"product"`
	Purpose           string       `json:"purpose"`
	Identifiers       []Identifier `json:"identifiers,omitempty"`
}
type Resolution struct {
	PartyID  string  `json:"party_id"`
	AliasID  string  `json:"alias_id"`
	Outcome  string  `json:"outcome"`
	ReviewID *string `json:"review_id,omitempty"`
}

// Record is the minimum identity evidence a downstream domain needs. It has no
// identifiers, aliases, wallet balances or lending information.
type Record struct {
	ID          string `json:"id"` // compatibility for existing document consumers
	PartyID     string `json:"party_id"`
	PartyType   string `json:"party_type"`
	DisplayName string `json:"display_name"`
	Status      string `json:"status"`
}
type Command struct{ ActorSubject, CorrelationID string }

// GetForTenant exposes a canonical party only where it has an active tenant
// alias or role. Global Party IDs must never bypass tenant/environment access.
func (s *Store) GetForTenant(ctx context.Context, partyID, tenantID, environment string) (Record, error) {
	var record Record
	err := s.db.QueryRow(ctx, `
SELECT p.public_id,p.party_type,p.display_name,p.status
FROM parties p
WHERE p.public_id=$1
  AND p.status IN ('provisional','active','restricted')
  AND (
    EXISTS (SELECT 1 FROM party_aliases a WHERE a.party_id=p.id AND a.tenant_id=$2 AND a.environment=$3 AND a.status='active')
    OR EXISTS (SELECT 1 FROM party_roles r WHERE r.party_id=p.id AND r.tenant_id=$2 AND r.environment=$3 AND r.status='active'
               AND r.effective_from<=now()
               AND (r.effective_to IS NULL OR r.effective_to>now()))
  )`, partyID, tenantID, environment).Scan(&record.PartyID, &record.PartyType, &record.DisplayName, &record.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return Record{}, ErrNotFound
	}
	if err == nil { record.ID = record.PartyID }
	return record, err
}

func (s *Store) Resolve(ctx context.Context, cmd Command, in ResolutionRequest) (Resolution, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Resolution{}, err
	}
	defer tx.Rollback(ctx)
	var existing Resolution
	var existingType string
	err = tx.QueryRow(ctx, `SELECT p.public_id,a.public_id,p.party_type FROM party_aliases a JOIN parties p ON p.id=a.party_id WHERE a.tenant_id=$1 AND a.environment=$2 AND a.source_system=$3 AND a.external_reference=$4`, in.TenantID, in.Environment, in.SourceSystem, in.ExternalReference).Scan(&existing.PartyID, &existing.AliasID, &existingType)
	if err == nil {
		if existingType != in.PartyType {
			return Resolution{}, ErrConflict
		}
		existing.Outcome = "matched"
		return existing, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Resolution{}, err
	}

	candidates := map[string]struct{}{}
	type tokenized struct {
		Identifier
		Token string
	}
	tokens := make([]tokenized, 0, len(in.Identifiers))
	for _, identifier := range in.Identifiers {
		identifier.Scheme = strings.ToLower(strings.TrimSpace(identifier.Scheme))
		identifier.Value = strings.ToUpper(strings.Join(strings.Fields(identifier.Value), ""))
		if identifier.VerificationStatus != "verified" {
			continue
		}
		token := s.token(identifier.Scheme, identifier.Value)
		tokens = append(tokens, tokenized{identifier, token})
		var id string
		e := tx.QueryRow(ctx, `SELECT p.public_id FROM party_identifiers i JOIN parties p ON p.id=i.party_id WHERE i.scheme=$1 AND i.value_token=$2`, identifier.Scheme, token).Scan(&id)
		if e == nil {
			candidates[id] = struct{}{}
		} else if !errors.Is(e, pgx.ErrNoRows) {
			return Resolution{}, e
		}
	}
	if len(candidates) > 1 {
		reviewID := newID("mrv")
		ids := make([]string, 0, len(candidates))
		for id := range candidates {
			ids = append(ids, id)
		}
		_, err = tx.Exec(ctx, `INSERT INTO match_reviews(public_id,tenant_id,environment,source_system,external_reference,candidate_party_ids,reason) SELECT $1,$2,$3,$4,$5,array_agg(id),'verified identifiers resolve to different parties' FROM parties WHERE public_id=ANY($6)`, reviewID, in.TenantID, in.Environment, in.SourceSystem, in.ExternalReference, ids)
		if err != nil {
			return Resolution{}, err
		}
		if err = tx.Commit(ctx); err != nil {
			return Resolution{}, err
		}
		return Resolution{Outcome: "review_required", ReviewID: &reviewID}, nil
	}
	partyID, outcome := newID("pty"), "created"
	if len(candidates) == 1 {
		for id := range candidates {
			partyID = id
		}
		outcome = "matched"
	} else {
		_, err = tx.Exec(ctx, `INSERT INTO parties(public_id,party_type,display_name) VALUES($1,$2,$3)`, partyID, in.PartyType, in.DisplayName)
		if err != nil {
			return Resolution{}, err
		}
	}
	aliasID := newID("pal")
	_, err = tx.Exec(ctx, `INSERT INTO party_aliases(public_id,party_id,tenant_id,environment,source_system,external_reference) SELECT $1,id,$2,$3,$4,$5 FROM parties WHERE public_id=$6`, aliasID, in.TenantID, in.Environment, in.SourceSystem, in.ExternalReference, partyID)
	if err != nil {
		return Resolution{}, err
	}
	for _, id := range tokens {
		_, err = tx.Exec(ctx, `INSERT INTO party_identifiers(party_id,scheme,value_token,verification_status,source_system,purpose) SELECT id,$1,$2,'verified',$3,$4 FROM parties WHERE public_id=$5 ON CONFLICT(scheme,value_token) DO NOTHING`, id.Scheme, id.Token, in.SourceSystem, in.Purpose, partyID)
		if err != nil {
			return Resolution{}, err
		}
	}
	_, err = tx.Exec(ctx, `INSERT INTO party_roles(party_id,tenant_id,environment,role_type,product) SELECT id,$1,$2,$3,$4 FROM parties WHERE public_id=$5 ON CONFLICT DO NOTHING`, in.TenantID, in.Environment, in.RoleType, in.Product, partyID)
	if err != nil {
		return Resolution{}, err
	}
	payload, _ := json.Marshal(map[string]any{"party_id": partyID, "alias_id": aliasID, "outcome": outcome, "source_system": in.SourceSystem})
	_, err = tx.Exec(ctx, `INSERT INTO party_history(public_id,party_id,action,actor_subject,tenant_id,correlation_id,payload) SELECT $1,id,'party.resolved',$2,$3,$4,$5 FROM parties WHERE public_id=$6`, newID("his"), cmd.ActorSubject, in.TenantID, cmd.CorrelationID, payload, partyID)
	if err != nil {
		return Resolution{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO outbox_events(public_id,aggregate_id,event_type,tenant_id,correlation_id,payload) VALUES($1,$2,'party.resolved.v1',$3,$4,$5)`, newID("evt"), partyID, in.TenantID, cmd.CorrelationID, payload)
	if err != nil {
		return Resolution{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Resolution{}, err
	}
	return Resolution{PartyID: partyID, AliasID: aliasID, Outcome: outcome}, nil
}
func (s *Store) token(scheme, value string) string {
	h := hmac.New(sha256.New, s.key)
	h.Write([]byte(scheme))
	h.Write([]byte{0})
	h.Write([]byte(value))
	return hex.EncodeToString(h.Sum(nil))
}
func newID(prefix string) string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(b[:]))
}
