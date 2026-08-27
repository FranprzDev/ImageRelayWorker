package worker

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"imagerelayworker/internal/api"
	"imagerelayworker/internal/downloader"
)

func TestRunClaimsStreamsUploadsAndCompletes(t *testing.T) {
	want := []byte("fake image bytes")
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(want)
	}))
	defer origin.Close()
	originURL, _ := url.Parse(origin.URL)

	var uploaded []byte
	claimed := false
	var cancel context.CancelFunc
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/image-jobs/claim":
			if claimed {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			claimed = true
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"job":{"id":"job-1","imageUrl":"http://example.com/image.jpg","productId":"product-1"}}`)
		case strings.HasSuffix(r.URL.Path, "/upload"):
			uploaded, _ = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusCreated)
		case strings.HasSuffix(r.URL.Path, "/complete"):
			w.WriteHeader(http.StatusNoContent)
			go func() { time.Sleep(10 * time.Millisecond); cancel() }()
		case strings.HasSuffix(r.URL.Path, "/fail"):
			t.Errorf("unexpected fail call")
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer apiServer.Close()

	imageClient := origin.Client()
	original := imageClient.Transport
	imageClient.Transport = rewriteTransport{base: originURL, next: original}
	dl := &downloader.Downloader{Client: imageClient, Timeout: time.Second, MaxBytes: 100, UserAgent: "test", AllowHTTP: true}
	client := &api.Client{BaseURL: apiServer.URL, Token: "token", WorkerID: "worker-1", HTTP: apiServer.Client(), UploadTimeout: time.Second}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	w := &Worker{API: client, DL: dl, Poll: time.Millisecond, Concurrency: 1, Attempts: 1, Log: logger, Stats: &Stats{}}
	ctx, stop := context.WithCancel(context.Background())
	cancel = stop
	if err := w.Run(ctx); err != context.Canceled {
		t.Fatalf("run returned %v", err)
	}
	if string(uploaded) != string(want) {
		t.Fatalf("uploaded %q, want %q", uploaded, want)
	}
	if w.Stats.JobsCompleted.Load() != 1 {
		t.Fatalf("completed=%d", w.Stats.JobsCompleted.Load())
	}
}

type rewriteTransport struct {
	base *url.URL
	next http.RoundTripper
}

func (t rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	copy := req.Clone(req.Context())
	copy.URL.Scheme, copy.URL.Host = t.base.Scheme, t.base.Host
	return t.next.RoundTrip(copy)
}
