package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"imagerelayworker/internal/api"
	"imagerelayworker/internal/config"
	"imagerelayworker/internal/downloader"
	"imagerelayworker/internal/worker"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("invalid configuration", "error", err)
		os.Exit(1)
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
