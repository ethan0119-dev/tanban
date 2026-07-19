package provider

import (
	"context"
	"fmt"
	"log/slog"
)

type PrintRequest struct {
	JobID      int64
	DeviceSN   string
	DeviceType string
	Content    string
	Reprint    bool
}

type PrintResult struct {
	ProviderJobNo string `json:"provider_job_no"`
	Status        string `json:"status"`
}

type PrinterProvider interface {
	Name() string
	Print(context.Context, PrintRequest) (PrintResult, error)
}

type MockPrinter struct{ Logger *slog.Logger }

func (m MockPrinter) Name() string { return "mock" }
func (m MockPrinter) Print(_ context.Context, req PrintRequest) (PrintResult, error) {
	if m.Logger != nil {
		m.Logger.Info("virtual printer output", "job_id", req.JobID, "sn", req.DeviceSN, "reprint", req.Reprint, "content", req.Content)
	}
	return PrintResult{ProviderJobNo: fmt.Sprintf("MOCKPRINT-%d", req.JobID), Status: "SUCCESS"}, nil
}

type XPrinter struct{}

func (XPrinter) Name() string { return "xprinter" }
func (XPrinter) Print(context.Context, PrintRequest) (PrintResult, error) {
	return PrintResult{}, ErrNotConfigured
}
