package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	APIBaseURL, WorkerToken, WorkerID, UserAgent, LogLevel, HealthBindAddress string
	PollInterval, DownloadTimeout, UploadTimeout                              time.Duration
	MaxConcurrent, MaxImageSizeMB, RetryMaxAttempts, RetryBaseDelayMS         int
	HealthPort                                                                int
	AllowHTTP                                                                 bool
}

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func num(k string, d int) (int, error) {
	v := env(k, strconv.Itoa(d))
	n, e := strconv.Atoi(v)
	if e != nil || n < 1 {
		return 0, fmt.Errorf("%s must be a positive integer", k)
	}
	return n, nil
}
func Load() (Config, error) {
	c := Config{APIBaseURL: strings.TrimRight(os.Getenv("API_BASE_URL"), "/"), WorkerToken: os.Getenv("WORKER_TOKEN"), WorkerID: os.Getenv("WORKER_ID"), UserAgent: env("HTTP_USER_AGENT", "ImageRelayWorker/1.0"), LogLevel: env("LOG_LEVEL", "info"), HealthBindAddress: env("HEALTH_BIND_ADDRESS", "127.0.0.1")}
	if c.APIBaseURL == "" || c.WorkerToken == "" || c.WorkerID == "" {
		return c, fmt.Errorf("API_BASE_URL, WORKER_TOKEN and WORKER_ID are required")
	}
	if !strings.HasPrefix(c.APIBaseURL, "https://") && !strings.HasPrefix(c.APIBaseURL, "http://") {
		return c, fmt.Errorf("API_BASE_URL must use http or https")
	}
	var e error
	var n int
	n, e = num("POLL_INTERVAL_SECONDS", 5)
	if e != nil {
		return c, e
	}
	c.PollInterval = time.Duration(n) * time.Second
	n, e = num("MAX_CONCURRENT_JOBS", 4)
	if e != nil {
		return c, e
	}
	c.MaxConcurrent = n
	n, e = num("DOWNLOAD_TIMEOUT_SECONDS", 30)
	if e != nil {
		return c, e
	}
	c.DownloadTimeout = time.Duration(n) * time.Second
	n, e = num("UPLOAD_TIMEOUT_SECONDS", 60)
	if e != nil {
		return c, e
	}
	c.UploadTimeout = time.Duration(n) * time.Second
	n, e = num("MAX_IMAGE_SIZE_MB", 25)
	if e != nil {
		return c, e
	}
	c.MaxImageSizeMB = n
	n, e = num("RETRY_MAX_ATTEMPTS", 4)
	if e != nil {
		return c, e
	}
	c.RetryMaxAttempts = n
	n, e = num("RETRY_BASE_DELAY_MS", 1000)
	if e != nil {
		return c, e
	}
	c.RetryBaseDelayMS = n
	n, e = num("HEALTH_PORT", 8080)
	if e != nil {
		return c, e
	}
	c.HealthPort = n
	c.AllowHTTP = strings.EqualFold(env("ALLOW_HTTP", "false"), "true")
	return c, nil
}
