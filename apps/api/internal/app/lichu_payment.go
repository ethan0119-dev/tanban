package app

import (
	"net/http"

	"github.com/ethan0119-dev/tanban/apps/api/internal/provider"
)

// lichuCallback deliberately fails closed until the adapter can verify the
// key_sign with the terminal access_token and reconcile the referenced order.
func (s *Server) lichuCallback(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusServiceUnavailable, "PAYMENT_PROVIDER_NOT_CONFIGURED", provider.ErrNotConfigured.Error())
}
