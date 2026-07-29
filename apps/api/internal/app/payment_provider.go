package app

type paymentProviderPresentation struct {
	DisplayName        string
	CheckoutMode       string
	AdapterImplemented bool
}

func describePaymentProvider(providerName string) paymentProviderPresentation {
	switch providerName {
	case "wechat_partner":
		return paymentProviderPresentation{
			DisplayName:        "微信支付（普通服务商）",
			CheckoutMode:       "WECHAT_MINI_PROGRAM",
			AdapterImplemented: false,
		}
	case "tianque":
		return paymentProviderPresentation{
			DisplayName:        "会生活 · 随行付",
			CheckoutMode:       "HALF_SCREEN_CASHIER",
			AdapterImplemented: false,
		}
	default:
		return paymentProviderPresentation{
			DisplayName:        "模拟支付（开发环境）",
			CheckoutMode:       "MOCK",
			AdapterImplemented: true,
		}
	}
}
