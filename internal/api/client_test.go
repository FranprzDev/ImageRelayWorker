package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"imagerelayworker/internal/model"
	"imagerelayworker/internal/retry"
)

func TestClientContractAndAuth(t *testing.T) {
	var upload []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" || r.Header.Get("X-Worker-Id") != "worker-1" {
			t.Fatal("missing worker authentication headers")
		}
		switch {
		case r.URL.Path == "/api/image-jobs/claim":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"job":{"id":"job-1","imageUrl":"https://example.com/a.jpg","productId":"product-1"}}`)
		case strings.HasSuffix(r.URL.Path, "/upload"):
			var err error
			upload, err = io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			w.WriteHeader(http.StatusCreated)
		case strings.HasSuffix(r.URL.Path, "/complete"), strings.HasSuffix(r.URL.Path, "/fail"):
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	c := &Client{BaseURL: server.URL, Token: "secret", WorkerID: "worker-1", HTTP: server.Client(), UploadTimeout: time.Second}
	job, err := c.Claim(context.Background())
	if err != nil || job == nil || job.ID != "job-1" {
		t.Fatalf("claim: %+v %v", job, err)
	}
	resp, err := c.Upload(context.Background(), job, strings.NewReader("bytes"), "image/jpeg")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if string(upload) != "bytes" {
		t.Fatalf("upload bytes: %q", upload)
	}
	if err := c.Complete(context.Background(), job, model.CompleteRequest{WorkerID: "worker-1"}); err != nil {
		t.Fatal(err)
	}
	if err := c.Fail(context.Background(), job, "bad image"); err != nil {
		t.Fatal(err)
	}
}

func TestClientClassifiesStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "no", http.StatusForbidden) }))
	defer server.Close()
	c := &Client{BaseURL: server.URL, Token: "secret", WorkerID: "worker-1", HTTP: server.Client()}
	_, err := c.Claim(context.Background())
	if !retry.IsPermanent(err) {
		t.Fatalf("expected permanent error, got %T %v", err, err)
	}
}

func TestHeartbeatAndStatusUseHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" || r.Header.Get("X-Worker-Id") != "worker-1" {
			t.Fatal("missing worker authentication headers")
		}
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		io.WriteString(w, `{"id":"job-1","state":"upload_received","upload":{"bytes":12,"sha256":"abc","contentType":"image/jpeg"}}`)
	}))
	defer server.Close()
	c := &Client{BaseURL: server.URL, Token: "secret", WorkerID: "worker-1", HTTP: server.Client()}
	if err := c.Heartbeat(context.Background(), "job-1"); err != nil {
		t.Fatal(err)
	}
	status, err := c.Status(context.Background(), "job-1")
	if err != nil || status.State != "upload_received" || status.Upload == nil || status.Upload.Bytes != 12 {
		t.Fatalf("unexpected status: %+v, %v", status, err)
	}
}
