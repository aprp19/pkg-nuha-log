package accesslog

import "testing"

type mockRequestWithCode struct {
	RequestCode string
}

func (m *mockRequestWithCode) GetRequestCode() string {
	return m.RequestCode
}

func TestExtractRequestCode(t *testing.T) {
	req := &mockRequestWithCode{RequestCode: "CREATE_ORGANIZATION"}
	if got := extractRequestCode(req); got != "CREATE_ORGANIZATION" {
		t.Fatalf("extractRequestCode() = %q, want CREATE_ORGANIZATION", got)
	}

	if got := extractRequestCode(nil); got != "" {
		t.Fatalf("extractRequestCode(nil) = %q, want empty", got)
	}

	if got := extractRequestCode(struct{}{}); got != "" {
		t.Fatalf("extractRequestCode(no getter) = %q, want empty", got)
	}
}

func TestShortMethodName(t *testing.T) {
	got := shortMethodName("/organization.OrganizationService/ConnectOrganizationCrm")
	if got != "ConnectOrganizationCrm" {
		t.Fatalf("shortMethodName() = %q, want ConnectOrganizationCrm", got)
	}
}
