package party

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
)

// Lock the membership evidence used by a write in that same transaction. A
// caller cannot create access merely by knowing a global canonical party ID.
func requireMembership(ctx context.Context, tx pgx.Tx, scope Scope, ids ...string) error {
	if strings.TrimSpace(scope.TenantID) == "" || (scope.Environment != "sandbox" && scope.Environment != "production" && scope.Environment != "local") {
		return ErrNotFound
	}
	ordered := append([]string(nil), ids...)
	sort.Strings(ordered)
	for _, id := range ordered {
		var found bool
		err := tx.QueryRow(ctx, `SELECT true FROM party_aliases a JOIN parties p ON p.id=a.party_id
WHERE p.public_id=$1 AND p.status IN ('provisional','active') AND a.tenant_id=$2 AND a.environment=$3 AND a.status='active'
LIMIT 1 FOR SHARE OF a,p`, id, scope.TenantID, scope.Environment).Scan(&found)
		if errors.Is(err, pgx.ErrNoRows) {
			err = tx.QueryRow(ctx, `SELECT true FROM party_roles r JOIN parties p ON p.id=r.party_id
WHERE p.public_id=$1 AND p.status IN ('provisional','active') AND r.tenant_id=$2 AND r.environment=$3 AND r.status='active'
AND r.effective_from<=now() AND (r.effective_to IS NULL OR r.effective_to>now()) LIMIT 1 FOR SHARE OF r,p`, id, scope.TenantID, scope.Environment).Scan(&found)
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
	}
	return nil
}
