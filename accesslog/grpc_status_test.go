package accesslog

import (
	"testing"

	"google.golang.org/grpc/codes"
)

func TestGRPCCodeToHTTPStatus(t *testing.T) {
	tests := []struct {
		code codes.Code
		want int
	}{
		{codes.OK, 200},
		{codes.InvalidArgument, 400},
		{codes.Unauthenticated, 401},
		{codes.NotFound, 404},
		{codes.Internal, 500},
		{codes.Unavailable, 503},
	}

	for _, tt := range tests {
		if got := grpcCodeToHTTPStatus(tt.code); got != tt.want {
			t.Fatalf("grpcCodeToHTTPStatus(%v) = %d, want %d", tt.code, got, tt.want)
		}
	}
}

func TestBuildHandlerName(t *testing.T) {
	got := buildHandlerName("/organization.OrganizationService/ConnectOrganizationCrm", "CREATE_ORGANIZATION")
	want := "ConnectOrganizationCrm/CREATE_ORGANIZATION"
	if got != want {
		t.Fatalf("buildHandlerName() = %q, want %q", got, want)
	}
}

func TestDefaultSkipMethods(t *testing.T) {
	if !defaultSkipMethods("/grpc.health.v1.Health/Check") {
		t.Fatal("expected health check to be skipped")
	}
	if defaultSkipMethods("/organization.OrganizationService/ConnectOrganizationCrm") {
		t.Fatal("expected business method not to be skipped")
	}
}
