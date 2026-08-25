package main

import (
	"context"
	"crypto/tls"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"sync/atomic"
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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	updates := make(chan config.Config, 1)
	if err == nil {
		updates <- cfg
	} else {
		slog.Error("worker is waiting for configuration", "error", err)
	}
	if !platform.IsWindowsService() {
		go serveWeb(cfg, err == nil, updates)
	}
	var cancel context.CancelFunc
	for {
		select {
		case next := <-updates:
			if cancel != nil {
				cancel()
			}
			var child context.Context
			child, cancel = context.WithCancel(ctx)
			go runWorker(child, next)
		case <-ctx.Done():
			return
		}
	}
}

func runWorker(ctx context.Context, cfg config.Config) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: parseLevel(cfg.LogLevel)}))
	transport := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}, MaxIdleConns: 32, MaxIdleConnsPerHost: 8, IdleConnTimeout: 90 * time.Second, ResponseHeaderTimeout: cfg.DownloadTimeout}
	httpClient := &http.Client{Transport: transport}
	client := &api.Client{BaseURL: cfg.APIBaseURL, Token: cfg.WorkerToken, WorkerID: cfg.WorkerID, HTTP: httpClient, UploadTimeout: cfg.UploadTimeout}
	dl := &downloader.Downloader{Client: httpClient, Timeout: cfg.DownloadTimeout, MaxBytes: int64(cfg.MaxImageSizeMB) * 1024 * 1024, UserAgent: cfg.UserAgent, AllowHTTP: cfg.AllowHTTP}
	w := &worker.Worker{API: client, DL: dl, Poll: cfg.PollInterval, Concurrency: cfg.MaxConcurrent, Attempts: cfg.RetryMaxAttempts, BaseDelay: cfg.RetryBaseDelayMS, Log: logger}
	logger.Info("worker started", "workerId", cfg.WorkerID, "concurrency", cfg.MaxConcurrent)
	_ = w.Run(ctx)
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

func serveWeb(cfg config.Config, running bool, updates chan<- config.Config) {
	active := &atomic.Bool{}
	active.Store(running)
	s := &web.Server{Config: cfg, Addr: "127.0.0.1:5173", Running: active.Load, OnConfigSaved: func(c config.Config) { active.Store(true); updates <- c }}
	go func() {
		time.Sleep(400 * time.Millisecond)
		_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", "http://127.0.0.1:5173").Start()
	}()
	if err := s.Listen(); err != nil {
		slog.Error("web server failed", "error", err)
	}
}
