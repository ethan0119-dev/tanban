package app

import "net/http"

type merchantCapability string

const (
	capabilityViewDashboard           merchantCapability = "VIEW_DASHBOARD"
	capabilityViewFinancials          merchantCapability = "VIEW_FINANCIALS"
	capabilityManageOrders            merchantCapability = "MANAGE_ORDERS"
	capabilityOperatePrintJobs        merchantCapability = "OPERATE_PRINT_JOBS"
	capabilityViewNotifications       merchantCapability = "VIEW_NOTIFICATIONS"
	capabilityManageCatalog           merchantCapability = "MANAGE_CATALOG"
	capabilityManageStore             merchantCapability = "MANAGE_STORE"
	capabilityManageDecoration        merchantCapability = "MANAGE_DECORATION"
	capabilityManageMarketing         merchantCapability = "MANAGE_MARKETING"
	capabilityManageMembers           merchantCapability = "MANAGE_MEMBERS"
	capabilityViewPayments            merchantCapability = "VIEW_PAYMENTS"
	capabilityCreateRefunds           merchantCapability = "CREATE_REFUNDS"
	capabilityManageStaff             merchantCapability = "MANAGE_STAFF"
	capabilityManageAllStaff          merchantCapability = "MANAGE_ALL_STAFF"
	capabilityArchiveCustomers        merchantCapability = "ARCHIVE_CUSTOMERS"
	capabilityAdjustCustomerBalance   merchantCapability = "ADJUST_CUSTOMER_BALANCE"
	capabilityCreateStoredValueRecord merchantCapability = "CREATE_STORED_VALUE_RECORD"
)

var commonMerchantCapabilities = []merchantCapability{
	capabilityViewDashboard,
	capabilityManageOrders,
	capabilityOperatePrintJobs,
	capabilityViewNotifications,
}

var managementMerchantCapabilities = []merchantCapability{
	capabilityViewFinancials,
	capabilityManageCatalog,
	capabilityManageStore,
	capabilityManageDecoration,
	capabilityManageMarketing,
	capabilityManageMembers,
	capabilityViewPayments,
	capabilityCreateRefunds,
	capabilityManageStaff,
}

func merchantCapabilitiesForRole(role string) []string {
	capabilities := append([]merchantCapability{}, commonMerchantCapabilities...)
	switch role {
	case RoleMerchantOwner:
		capabilities = append(capabilities, managementMerchantCapabilities...)
		capabilities = append(capabilities,
			capabilityManageAllStaff,
			capabilityArchiveCustomers,
			capabilityAdjustCustomerBalance,
			capabilityCreateStoredValueRecord,
		)
	case RoleMerchantManager:
		capabilities = append(capabilities, managementMerchantCapabilities...)
	case RoleMerchantStaff:
		// Common operational capabilities only.
	default:
		return []string{}
	}

	result := make([]string, len(capabilities))
	for index, capability := range capabilities {
		result[index] = string(capability)
	}
	return result
}

func roleHasMerchantCapability(role string, required merchantCapability) bool {
	for _, capability := range merchantCapabilitiesForRole(role) {
		if capability == string(required) {
			return true
		}
	}
	return false
}

func requireMerchantCapability(required merchantCapability) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !roleHasMerchantCapability(currentIdentity(r.Context()).Role, required) {
				writeError(w, http.StatusForbidden, "FORBIDDEN", "insufficient permission")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
