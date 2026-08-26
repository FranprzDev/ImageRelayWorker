package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"sync/atomic"
	"time"

	"imagerelayworker/internal/api"
	"imagerelayworker/internal/config"
	"imagerelayworker/internal/downloader"
	"imagerelayworker/internal/platform"
	"imagerelayworker/internal/web"
	"imagerelayworker/internal/worker"
)

func main() {
	if err := platform.RunService(run); err != nil {
		slog.Error("service failed", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) {
	cfg, err := config.Load()
	if err != nil {
		cfg = config.Config{HealthBindAddress: "127.0.0.1", HealthPort: 8080, PollInterval: 5 * time.Second}
	}
	updates := make(chan config.Config, 1)
	if err == nil {
		updates <- cfg
	} else {
		slog.Error("worker is waiting for configuration", "error", err)
	}
	go serveWeb(cfg, err == nil, updates)
	var lifecycle workerLifecycle
	defer lifecycle.Stop()
	for {
		select {
		case next := <-updates:
			lifecycle.Restart(ctx, next)
		case <-ctx.Done():
			return
		}
	}
}

type workerLifecycle struct{ cancel context.CancelFunc }

func (l *workerLifecycle) Restart(parent context.Context, cfg config.Config) {
	l.Stop()
	child, cancel := context.WithCancel(parent)
	l.cancel = cancel
	go runWorker(child, cfg)
}

func (l *workerLifecycle) Stop() {
	if l.cancel != nil {
		l.cancel()
		l.cancel = nil
	}
}

func runWorker(ctx context.Context, cfg config.Config) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: parseLevel(cfg.LogLevel)}))
	transport := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}, MaxIdleConns: 32, MaxIdleConnsPerHost: 8, IdleConnTimeout: 90 * time.Second, ResponseHeaderTimeout: cfg.DownloadTimeout}
	httpClient := &http.Client{Transport: transport}
	client := &api.Client{BaseURL: cfg.APIBaseURL, Token: cfg.WorkerToken, WorkerID: cfg.WorkerID, HTTP: httpClient, UploadTimeout: cfg.UploadTimeout}
	dl := &downloader.Downloader{Client: httpClient, Timeout: cfg.DownloadTimeout, MaxBytes: int64(cfg.MaxImageSizeMB) * 1024 * 1024, UserAgent: cfg.UserAgent, AllowHTTP: cfg.AllowHTTP}
	w := &worker.Worker{API: client, DL: dl, Poll: cfg.PollInterval, Concurrency: cfg.MaxConcurrent, Attempts: cfg.RetryMaxAttempts, BaseDelay: cfg.RetryBaseDelayMS, Log: logger}
	healthServer := &http.Server{Addr: fmt.Sprintf("%s:%d", cfg.HealthBindAddress, cfg.HealthPort), Handler: worker.Health(&w.Stats, cfg.WorkerID)}
	go func() {
		if err := healthServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("health server failed", "error", err)
		}
	}()
	logger.Info("worker started", "workerId", cfg.WorkerID, "concurrency", cfg.MaxConcurrent)
	_ = w.Run(ctx)
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

func serveWeb(cfg config.Config, running bool, updates chan<- config.Config) {
	active := &atomic.Bool{}
	active.Store(running)
	s := &web.Server{Config: cfg, Addr: "127.0.0.1:5173", Running: active.Load, OnConfigSaved: func(c config.Config) { active.Store(true); updates <- c }}
	if !platform.IsWindowsService() {
		go func() {
			time.Sleep(400 * time.Millisecond)
			_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", "http://127.0.0.1:5173").Start()
		}()
	}
	if err := s.Listen(); err != nil {
		slog.Error("web server failed", "error", err)
	}
}
