package config

import (
	"os"
	"testing"
)

func TestLoadRequiresCredentials(t *testing.T) {
	os.Unsetenv("API_BASE_URL")
	os.Unsetenv("WORKER_TOKEN")
	os.Unsetenv("WORKER_ID")
	if _, e := Load(); e == nil {
		t.Fatal("expected missing credentials")
	}
}
func TestLoadValid(t *testing.T) {
	os.Setenv("API_BASE_URL", "https://example.invalid")
	os.Setenv("WORKER_TOKEN", "x")
	os.Setenv("WORKER_ID", "w")
	defer func() { os.Clearenv() }()
	c, e := Load()
	if e != nil || c.MaxConcurrent != 4 {
		t.Fatalf("config: %+v %v", c, e)
	}
}
