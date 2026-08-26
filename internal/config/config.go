package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

func FilePath() string {
	if p := os.Getenv("WORKER_CONFIG_FILE"); p != "" {
		return p
	}
	d, err := os.UserConfigDir()
	if err != nil {
		return "config.json"
	}
	return d + string(os.PathSeparator) + "ImageRelayWorker" + string(os.PathSeparator) + "config.json"
}

func LoadFile() (Config, error) {
	b, err := os.ReadFile(FilePath())
	if err != nil {
		return Config{}, err
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return c, err
	}
	c = c.WithDefaults()
	return c, c.Validate()
}

func (c Config) WithDefaults() Config {
	if c.UserAgent == "" {
		c.UserAgent = "ImageRelayWorker/1.0"
	}
	if c.LogLevel == "" {
		c.LogLevel = "info"
	}
	if c.HealthBindAddress == "" {
		c.HealthBindAddress = "127.0.0.1"
	}
	if c.PollInterval == 0 {
		c.PollInterval = 5 * time.Second
	}
	if c.MaxConcurrent == 0 {
		c.MaxConcurrent = 4
	}
	if c.DownloadTimeout == 0 {
		c.DownloadTimeout = 30 * time.Second
	}
	if c.UploadTimeout == 0 {
		c.UploadTimeout = 60 * time.Second
	}
	if c.MaxImageSizeMB == 0 {
		c.MaxImageSizeMB = 25
	}
	if c.RetryMaxAttempts == 0 {
		c.RetryMaxAttempts = 4
	}
	if c.RetryBaseDelayMS == 0 {
		c.RetryBaseDelayMS = 1000
	}
	if c.HealthPort == 0 {
		c.HealthPort = 8080
	}
	return c
}

func (c Config) Validate() error {
	if c.APIBaseURL == "" || c.WorkerToken == "" || c.WorkerID == "" {
		return fmt.Errorf("APIBaseURL, WorkerToken and WorkerID are required")
	}
	if !strings.HasPrefix(c.APIBaseURL, "https://") && !strings.HasPrefix(c.APIBaseURL, "http://") {
		return fmt.Errorf("APIBaseURL must use http or https")
	}
	return nil
}

func SaveFile(c Config) error {
	c = c.WithDefaults()
	if err := c.Validate(); err != nil {
		return err
	}
	p := FilePath()
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0600)
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
	if c, err := LoadFile(); err == nil {
		return c, nil
	}
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
