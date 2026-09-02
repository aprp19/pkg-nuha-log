package accesslog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"google.golang.org/grpc/metadata"
)

func TestParseUserID(t *testing.T) {
	tests := []struct {
		val  interface{}
		want int
		ok   bool
	}{
		{7, 7, true},
		{float64(7), 7, true},
		{"42", 42, true},
		{"abc", 0, false},
		{nil, 0, false},
	}

	for _, tt := range tests {
		got, ok := parseUserID(tt.val)
		if ok != tt.ok || got != tt.want {
			t.Fatalf("parseUserID(%v) = (%d, %v), want (%d, %v)", tt.val, got, ok, tt.want, tt.ok)
		}
	}
}

func TestMergeActorBaseWins(t *testing.T) {
	baseID := 1
	extraID := 2
	baseEmail := "base@example.com"
	extraEmail := "extra@example.com"

	base := Actor{UserID: &baseID, UserEmail: &baseEmail}
	extra := Actor{UserID: &extraID, UserEmail: &extraEmail, UserName: strPtr("Extra")}

	merged := mergeActor(base, extra)

	if merged.UserID == nil || *merged.UserID != 1 {
		t.Fatalf("mergeActor() user_id = %v, want 1", merged.UserID)
	}
	if merged.UserEmail == nil || *merged.UserEmail != "base@example.com" {
		t.Fatalf("mergeActor() user_email = %v, want base@example.com", merged.UserEmail)
	}
	if merged.UserName == nil || *merged.UserName != "Extra" {
		t.Fatalf("mergeActor() user_name = %v, want Extra", merged.UserName)
	}
}

func TestActorFromContextKeys(t *testing.T) {
	ctx := context.Background()
	ctx = context.WithValue(ctx, "userID", 7)
	ctx = context.WithValue(ctx, "email", "admin@example.com")
	ctx = context.WithValue(ctx, "name", "Admin User")

	actor := actorFromContextKeys(ctx)

	if actor.UserID == nil || *actor.UserID != 7 {
		t.Fatalf("user_id = %v, want 7", actor.UserID)
	}
	if actor.UserEmail == nil || *actor.UserEmail != "admin@example.com" {
		t.Fatalf("user_email = %v", actor.UserEmail)
	}
	if actor.UserName == nil || *actor.UserName != "Admin User" {
		t.Fatalf("user_name = %v", actor.UserName)
	}
}

func TestActorFromClaimsNuhaAuthStyle(t *testing.T) {
	claims := map[string]interface{}{
		"userID": float64(7),
		"email":  "admin@example.com",
		"name":   "Admin User",
	}

	actor := actorFromClaims(claims)

	if actor.UserID == nil || *actor.UserID != 7 {
		t.Fatalf("user_id = %v, want 7", actor.UserID)
	}
	if actor.UserEmail == nil || *actor.UserEmail != "admin@example.com" {
		t.Fatalf("user_email = %v", actor.UserEmail)
	}
	if actor.UserName == nil || *actor.UserName != "Admin User" {
		t.Fatalf("user_name = %v", actor.UserName)
	}
}

func TestDefaultGRPCActorExtractorClaimsFallback(t *testing.T) {
	ctx := context.WithValue(context.Background(), "claims", map[string]interface{}{
		"userID": float64(9),
		"email":  "claims@example.com",
		"name":   "Claims User",
	})

	actor := defaultGRPCActorExtractor(ctx)

	if actor.UserID == nil || *actor.UserID != 9 {
		t.Fatalf("user_id = %v, want 9", actor.UserID)
	}
	if actor.UserEmail == nil || *actor.UserEmail != "claims@example.com" {
		t.Fatalf("user_email = %v", actor.UserEmail)
	}
	if actor.UserName == nil || *actor.UserName != "Claims User" {
		t.Fatalf("user_name = %v", actor.UserName)
	}
}

func TestDefaultGRPCActorExtractorFloat64UserID(t *testing.T) {
	ctx := context.WithValue(context.Background(), "userID", float64(7))
	ctx = context.WithValue(ctx, "email", "admin@example.com")

	actor := defaultGRPCActorExtractor(ctx)

	if actor.UserID == nil || *actor.UserID != 7 {
		t.Fatalf("user_id = %v, want 7", actor.UserID)
	}
}

func TestTenantHubIDFromMetadata(t *testing.T) {
	md := metadata.Pairs("x-tenant-hub-id", "hub-tenant-99")
	if got := tenantHubIDFromMetadata(md); got != "hub-tenant-99" {
		t.Fatalf("tenantHubIDFromMetadata() = %q, want hub-tenant-99", got)
	}

	md = metadata.Pairs("tenant-hub-id", "primary-tenant")
	if got := tenantHubIDFromMetadata(md); got != "primary-tenant" {
		t.Fatalf("tenantHubIDFromMetadata() = %q, want primary-tenant", got)
	}
}

func TestEnrichActorFromResponseUID(t *testing.T) {
	response := map[string]interface{}{
		"data": map[string]interface{}{
			"user": map[string]interface{}{
				"uid":   "550e8400-e29b-41d4-a716-446655440000",
				"email": "admin@example.com",
			},
		},
	}

	actor := enrichActorFromResponse(Actor{}, response)

	if actor.UserUUID == nil || *actor.UserUUID != "550e8400-e29b-41d4-a716-446655440000" {
		t.Fatalf("user_uuid = %v", actor.UserUUID)
	}
	if actor.UserEmail == nil || *actor.UserEmail != "admin@example.com" {
		t.Fatalf("user_email = %v", actor.UserEmail)
	}
}

func TestCaptureActorFromRequestEchoContext(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/users/1", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("userID", "42")
	c.Set("email", "gateway@example.com")

	actor := captureActorFromRequest(c)

	if actor.UserID == nil || *actor.UserID != 42 {
		t.Fatalf("user_id = %v, want 42", actor.UserID)
	}
	if actor.UserEmail == nil || *actor.UserEmail != "gateway@example.com" {
		t.Fatalf("user_email = %v", actor.UserEmail)
	}
}
