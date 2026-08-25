package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"imagerelayworker/internal/config"
)

func TestConfigSaveNotifiesWorkerLifecycle(t *testing.T) {
	t.Setenv("WORKER_CONFIG_FILE", filepath.Join(t.TempDir(), "config.json"))
	var received config.Config
	s := &Server{Config: config.Config{WorkerToken: "secret"}, Running: func() bool { return false }, OnConfigSaved: func(c config.Config) { received = c }}
	r := httptest.NewRequest(http.MethodPut, "/api/config", bytes.NewBufferString(`{"APIBaseURL":"https://api.example.test","WorkerToken":"secret","WorkerID":"worker-1"}`))
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusOK || received.WorkerID != "worker-1" {
		t.Fatalf("save notification failed: status=%d config=%+v", w.Code, received)
	}
	var response map[string]bool
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil || !response["workerRestarted"] {
		t.Fatalf("unexpected response: %s", w.Body.String())
	}
}
