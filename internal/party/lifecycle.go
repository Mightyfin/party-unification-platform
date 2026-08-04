package party

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var ErrNotFound = errors.New("party resource not found")
var ErrInvalidTransition = errors.New("invalid lifecycle transition")

var validRoles = map[string]bool{"CUSTOMER": true, "PARTNER": true, "SUPPLIER": true, "EMPLOYEE": true, "AGENT": true, "INVESTOR_FUNDER": true, "REGULATOR": true}
var validProducts = map[string]bool{"DIRECT_LENDING": true, "EMBEDDED_FINANCE": true, "EFAAS": true, "PAYMENTS": true, "WALLET": true, "INTERNAL": true, "EXTERNAL": true}
var validRelationships = map[string]bool{"EMPLOYED_BY": true, "SUPPLIES": true, "AGGREGATED_BY": true, "MEMBER_OF": true, "REPRESENTS": true, "FUNDS": true, "REGULATES": true, "SERVES": true, "INTEGRATES_WITH": true}

type Scope struct{ TenantID, Environment string }
type RoleRequest struct {
	RoleType             string `json:"role_type"`
	Product              string `json:"product"`
	Classification       string `json:"classification"`
	Channel              string `json:"channel"`
	OnboardingStatus     string `json:"onboarding_status"`
	VerificationLevel    string `json:"verification_level"`
	ConsentReference     string `json:"consent_reference,omitempty"`
	EligibilityReference string `json:"eligibility_reference,omitempty"`
}
type PartyRole struct {
	ID                   string     `json:"id"`
	PartyID              string     `json:"party_id"`
	TenantID             string     `json:"tenant_id"`
	Environment          string     `json:"environment"`
	RoleType             string     `json:"role_type"`
	Product              string     `json:"product"`
	Classification       string     `json:"classification"`
	Channel              string     `json:"channel"`
	Status               string     `json:"status"`
	OnboardingStatus     string     `json:"onboarding_status"`
	VerificationLevel    string     `json:"verification_level"`
	ConsentReference     *string    `json:"consent_reference,omitempty"`
	EligibilityReference *string    `json:"eligibility_reference,omitempty"`
	EffectiveFrom        time.Time  `json:"effective_from"`
	EffectiveTo          *time.Time `json:"effective_to,omitempty"`
}
type LifecycleRequest struct {
	Action string `json:"action"`
	Reason string `json:"reason"`
}
type RelationshipRequest struct {
	ToPartyID        string `json:"to_party_id"`
	RelationshipType string `json:"relationship_type"`
	Classification   string `json:"classification"`
}
type Relationship struct {
	ID               string     `json:"id"`
	FromPartyID      string     `json:"from_party_id"`
	ToPartyID        string     `json:"to_party_id"`
	TenantID         string     `json:"tenant_id"`
	Environment      string     `json:"environment"`
	RelationshipType string     `json:"relationship_type"`
	Classification   string     `json:"classification"`
	Status           string     `json:"status"`
	EffectiveFrom    time.Time  `json:"effective_from"`
	EffectiveTo      *time.Time `json:"effective_to,omitempty"`
}
type ParticipationTransitionRequest struct {
	SourceRoleID         string `json:"source_role_id"`
	SourceTenantID       string `json:"source_tenant_id"`
	TargetProduct        string `json:"target_product"`
	TargetClassification string `json:"target_classification"`
	ConsentReference     string `json:"consent_reference"`
	EligibilityReference string `json:"eligibility_reference"`
	Reason               string `json:"reason"`
}
type ParticipationTransition struct {
	EndedRole     PartyRole `json:"ended_role"`
	ActivatedRole PartyRole `json:"activated_role"`
}

func ValidateRole(in RoleRequest) bool {
	return validRoles[in.RoleType] && validProducts[in.Product] && in.Classification != "" && (in.Channel == "DIRECT" || in.Channel == "PARTNER" || in.Channel == "INTERNAL" || in.Channel == "EXTERNAL") && (in.OnboardingStatus == "PENDING" || in.OnboardingStatus == "UNDER_REVIEW" || in.OnboardingStatus == "APPROVED" || in.OnboardingStatus == "REJECTED") && (in.VerificationLevel == "UNVERIFIED" || in.VerificationLevel == "IDENTITY_VERIFIED" || in.VerificationLevel == "PRODUCT_VERIFIED" || in.VerificationLevel == "ENHANCED_DUE_DILIGENCE")
}
func ValidateRelationship(in RelationshipRequest) bool {
	return validRelationships[in.RelationshipType] && in.Classification != "" && in.ToPartyID != ""
}

func (s *Store) AssignRole(ctx context.Context, cmd Command, scope Scope, partyID string, in RoleRequest) (PartyRole, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return PartyRole{}, err
	}
	defer tx.Rollback(ctx)
	id := newID("pro")
	var out PartyRole
	err = tx.QueryRow(ctx, `INSERT INTO party_roles(public_id,party_id,tenant_id,environment,role_type,product,classification,channel,onboarding_status,verification_level,consent_reference,eligibility_reference,created_by) SELECT $1,id,$2,$3,$4,$5,$6,$7,$8,$9,NULLIF($10,''),NULLIF($11,''),$12 FROM parties WHERE public_id=$13 RETURNING public_id,$13,tenant_id,environment,role_type,product,classification,channel,status,onboarding_status,verification_level,consent_reference,eligibility_reference,effective_from,effective_to`, id, scope.TenantID, scope.Environment, in.RoleType, in.Product, in.Classification, in.Channel, in.OnboardingStatus, in.VerificationLevel, in.ConsentReference, in.EligibilityReference, cmd.ActorSubject, partyID).Scan(&out.ID, &out.PartyID, &out.TenantID, &out.Environment, &out.RoleType, &out.Product, &out.Classification, &out.Channel, &out.Status, &out.OnboardingStatus, &out.VerificationLevel, &out.ConsentReference, &out.EligibilityReference, &out.EffectiveFrom, &out.EffectiveTo)
	if errors.Is(err, pgx.ErrNoRows) {
		return PartyRole{}, ErrNotFound
	}
	if uniqueViolation(err) {
		return PartyRole{}, ErrConflict
	}
	if err != nil {
		return PartyRole{}, err
	}
	if err = writeLifecycle(ctx, tx, cmd, scope, partyID, "party.role.assigned.v1", out); err != nil {
		return PartyRole{}, err
	}
	return out, tx.Commit(ctx)
}
func (s *Store) ListRoles(ctx context.Context, scope Scope, partyID string) ([]PartyRole, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `SELECT r.public_id,p.public_id,r.tenant_id,r.environment,r.role_type,r.product,r.classification,r.channel,r.status,r.onboarding_status,r.verification_level,r.consent_reference,r.eligibility_reference,r.effective_from,r.effective_to FROM party_roles r JOIN parties p ON p.id=r.party_id WHERE p.public_id=$1 AND r.tenant_id=$2 AND r.environment=$3 ORDER BY r.effective_from DESC`, partyID, scope.TenantID, scope.Environment)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PartyRole{}
	for rows.Next() {
		var v PartyRole
		if err = rows.Scan(&v.ID, &v.PartyID, &v.TenantID, &v.Environment, &v.RoleType, &v.Product, &v.Classification, &v.Channel, &v.Status, &v.OnboardingStatus, &v.VerificationLevel, &v.ConsentReference, &v.EligibilityReference, &v.EffectiveFrom, &v.EffectiveTo); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) ChangeRole(ctx context.Context, cmd Command, scope Scope, roleID string, in LifecycleRequest) (PartyRole, error) {
	next := map[string]string{"suspend": "suspended", "activate": "active", "end": "ended"}[in.Action]
	if next == "" || in.Reason == "" {
		return PartyRole{}, ErrInvalidTransition
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return PartyRole{}, err
	}
	defer tx.Rollback(ctx)
	out, err := changeRoleTx(ctx, tx, scope, roleID, next, in.Reason)
	if err != nil {
		return PartyRole{}, err
	}
	if err = writeLifecycle(ctx, tx, cmd, scope, out.PartyID, "party.role."+in.Action+"ed.v1", out); err != nil {
		return PartyRole{}, err
	}
	return out, tx.Commit(ctx)
}
func (s *Store) CreateRelationship(ctx context.Context, cmd Command, scope Scope, fromPartyID string, in RelationshipRequest) (Relationship, error) {
	if fromPartyID == in.ToPartyID {
		return Relationship{}, ErrInvalidTransition
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Relationship{}, err
	}
	defer tx.Rollback(ctx)
	id := newID("prl")
	var out Relationship
	err = tx.QueryRow(ctx, `INSERT INTO party_relationships(public_id,from_party_id,to_party_id,relationship_type,classification,tenant_id,environment,created_by) SELECT $1,f.id,t.id,$2,$3,$4,$5,$6 FROM parties f,parties t WHERE f.public_id=$7 AND t.public_id=$8 RETURNING public_id,$7,$8,tenant_id,environment,relationship_type,classification,status,effective_from,effective_to`, id, in.RelationshipType, in.Classification, scope.TenantID, scope.Environment, cmd.ActorSubject, fromPartyID, in.ToPartyID).Scan(&out.ID, &out.FromPartyID, &out.ToPartyID, &out.TenantID, &out.Environment, &out.RelationshipType, &out.Classification, &out.Status, &out.EffectiveFrom, &out.EffectiveTo)
	if errors.Is(err, pgx.ErrNoRows) {
		return Relationship{}, ErrNotFound
	}
	if uniqueViolation(err) {
		return Relationship{}, ErrConflict
	}
	if err != nil {
		return Relationship{}, err
	}
	if err = writeLifecycle(ctx, tx, cmd, scope, fromPartyID, "party.relationship.created.v1", out); err != nil {
		return Relationship{}, err
	}
	return out, tx.Commit(ctx)
}
func (s *Store) ListRelationships(ctx context.Context, scope Scope, partyID string) ([]Relationship, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `SELECT r.public_id,f.public_id,t.public_id,r.tenant_id,r.environment,r.relationship_type,r.classification,r.status,r.effective_from,r.effective_to FROM party_relationships r JOIN parties f ON f.id=r.from_party_id JOIN parties t ON t.id=r.to_party_id WHERE (f.public_id=$1 OR t.public_id=$1) AND r.tenant_id=$2 AND r.environment=$3 ORDER BY r.effective_from DESC`, partyID, scope.TenantID, scope.Environment)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Relationship{}
	for rows.Next() {
		var v Relationship
		if err = rows.Scan(&v.ID, &v.FromPartyID, &v.ToPartyID, &v.TenantID, &v.Environment, &v.RelationshipType, &v.Classification, &v.Status, &v.EffectiveFrom, &v.EffectiveTo); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) ChangeRelationship(ctx context.Context, cmd Command, scope Scope, id string, in LifecycleRequest) (Relationship, error) {
	next := map[string]string{"suspend": "suspended", "activate": "active", "end": "ended"}[in.Action]
	if next == "" || in.Reason == "" {
		return Relationship{}, ErrInvalidTransition
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Relationship{}, err
	}
	defer tx.Rollback(ctx)
	var current string
	if err = tx.QueryRow(ctx, `SELECT status FROM party_relationships WHERE public_id=$1 AND tenant_id=$2 AND environment=$3 FOR UPDATE`, id, scope.TenantID, scope.Environment).Scan(&current); errors.Is(err, pgx.ErrNoRows) {
		return Relationship{}, ErrNotFound
	} else if err != nil {
		return Relationship{}, err
	}
	if current == "ended" || current == next {
		return Relationship{}, ErrInvalidTransition
	}
	var out Relationship
	err = tx.QueryRow(ctx, `UPDATE party_relationships r SET status=$1::varchar,status_reason=$2,effective_to=CASE WHEN $1::varchar='ended' THEN now() ELSE NULL END,updated_at=now() FROM parties f,parties t WHERE r.from_party_id=f.id AND r.to_party_id=t.id AND r.public_id=$3 AND r.tenant_id=$4 AND r.environment=$5 RETURNING r.public_id,f.public_id,t.public_id,r.tenant_id,r.environment,r.relationship_type,r.classification,r.status,r.effective_from,r.effective_to`, next, in.Reason, id, scope.TenantID, scope.Environment).Scan(&out.ID, &out.FromPartyID, &out.ToPartyID, &out.TenantID, &out.Environment, &out.RelationshipType, &out.Classification, &out.Status, &out.EffectiveFrom, &out.EffectiveTo)
	if err != nil {
		return Relationship{}, err
	}
	if err = writeLifecycle(ctx, tx, cmd, scope, out.FromPartyID, "party.relationship."+in.Action+"ed.v1", out); err != nil {
		return Relationship{}, err
	}
	return out, tx.Commit(ctx)
}
func (s *Store) TransitionParticipation(ctx context.Context, cmd Command, scope Scope, partyID string, in ParticipationTransitionRequest) (ParticipationTransition, error) {
	if in.SourceRoleID == "" || in.SourceTenantID == "" || in.SourceTenantID == scope.TenantID || in.TargetProduct != "EMBEDDED_FINANCE" || in.TargetClassification == "" || in.ConsentReference == "" || in.EligibilityReference == "" || in.Reason == "" {
		return ParticipationTransition{}, ErrInvalidTransition
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return ParticipationTransition{}, err
	}
	defer tx.Rollback(ctx)
	sourceScope := Scope{TenantID: in.SourceTenantID, Environment: scope.Environment}
	ended, err := changeRoleTx(ctx, tx, sourceScope, in.SourceRoleID, "ended", in.Reason)
	if err != nil {
		return ParticipationTransition{}, err
	}
	if ended.PartyID != partyID || ended.RoleType != "CUSTOMER" || ended.Product != "EFAAS" {
		return ParticipationTransition{}, ErrInvalidTransition
	}
	var activated PartyRole
	id := newID("pro")
	err = tx.QueryRow(ctx, `INSERT INTO party_roles(public_id,party_id,tenant_id,environment,role_type,product,classification,channel,onboarding_status,verification_level,consent_reference,eligibility_reference,created_by) SELECT $1,id,$2,$3,'CUSTOMER','EMBEDDED_FINANCE',$4,'DIRECT','APPROVED','PRODUCT_VERIFIED',$5,$6,$7 FROM parties WHERE public_id=$8 RETURNING public_id,$8,tenant_id,environment,role_type,product,classification,channel,status,onboarding_status,verification_level,consent_reference,eligibility_reference,effective_from,effective_to`, id, scope.TenantID, scope.Environment, in.TargetClassification, in.ConsentReference, in.EligibilityReference, cmd.ActorSubject, partyID).Scan(&activated.ID, &activated.PartyID, &activated.TenantID, &activated.Environment, &activated.RoleType, &activated.Product, &activated.Classification, &activated.Channel, &activated.Status, &activated.OnboardingStatus, &activated.VerificationLevel, &activated.ConsentReference, &activated.EligibilityReference, &activated.EffectiveFrom, &activated.EffectiveTo)
	if err != nil {
		return ParticipationTransition{}, err
	}
	result := ParticipationTransition{EndedRole: ended, ActivatedRole: activated}
	if err = writeLifecycle(ctx, tx, cmd, sourceScope, partyID, "party.role.ended.v1", ended); err != nil {
		return ParticipationTransition{}, err
	}
	if err = writeLifecycle(ctx, tx, cmd, scope, partyID, "party.participation.transitioned.v1", result); err != nil {
		return ParticipationTransition{}, err
	}
	return result, tx.Commit(ctx)
}
func changeRoleTx(ctx context.Context, tx pgx.Tx, scope Scope, roleID, next, reason string) (PartyRole, error) {
	var current string
	if err := tx.QueryRow(ctx, `SELECT status FROM party_roles WHERE public_id=$1 AND tenant_id=$2 AND environment=$3 FOR UPDATE`, roleID, scope.TenantID, scope.Environment).Scan(&current); errors.Is(err, pgx.ErrNoRows) {
		return PartyRole{}, ErrNotFound
	} else if err != nil {
		return PartyRole{}, err
	}
	if current == "ended" || current == next {
		return PartyRole{}, ErrInvalidTransition
	}
	var out PartyRole
	err := tx.QueryRow(ctx, `UPDATE party_roles r SET status=$1::varchar,status_reason=$2,effective_to=CASE WHEN $1::varchar='ended' THEN now() ELSE NULL END,updated_at=now() FROM parties p WHERE r.party_id=p.id AND r.public_id=$3 AND r.tenant_id=$4 AND r.environment=$5 RETURNING r.public_id,p.public_id,r.tenant_id,r.environment,r.role_type,r.product,r.classification,r.channel,r.status,r.onboarding_status,r.verification_level,r.consent_reference,r.eligibility_reference,r.effective_from,r.effective_to`, next, reason, roleID, scope.TenantID, scope.Environment).Scan(&out.ID, &out.PartyID, &out.TenantID, &out.Environment, &out.RoleType, &out.Product, &out.Classification, &out.Channel, &out.Status, &out.OnboardingStatus, &out.VerificationLevel, &out.ConsentReference, &out.EligibilityReference, &out.EffectiveFrom, &out.EffectiveTo)
	return out, err
}
func writeLifecycle(ctx context.Context, tx pgx.Tx, cmd Command, scope Scope, partyID, event string, value any) error {
	payload, _ := json.Marshal(value)
	_, err := tx.Exec(ctx, `WITH p AS (SELECT id FROM parties WHERE public_id=$1),h AS (INSERT INTO party_history(public_id,party_id,action,actor_subject,tenant_id,correlation_id,payload) SELECT $2,id,$3,$4,$5,$6,$7 FROM p) INSERT INTO outbox_events(public_id,aggregate_id,event_type,tenant_id,correlation_id,payload) VALUES($8,$1,$3,$5,$6,$7)`, partyID, newID("his"), event, cmd.ActorSubject, scope.TenantID, cmd.CorrelationID, payload, newID("evt"))
	return err
}
func uniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
