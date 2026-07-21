package provider

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

const defaultXPYunBaseURL = "https://open.xpyun.net/api/openapi/xprinter"

type PrintRequest struct {
	JobID      int64
	Provider   string
	DeviceSN   string
	DeviceType string
	OutputType string
	Content    string
	Reprint    bool
}

type PrintResult struct {
	ProviderJobNo string `json:"provider_job_no"`
	Status        string `json:"status"`
}

type PrinterStatusRequest struct {
	Provider string
	DeviceSN string
}

type PrinterStatusResult struct {
	Status    string    `json:"status"`
	Message   string    `json:"message,omitempty"`
	CheckedAt time.Time `json:"checked_at"`
}

type PrinterProvider interface {
	Name() string
	Print(context.Context, PrintRequest) (PrintResult, error)
	Status(context.Context, PrinterStatusRequest) PrinterStatusResult
}

type MockPrinter struct{ Logger *slog.Logger }

func (m MockPrinter) Name() string { return "mock" }
func (m MockPrinter) Print(_ context.Context, req PrintRequest) (PrintResult, error) {
	if m.Logger != nil {
		m.Logger.Info("virtual printer output", "job_id", req.JobID, "sn", req.DeviceSN, "reprint", req.Reprint, "content", req.Content)
	}
	return PrintResult{ProviderJobNo: fmt.Sprintf("MOCKPRINT-%d", req.JobID), Status: "SUCCESS"}, nil
}
func (MockPrinter) Status(_ context.Context, _ PrinterStatusRequest) PrinterStatusResult {
	return PrinterStatusResult{Status: "SIMULATED", Message: "模拟设备仅记录任务，不代表实体设备在线", CheckedAt: time.Now()}
}

type XPrinterConfig struct {
	BaseURL string
	User    string
	UserKey string
	Client  *http.Client
}

type XPrinter struct {
	baseURL string
	user    string
	userKey string
	client  *http.Client
}

func NewXPrinter(cfg XPrinterConfig) *XPrinter {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = defaultXPYunBaseURL
	}
	client := cfg.Client
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}
	return &XPrinter{baseURL: baseURL, user: strings.TrimSpace(cfg.User), userKey: strings.TrimSpace(cfg.UserKey), client: client}
}

func (x *XPrinter) Name() string { return "xpyun" }

func (x *XPrinter) configured() bool {
	return x != nil && x.user != "" && x.userKey != ""
}

func (x *XPrinter) Print(ctx context.Context, req PrintRequest) (PrintResult, error) {
	if !x.configured() {
		return PrintResult{}, fmt.Errorf("%w: 芯烨云缺少开发者ID或UserKEY", ErrNotConfigured)
	}
	endpoint := "print"
	if strings.EqualFold(req.OutputType, "LABEL") {
		endpoint = "printLabel"
		if !strings.Contains(strings.ToUpper(req.Content), "<SIZE>") {
			return PrintResult{}, errors.New("芯烨标签打印内容缺少标签尺寸；请先配置标签宽度和高度")
		}
	}
	payload := map[string]any{
		"sn": req.DeviceSN, "content": req.Content, "copies": 1,
		"voice": 2, "mode": 0,
	}
	if req.JobID > 0 {
		payload["idempotent"] = fmt.Sprintf("TB-%d", req.JobID)
	}
	var response xPrinterResponse
	if err := x.call(ctx, endpoint, payload, &response); err != nil {
		return PrintResult{}, err
	}
	jobNo, _ := response.Data.(string)
	if jobNo == "" {
		return PrintResult{}, errors.New("芯烨云未返回打印订单号")
	}
	return PrintResult{ProviderJobNo: jobNo, Status: "SUCCESS"}, nil
}

func (x *XPrinter) Status(ctx context.Context, req PrinterStatusRequest) PrinterStatusResult {
	checkedAt := time.Now()
	if !x.configured() {
		return PrinterStatusResult{Status: "UNREACHABLE", Message: "芯烨云未配置开发者ID / UserKEY", CheckedAt: checkedAt}
	}
	var response xPrinterResponse
	if err := x.call(ctx, "queryPrinterStatus", map[string]any{"sn": req.DeviceSN}, &response); err != nil {
		return PrinterStatusResult{Status: "UNREACHABLE", Message: userFacingPrinterError(err), CheckedAt: checkedAt}
	}
	status, ok := jsonNumberInt(response.Data)
	if !ok {
		return PrinterStatusResult{Status: "UNREACHABLE", Message: "芯烨云返回了无法识别的设备状态", CheckedAt: checkedAt}
	}
	switch status {
	case 0:
		return PrinterStatusResult{Status: "OFFLINE", Message: "设备与芯烨云失去联系超过 30 秒", CheckedAt: checkedAt}
	case 1:
		return PrinterStatusResult{Status: "ONLINE", Message: "设备在线且状态正常", CheckedAt: checkedAt}
	case 2:
		return PrinterStatusResult{Status: "PAPER_OUT", Message: "设备在线但状态异常，通常为缺纸", CheckedAt: checkedAt}
	default:
		return PrinterStatusResult{Status: "UNREACHABLE", Message: fmt.Sprintf("芯烨云返回未知状态 %d", status), CheckedAt: checkedAt}
	}
}

type xPrinterResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data"`
}

type xPrinterError struct {
	Code int
	Msg  string
}

func (e xPrinterError) Error() string {
	return fmt.Sprintf("芯烨云接口错误 code=%d msg=%s", e.Code, e.Msg)
}

func (x *XPrinter) call(ctx context.Context, endpoint string, payload map[string]any, output *xPrinterResponse) error {
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	hash := sha1.Sum([]byte(x.user + x.userKey + timestamp))
	payload["user"] = x.user
	payload["timestamp"] = timestamp
	payload["sign"] = hex.EncodeToString(hash[:])
	payload["debug"] = "0"
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, x.baseURL+"/"+endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json;charset=UTF-8")
	response, err := x.client.Do(request)
	if err != nil {
		return fmt.Errorf("连接芯烨云失败: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("芯烨云 HTTP 状态 %d", response.StatusCode)
	}
	decoder := json.NewDecoder(response.Body)
	decoder.UseNumber()
	if err = decoder.Decode(output); err != nil {
		return fmt.Errorf("解析芯烨云响应失败: %w", err)
	}
	if output.Code != 0 {
		return xPrinterError{Code: output.Code, Msg: output.Msg}
	}
	return nil
}

func jsonNumberInt(value any) (int, bool) {
	switch typed := value.(type) {
	case json.Number:
		parsed, err := typed.Int64()
		return int(parsed), err == nil
	case float64:
		return int(typed), typed == float64(int(typed))
	case int:
		return typed, true
	default:
		return 0, false
	}
}

func userFacingPrinterError(err error) string {
	var apiErr xPrinterError
	if errors.As(err, &apiErr) {
		switch apiErr.Code {
		case -4:
			return "芯烨云开发者账号不存在"
		case 1001:
			return "设备 SN 不属于当前芯烨云开发者账号"
		case 1002:
			return "设备 SN 尚未绑定到当前芯烨云开发者账号"
		case 1003:
			return "设备当前离线"
		}
	}
	return truncateProviderMessage(err.Error())
}

func truncateProviderMessage(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 180 {
		return value
	}
	return value[:180]
}

type PrinterRouter struct {
	defaultName string
	mock        MockPrinter
	xpyun       *XPrinter
}

func NewPrinterRouter(defaultName string, logger *slog.Logger, xpyun *XPrinter) *PrinterRouter {
	return &PrinterRouter{defaultName: normalizePrinterProvider(defaultName), mock: MockPrinter{Logger: logger}, xpyun: xpyun}
}

func (r *PrinterRouter) Name() string { return "router" }

func (r *PrinterRouter) Print(ctx context.Context, req PrintRequest) (PrintResult, error) {
	selected, err := r.provider(req.Provider)
	if err != nil {
		return PrintResult{}, err
	}
	return selected.Print(ctx, req)
}

func (r *PrinterRouter) Status(ctx context.Context, req PrinterStatusRequest) PrinterStatusResult {
	selected, err := r.provider(req.Provider)
	if err != nil {
		return PrinterStatusResult{Status: "UNREACHABLE", Message: err.Error(), CheckedAt: time.Now()}
	}
	return selected.Status(ctx, req)
}

func (r *PrinterRouter) provider(name string) (PrinterProvider, error) {
	name = normalizePrinterProvider(name)
	if name == "" {
		name = r.defaultName
	}
	switch name {
	case "mock":
		return r.mock, nil
	case "xpyun":
		return r.xpyun, nil
	default:
		return nil, fmt.Errorf("不支持打印服务商 %q", name)
	}
}

func normalizePrinterProvider(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "芯烨", "芯烨云", "xprinter", "x-printer", "xpyun":
		return "xpyun"
	default:
		return value
	}
}
