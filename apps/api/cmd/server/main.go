package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ethan0119-dev/tanban/apps/api/internal/app"
	"github.com/ethan0119-dev/tanban/apps/api/internal/config"
	"github.com/ethan0119-dev/tanban/apps/api/internal/database"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("load configuration", "error", err)
		os.Exit(1)
	}
	db, err := database.Open(cfg.DatabaseDSN)
	if err != nil {
		logger.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	if cfg.AutoMigrate {
		if err = database.Migrate(ctx, db, cfg.MigrationsDir); err != nil {
			cancel()
			logger.Error("apply migrations", "error", err)
			os.Exit(1)
		}
	}
	server := app.New(db, cfg, logger)
	if err = server.BootstrapAdmin(ctx); err == nil {
		err = server.SeedDemo(ctx)
	}
	cancel()
	if err != nil {
		logger.Error("bootstrap application", "error", err)
		os.Exit(1)
	}
	workerCtx, workerCancel := context.WithCancel(context.Background())
	server.StartPrintWorker(workerCtx)
	server.StartPaymentReconciler(workerCtx)
	server.StartRefundReconciler(workerCtx)
	server.StartOrderExpirationWorker(workerCtx)
	httpServer := &http.Server{Addr: cfg.HTTPAddr, Handler: server.Routes(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		logger.Info("tanban api started", "addr", cfg.HTTPAddr, "payment_provider", server.Payment.Name(), "printer_provider", server.Printer.Name())
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("http server stopped", "error", err)
			os.Exit(1)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	workerCancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	_ = httpServer.Shutdown(shutdownCtx)
}
