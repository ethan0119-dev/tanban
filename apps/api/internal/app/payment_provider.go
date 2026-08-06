package app

type paymentProviderPresentation struct {
	DisplayName        string
	CheckoutMode       string
	AdapterImplemented bool
}

func paymentClientAction(providerName string, payParams map[string]string) string {
	if providerName == "mock" {
		return "MOCK_CONFIRM"
	}
	if len(payParams) == 0 {
		return "NONE"
	}
	if payParams["timeStamp"] != "" && payParams["nonceStr"] != "" && payParams["package"] != "" && payParams["signType"] != "" && payParams["paySign"] != "" {
		return "WECHAT_REQUEST_PAYMENT"
	}
	return "NONE"
}

func describePaymentProvider(providerName string) paymentProviderPresentation {
	switch providerName {
	case "wechat_partner":
		return paymentProviderPresentation{
			DisplayName:        "微信支付（普通服务商）",
			CheckoutMode:       "WECHAT_MINI_PROGRAM",
			AdapterImplemented: true,
		}
	case "tianque":
		return paymentProviderPresentation{
			DisplayName:        "会生活 · 随行付",
			CheckoutMode:       "HALF_SCREEN_CASHIER",
			AdapterImplemented: false,
		}
	case "lichu":
		return paymentProviderPresentation{
			DisplayName:        "利楚支付 · 扫呗",
			CheckoutMode:       "WECHAT_MINI_PROGRAM",
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
