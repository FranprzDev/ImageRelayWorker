package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"imagerelayworker/internal/api"
	"imagerelayworker/internal/config"
	"imagerelayworker/internal/downloader"
	"imagerelayworker/internal/platform"
	"imagerelayworker/internal/web"
	"imagerelayworker/internal/worker"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		cfg = config.Config{HealthBindAddress: "127.0.0.1", HealthPort: 8080, PollInterval: 5 * time.Second}
	}
	if !platform.IsWindowsService() {
		go serveWeb(cfg, err == nil)
	}
	if err != nil {
		slog.Error("worker is waiting for configuration", "error", err)
		select {}
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: parseLevel(cfg.LogLevel)}))
	transport := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}, MaxIdleConns: 32, MaxIdleConnsPerHost: 8, IdleConnTimeout: 90 * time.Second, ResponseHeaderTimeout: cfg.DownloadTimeout}
	httpClient := &http.Client{Transport: transport}
	client := &api.Client{BaseURL: cfg.APIBaseURL, Token: cfg.WorkerToken, WorkerID: cfg.WorkerID, HTTP: httpClient, UploadTimeout: cfg.UploadTimeout}
	downloaderClient := &downloader.Downloader{Client: httpClient, Timeout: cfg.DownloadTimeout, MaxBytes: int64(cfg.MaxImageSizeMB) * 1024 * 1024, UserAgent: cfg.UserAgent, AllowHTTP: cfg.AllowHTTP}
	jobWorker := &worker.Worker{API: client, DL: downloaderClient, Poll: cfg.PollInterval, Concurrency: cfg.MaxConcurrent, Attempts: cfg.RetryMaxAttempts, BaseDelay: cfg.RetryBaseDelayMS, Log: logger}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	healthServer := &http.Server{Addr: fmt.Sprintf("%s:%d", cfg.HealthBindAddress, cfg.HealthPort), Handler: worker.Health(&jobWorker.Stats, cfg.WorkerID)}
	go func() {
		if err := healthServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("health server failed", "error", err)
		}
	}()
	logger.Info("worker started", "workerId", cfg.WorkerID, "concurrency", cfg.MaxConcurrent)
	_ = jobWorker.Run(ctx)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = healthServer.Shutdown(shutdownCtx)
	transport.CloseIdleConnections()
}

func parseLevel(value string) slog.Level {
	switch value {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func serveWeb(cfg config.Config, running bool) {
	s := &web.Server{Config: cfg, Addr: "127.0.0.1:5173", Running: func() bool { return running }}
	go func() {
		time.Sleep(400 * time.Millisecond)
		_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", "http://127.0.0.1:5173").Start()
	}()
	if err := s.Listen(); err != nil {
		slog.Error("web server failed", "error", err)
	}
}
