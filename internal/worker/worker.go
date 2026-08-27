package worker

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"imagerelayworker/internal/api"
	"imagerelayworker/internal/downloader"
	"imagerelayworker/internal/model"
	"imagerelayworker/internal/retry"
)

type Stats struct{ JobsClaimed, JobsCompleted, JobsFailed, BytesTransferred, Retries atomic.Int64 }

type Worker struct {
	API                              *api.Client
	DL                               *downloader.Downloader
	Poll                             time.Duration
	Concurrency, Attempts, BaseDelay int
	Log                              *slog.Logger
	Stats                            *Stats
}

func (w *Worker) Run(ctx context.Context) error {
	sem := make(chan struct{}, w.Concurrency)
	var jobs sync.WaitGroup
	for {
		if ctx.Err() != nil {
			break
		}
		job, err := w.API.Claim(ctx)
		if err != nil {
			w.Log.Error("claim failed", "error", err)
			if !wait(ctx, w.Poll) {
				break
			}
			continue
		}
		if job == nil {
			if !wait(ctx, w.Poll) {
				break
			}
			continue
		}
		w.Stats.JobsClaimed.Add(1)
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			break
		}
		if ctx.Err() != nil {
			break
		}
		jobs.Add(1)
		go func(job *model.ImageJob) {
			defer jobs.Done()
			defer func() { <-sem }()
			attempt := 0
			if err := retry.Do(ctx, w.Attempts, w.BaseDelay, func() error {
				attempt++
				if attempt > 1 {
					w.Stats.Retries.Add(1)
					status, statusErr := w.API.Status(ctx, job.ID)
					if statusErr == nil && (status.State == "upload_received" || status.State == "completed") {
						return nil
					}
				}
				return w.process(ctx, job)
			}); err != nil {
				w.Stats.JobsFailed.Add(1)
				failCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				_ = w.API.Fail(failCtx, job, short(err.Error()))
				cancel()
				w.Log.Error("job failed", "jobId", job.ID, "error", err)
			} else {
				w.Stats.JobsCompleted.Add(1)
			}
		}(job)
	}

	done := make(chan struct{})
	go func() { jobs.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		w.Log.Warn("graceful shutdown deadline reached")
	}
	return ctx.Err()
}

func wait(ctx context.Context, duration time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(duration):
		return true
	}
}

type countingWriter struct {
	hash  hash.Hash
	bytes int64
}

func (w *countingWriter) Write(p []byte) (int, error) {
	n, err := w.hash.Write(p)
	w.bytes += int64(n)
	return n, err
}

func (w *Worker) process(ctx context.Context, job *model.ImageJob) error {
	heartbeatCtx, heartbeatCancel := context.WithCancel(ctx)
	defer heartbeatCancel()
	go w.heartbeat(heartbeatCtx, job.ID)
	image, err := w.DL.Get(ctx, job.ImageURL)
	if err != nil {
		return err
	}
	defer image.Body.Close()
	digest := &countingWriter{hash: sha256.New()}
	response, err := w.API.Upload(ctx, job, io.TeeReader(image.Body, digest), image.ContentType)
	if err != nil {
		return err
	}
	response.Body.Close()
	if err := w.API.Complete(ctx, job, model.CompleteRequest{WorkerID: w.API.WorkerID, SHA256: fmt.Sprintf("%x", digest.hash.Sum(nil)), Bytes: digest.bytes, ContentType: image.ContentType}); err != nil {
		return err
	}
	w.Stats.BytesTransferred.Add(digest.bytes)
	return nil
}

func (w *Worker) heartbeat(ctx context.Context, jobID string) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	beat := func() {
		callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		if err := w.API.Heartbeat(callCtx, jobID); err != nil {
			w.Log.Warn("heartbeat failed", "jobId", jobID, "error", err)
		}
		cancel()
	}
	beat()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			beat()
		}
	}
}

func short(message string) string {
	if len(message) > 500 {
		return message[:500]
	}
	return message
}

func Health(stats *Stats, workerID string) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("Cache-Control", "no-store")
		if request.URL.Path == "/health" {
			_ = json.NewEncoder(writer).Encode(map[string]any{"status": "ok", "workerId": workerID})
			return
		}
		if request.URL.Path != "/stats" {
			http.NotFound(writer, request)
			return
		}
		_ = json.NewEncoder(writer).Encode(map[string]int64{"jobsClaimed": stats.JobsClaimed.Load(), "jobsCompleted": stats.JobsCompleted.Load(), "jobsFailed": stats.JobsFailed.Load(), "bytesTransferred": stats.BytesTransferred.Load(), "retries": stats.Retries.Load()})
	})
}

func SourceHost(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return parsed.Hostname()
}
