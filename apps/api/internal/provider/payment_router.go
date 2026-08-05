package provider

import (
	"fmt"
	"sort"
	"strings"
)

// PaymentRouter keeps all payment adapters available in one API process.
// Selection is deliberately explicit: callers must use the provider snapshot
// from the tenant for a new payment, or from the transaction for every later
// operation such as query, close and refund.
type PaymentRouter struct {
	providers map[string]PaymentProvider
}

func NewPaymentRouter(providers ...PaymentProvider) *PaymentRouter {
	router := &PaymentRouter{providers: make(map[string]PaymentProvider, len(providers))}
	for _, paymentProvider := range providers {
		router.Register(paymentProvider)
	}
	return router
}

func (r *PaymentRouter) Register(paymentProvider PaymentProvider) {
	if r == nil || paymentProvider == nil {
		return
	}
	name := strings.ToLower(strings.TrimSpace(paymentProvider.Name()))
	if name == "" {
		return
	}
	r.providers[name] = paymentProvider
}

func (r *PaymentRouter) Resolve(name string) (PaymentProvider, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	if r == nil || name == "" {
		return nil, fmt.Errorf("%w: %s", ErrNotConfigured, name)
	}
	paymentProvider, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("%w: payment provider %s is not registered", ErrNotConfigured, name)
	}
	return paymentProvider, nil
}

func (r *PaymentRouter) Names() []string {
	if r == nil {
		return []string{}
	}
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
