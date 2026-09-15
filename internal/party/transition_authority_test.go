package party

import (
	"context"
	"errors"
	"testing"
)

func TestTransferWithoutAuthorityNeverTouchesDatabase(t *testing.T) {
	s := NewStore(nil, nil)
	for _, cmd := range []Command{{ActorSubject: "tenant"}, {ParticipationTransferAuthorized: true}} {
		_, err := s.TransitionParticipation(context.Background(), cmd, Scope{"target", "sandbox"}, "party", ParticipationTransitionRequest{SourceRoleID: "foreign-role", SourceTenantID: "source"})
		if !errors.Is(err, ErrTransferAuthority) {
			t.Fatal("unauthorized transfer reached storage", err)
		}
	}
}
