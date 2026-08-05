package provider

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	wechatMicropayPath   = "/pay/micropay"
	wechatOrderQueryPath = "/pay/orderquery"
	wechatReversePath    = "/secapi/pay/reverse"
)

func (w *WeChatPayPartner) CodePaymentReady() (bool, string) {
	switch {
	case strings.TrimSpace(w.Config.ServiceProviderMchID) == "":
		return false, "平台尚未配置微信支付服务商商户号"
	case strings.TrimSpace(w.Config.ServiceProviderAppID) == "":
		return false, "平台尚未配置微信支付服务商 AppID"
	case len(strings.TrimSpace(w.Config.APIV2Key)) != 32:
		return false, "平台尚未配置 32 位微信支付 APIv2 密钥"
	case strings.TrimSpace(w.Config.ServerIP) == "":
		return false, "平台尚未配置付款码支付服务器公网 IP"
	case net.ParseIP(strings.TrimSpace(w.Config.ServerIP)).To4() == nil:
		return false, "微信付款码支付服务器 IP 必须是有效的 IPv4 地址"
	case strings.TrimSpace(w.Config.MerchantCertificate) == "":
		return false, "平台尚未配置微信支付 API 证书"
	case strings.TrimSpace(w.Config.MerchantPrivateKey) == "":
		return false, "平台尚未配置微信支付 API 证书私钥"
	default:
		if w.HTTPClient == nil {
			if _, err := loadWechatClientCertificate(w.Config.MerchantCertificate, w.Config.MerchantPrivateKey); err != nil {
				return false, "微信支付 API 证书或私钥无法读取"
			}
		}
		return true, ""
	}
}

func (w *WeChatPayPartner) PayCode(ctx context.Context, req CodePaymentRequest) (CodePaymentResult, error) {
	if ready, reason := w.CodePaymentReady(); !ready {
		return CodePaymentResult{}, fmt.Errorf("%w: %s", ErrNotConfigured, reason)
	}
	if !validWeChatAuthCode(req.AuthCode) {
		return CodePaymentResult{}, errors.New("invalid WeChat payment auth code")
	}
	if req.Amount <= 0 || req.MerchantNo == "" || req.OrderNo == "" {
		return CodePaymentResult{}, errors.New("merchant number, order number and positive amount are required")
	}
	serverIP := strings.TrimSpace(req.ServerIP)
	if serverIP == "" {
		serverIP = strings.TrimSpace(w.Config.ServerIP)
	}
	params := w.baseV2Params(req.MerchantNo, req.SubAppID)
	params["body"] = truncateUTF8(req.Description, 120)
	params["out_trade_no"] = req.OrderNo
	params["total_fee"] = strconv.FormatInt(req.Amount, 10)
	params["spbill_create_ip"] = serverIP
	params["auth_code"] = req.AuthCode
	if deviceID := strings.TrimSpace(req.DeviceID); deviceID != "" {
		params["device_info"] = truncateUTF8(deviceID, 32)
	}
	if sceneInfo := wechatStoreSceneInfo(req.StoreID, req.StoreName, req.StoreAddress); sceneInfo != "" {
		params["scene_info"] = sceneInfo
	}
	response, err := w.postV2(ctx, wechatMicropayPath, params, false)
	if err != nil {
		return CodePaymentResult{ProviderOrderNo: req.OrderNo, Status: PaymentPending, Message: "支付结果未知，正在查单"}, err
	}
	result := codePaymentResultFromV2(req.OrderNo, response)
	if result.Status == PaymentSuccess {
		result.PaidAt = parseWeChatTime(response["time_end"])
	}
	return result, nil
}

func (w *WeChatPayPartner) QueryCode(ctx context.Context, req QueryCodePaymentRequest) (CodePaymentResult, error) {
	if ready, reason := w.CodePaymentReady(); !ready {
		return CodePaymentResult{}, fmt.Errorf("%w: %s", ErrNotConfigured, reason)
	}
	if req.MerchantNo == "" || req.OrderNo == "" {
		return CodePaymentResult{}, errors.New("merchant number and order number are required")
	}
	response, err := w.postV2(ctx, wechatOrderQueryPath, w.queryV2Params(req.MerchantNo, req.OrderNo), false)
	if err != nil {
		return CodePaymentResult{ProviderOrderNo: req.OrderNo, Status: PaymentPending, Message: "查单结果未知"}, err
	}
	result := codePaymentResultFromQuery(req.OrderNo, response)
	if result.Status == PaymentSuccess {
		result.PaidAt = parseWeChatTime(response["time_end"])
	}
	return result, nil
}

func (w *WeChatPayPartner) ReverseCode(ctx context.Context, req ReverseCodePaymentRequest) (ReverseCodePaymentResult, error) {
	if ready, reason := w.CodePaymentReady(); !ready {
		return ReverseCodePaymentResult{}, fmt.Errorf("%w: %s", ErrNotConfigured, reason)
	}
	if req.MerchantNo == "" || req.OrderNo == "" {
		return ReverseCodePaymentResult{}, errors.New("merchant number and order number are required")
	}
	response, err := w.postV2(ctx, wechatReversePath, w.queryV2Params(req.MerchantNo, req.OrderNo), true)
	if err != nil {
		return ReverseCodePaymentResult{Status: PaymentPending, Retry: true, Message: "撤销结果未知，请继续查单"}, err
	}
	result := ReverseCodePaymentResult{
		Status:    PaymentClosed,
		ErrorCode: response["err_code"],
		Message:   firstNonEmpty(response["err_code_des"], response["return_msg"]),
	}
	if response["return_code"] != "SUCCESS" || response["result_code"] != "SUCCESS" {
		result.Status = PaymentPending
		result.Retry = true
	}
	if response["recall"] == "Y" || response["err_code"] == "USERPAYING" || response["err_code"] == "SYSTEMERROR" {
		result.Status = PaymentPending
		result.Retry = true
	}
	if result.Status == PaymentClosed && result.Message == "" {
		result.Message = "本次付款已撤销"
	}
	return result, nil
}

func (w *WeChatPayPartner) baseV2Params(merchantNo, subAppID string) map[string]string {
	params := map[string]string{
		"appid":      strings.TrimSpace(w.Config.ServiceProviderAppID),
		"mch_id":     strings.TrimSpace(w.Config.ServiceProviderMchID),
		"sub_mch_id": strings.TrimSpace(merchantNo),
		"nonce_str":  wechatNonce(),
	}
	if strings.TrimSpace(subAppID) != "" {
		params["sub_appid"] = strings.TrimSpace(subAppID)
	}
	return params
}

func (w *WeChatPayPartner) queryV2Params(merchantNo, orderNo string) map[string]string {
	params := w.baseV2Params(merchantNo, "")
	params["out_trade_no"] = orderNo
	return params
}

func (w *WeChatPayPartner) postV2(ctx context.Context, path string, params map[string]string, mutualTLS bool) (map[string]string, error) {
	params["sign"] = signWeChatV2(params, w.Config.APIV2Key)
	body := marshalWeChatXML(params)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(w.Config.BaseURL, "/")+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/xml; charset=utf-8")
	client := w.HTTPClient
	if client == nil {
		httpClient := &http.Client{Timeout: 12 * time.Second}
		if mutualTLS {
			certificate, certErr := loadWechatClientCertificate(w.Config.MerchantCertificate, w.Config.MerchantPrivateKey)
			if certErr != nil {
				return nil, fmt.Errorf("%w: load WeChat Pay API certificate: %v", ErrNotConfigured, certErr)
			}
			httpClient.Transport = &http.Transport{TLSClientConfig: &tls.Config{
				MinVersion:   tls.VersionTLS12,
				Certificates: []tls.Certificate{certificate},
			}}
		}
		client = httpClient
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
		return nil, fmt.Errorf("WeChat Pay HTTP status %d", response.StatusCode)
	}
	payload, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	fields, err := parseWeChatXML(payload)
	if err != nil {
		return nil, err
	}
	if responseSign := strings.TrimSpace(fields["sign"]); responseSign != "" {
		expected := signWeChatV2(fields, w.Config.APIV2Key)
		if !strings.EqualFold(responseSign, expected) {
			return nil, errors.New("WeChat Pay response signature mismatch")
		}
	}
	return fields, nil
}

func signWeChatV2(params map[string]string, apiKey string) string {
	keys := make([]string, 0, len(params))
	for key, value := range params {
		if key != "sign" && strings.TrimSpace(value) != "" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	var source strings.Builder
	for _, key := range keys {
		if source.Len() > 0 {
			source.WriteByte('&')
		}
		source.WriteString(key)
		source.WriteByte('=')
		source.WriteString(params[key])
	}
	source.WriteString("&key=")
	source.WriteString(apiKey)
	sum := md5.Sum([]byte(source.String()))
	return strings.ToUpper(hex.EncodeToString(sum[:]))
}

func marshalWeChatXML(params map[string]string) []byte {
	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var output bytes.Buffer
	output.WriteString("<xml>")
	for _, key := range keys {
		output.WriteByte('<')
		output.WriteString(key)
		output.WriteByte('>')
		_ = xml.EscapeText(&output, []byte(params[key]))
		output.WriteString("</")
		output.WriteString(key)
		output.WriteByte('>')
	}
	output.WriteString("</xml>")
	return output.Bytes()
}

func parseWeChatXML(payload []byte) (map[string]string, error) {
	decoder := xml.NewDecoder(bytes.NewReader(payload))
	fields := make(map[string]string)
	var current string
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("decode WeChat Pay XML: %w", err)
		}
		switch value := token.(type) {
		case xml.StartElement:
			if value.Name.Local != "xml" {
				current = value.Name.Local
			}
		case xml.CharData:
			if current != "" {
				fields[current] += string(value)
			}
		case xml.EndElement:
			if value.Name.Local == current {
				fields[current] = strings.TrimSpace(fields[current])
				current = ""
			}
		}
	}
	if len(fields) == 0 {
		return nil, errors.New("empty WeChat Pay response")
	}
	return fields, nil
}

func codePaymentResultFromV2(orderNo string, response map[string]string) CodePaymentResult {
	result := CodePaymentResult{
		ProviderOrderNo:       orderNo,
		ProviderTransactionNo: response["transaction_id"],
		Status:                PaymentFailed,
		ErrorCode:             response["err_code"],
		Message:               firstNonEmpty(response["err_code_des"], response["return_msg"]),
		Raw:                   safeWeChatPaymentResponse(response),
	}
	if response["return_code"] == "SUCCESS" && response["result_code"] == "SUCCESS" {
		result.Status = PaymentSuccess
		result.Message = "微信支付成功"
		return result
	}
	switch response["err_code"] {
	case "USERPAYING":
		result.Status = PaymentPending
		result.NeedCustomerAction = true
		result.Message = "顾客正在输入支付密码"
	case "SYSTEMERROR", "BANKERROR":
		result.Status = PaymentPending
		result.Message = "支付结果确认中"
	}
	return result
}

func codePaymentResultFromQuery(orderNo string, response map[string]string) CodePaymentResult {
	result := CodePaymentResult{
		ProviderOrderNo:       orderNo,
		ProviderTransactionNo: response["transaction_id"],
		Status:                PaymentPending,
		ErrorCode:             response["err_code"],
		Message:               firstNonEmpty(response["err_code_des"], response["trade_state_desc"], response["return_msg"]),
		Raw:                   safeWeChatPaymentResponse(response),
	}
	if response["return_code"] != "SUCCESS" || response["result_code"] != "SUCCESS" {
		if response["err_code"] == "ORDERNOTEXIST" {
			result.Status = PaymentFailed
			result.Message = "微信支付订单不存在，请重新扫码"
		}
		return result
	}
	switch response["trade_state"] {
	case "SUCCESS":
		result.Status = PaymentSuccess
		result.Message = "微信支付成功"
	case "USERPAYING":
		result.NeedCustomerAction = true
		result.Message = "顾客正在输入支付密码"
	case "NOTPAY":
		result.Message = "等待顾客支付"
	case "CLOSED", "REVOKED":
		result.Status = PaymentClosed
		result.Message = "本次付款已关闭"
	case "PAYERROR":
		result.Status = PaymentFailed
		result.Message = "微信支付失败"
	case "REFUND":
		result.Status = PaymentRefunded
		result.Message = "该笔微信支付已退款"
	}
	return result
}

func safeWeChatPaymentResponse(response map[string]string) map[string]string {
	keys := []string{"return_code", "return_msg", "result_code", "err_code", "err_code_des", "trade_type", "trade_state", "trade_state_desc", "transaction_id", "out_trade_no", "total_fee", "time_end", "recall"}
	safe := make(map[string]string, len(keys))
	for _, key := range keys {
		if value := strings.TrimSpace(response[key]); value != "" {
			safe[key] = value
		}
	}
	return safe
}

func validWeChatAuthCode(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 18 {
		return false
	}
	switch value[:2] {
	case "10", "11", "12", "13", "14", "15":
	default:
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func wechatNonce() string {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return hex.EncodeToString(value)
}

func parseWeChatTime(value string) *time.Time {
	parsed, err := time.ParseInLocation("20060102150405", strings.TrimSpace(value), time.Local)
	if err != nil {
		return nil
	}
	return &parsed
}

func truncateUTF8(value string, maxRunes int) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes])
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func wechatStoreSceneInfo(storeID, storeName, storeAddress string) string {
	storeID = strings.TrimSpace(storeID)
	storeName = strings.TrimSpace(storeName)
	storeAddress = strings.TrimSpace(storeAddress)
	if storeID == "" || storeName == "" {
		return ""
	}
	escape := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", " ", "\r", " ")
	return fmt.Sprintf(`{"store_info":{"id":"%s","name":"%s","area_code":"000000","address":"%s"}}`,
		escape.Replace(storeID), escape.Replace(storeName), escape.Replace(storeAddress))
}

func loadWechatClientCertificate(certificateValue, privateKeyValue string) (tls.Certificate, error) {
	certificatePEM, err := loadPEMMaterial(certificateValue)
	if err != nil {
		return tls.Certificate{}, err
	}
	privateKeyPEM, err := loadPEMMaterial(privateKeyValue)
	if err != nil {
		return tls.Certificate{}, err
	}
	certificate, err := tls.X509KeyPair(certificatePEM, privateKeyPEM)
	if err != nil {
		return tls.Certificate{}, err
	}
	if len(certificate.Certificate) == 0 {
		return tls.Certificate{}, errors.New("API certificate contains no certificates")
	}
	if _, err = x509.ParseCertificate(certificate.Certificate[0]); err != nil {
		return tls.Certificate{}, err
	}
	return certificate, nil
}

func loadPEMMaterial(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, errors.New("PEM material is empty")
	}
	value = strings.ReplaceAll(value, `\n`, "\n")
	if strings.Contains(value, "-----BEGIN ") {
		return []byte(value), nil
	}
	if strings.HasPrefix(value, "file:") {
		value = strings.TrimSpace(strings.TrimPrefix(value, "file:"))
	}
	return os.ReadFile(value)
}
