package app

import (
	"fmt"
	"strings"

	"github.com/ethan0119-dev/tanban/apps/api/internal/provider"
)

func (s *Server) resolvePaymentProvider(name string) (provider.PaymentProvider, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	if s.Payments != nil {
		return s.Payments.Resolve(name)
	}
	// Preserve small unit tests and embedders that construct Server directly.
	if s.Payment != nil && s.Payment.Name() == name {
		return s.Payment, nil
	}
	return nil, fmt.Errorf("%w: payment provider %s is unavailable", provider.ErrNotConfigured, name)
}

func (s *Server) paymentProviderAvailable(name string) bool {
	_, err := s.resolvePaymentProvider(name)
	return err == nil
}

func (s *Server) availablePaymentProviders() []string {
	if s.Payments != nil {
		return s.Payments.Names()
	}
	if s.Payment != nil {
		return []string{s.Payment.Name()}
	}
	return []string{}
}

func (s *Server) paymentNotifyURLFor(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "wechat_partner":
		return s.Config.WeChatPayPartner.NotifyURL
	case "tianque":
		return s.Config.TianQue.NotifyURL
	case "lichu":
		return s.Config.Lichu.NotifyURL
	default:
		return ""
	}
}
