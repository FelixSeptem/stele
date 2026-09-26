package auth

import (
	"context"
	"testing"
)

func TestContextWithPrincipalCarriesAuthenticatedPrincipal(t *testing.T) {
	want := Principal{ID: "principal-1", Role: PrincipalRoleAdmin, Status: PrincipalStatusActive}
	ctx := ContextWithPrincipal(context.Background(), want)
	got, ok := PrincipalFromContext(ctx)
	if !ok || got.ID != want.ID || got.Role != want.Role || got.Status != want.Status {
		t.Fatalf("PrincipalFromContext() = %+v, %v; want %+v", got, ok, want)
	}
}
