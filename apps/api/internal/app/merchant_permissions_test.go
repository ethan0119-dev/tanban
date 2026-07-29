package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func capabilitySet(role string) map[string]bool {
	result := map[string]bool{}
	for _, capability := range merchantCapabilitiesForRole(role) {
		result[capability] = true
	}
	return result
}

func TestMerchantRoleCapabilityMatrix(t *testing.T) {
	owner := capabilitySet(RoleMerchantOwner)
	manager := capabilitySet(RoleMerchantManager)
	staff := capabilitySet(RoleMerchantStaff)

	for _, capability := range []merchantCapability{
		capabilityCreateRefunds,
		capabilityManageCatalog,
		capabilityManageMembers,
		capabilityManageStore,
	} {
		if !owner[string(capability)] || !manager[string(capability)] {
			t.Fatalf("%s must be granted to owner and manager", capability)
		}
		if staff[string(capability)] {
			t.Fatalf("%s must not be granted to staff", capability)
		}
	}

	for _, capability := range []merchantCapability{
		capabilityArchiveCustomers,
		capabilityAdjustCustomerBalance,
		capabilityCreateStoredValueRecord,
	} {
		if !owner[string(capability)] {
			t.Fatalf("%s must be granted to owner", capability)
		}
		if manager[string(capability)] || staff[string(capability)] {
			t.Fatalf("%s must be owner-only", capability)
		}
	}
}

func TestRequireMerchantCapabilityFailsClosed(t *testing.T) {
	handler := requireMerchantCapability(capabilityArchiveCustomers)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	for _, testCase := range []struct {
		role string
		want int
	}{
		{role: RoleMerchantOwner, want: http.StatusNoContent},
		{role: RoleMerchantManager, want: http.StatusForbidden},
		{role: RoleMerchantStaff, want: http.StatusForbidden},
		{role: "UNKNOWN", want: http.StatusForbidden},
	} {
		request := httptest.NewRequest(http.MethodDelete, "/customers/1", nil)
		request = request.WithContext(context.WithValue(request.Context(), identityKey{}, identity{Role: testCase.role}))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != testCase.want {
			t.Fatalf("role=%s status=%d want=%d body=%s", testCase.role, response.Code, testCase.want, response.Body.String())
		}
	}
}

func TestWorkspaceIdentityIncludesRoleCapabilities(t *testing.T) {
	user := workspaceIdentity(1, "manager", "店长", merchantWorkspace{
		MembershipID: 2,
		TenantID:     3,
		StoreID:      4,
		Role:         RoleMerchantManager,
	})
	if len(user.Capabilities) == 0 {
		t.Fatal("merchant identity must include capabilities")
	}
	if !roleHasMerchantCapability(user.Role, capabilityCreateRefunds) {
		t.Fatal("manager identity must include refund capability")
	}
	if roleHasMerchantCapability(user.Role, capabilityArchiveCustomers) {
		t.Fatal("manager identity must not include customer archive capability")
	}
}
