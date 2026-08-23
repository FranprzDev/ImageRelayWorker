package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"imagerelayworker/internal/model"
	"imagerelayworker/internal/retry"
)

type HTTPError struct {
	StatusCode int
	Operation  string
}

func (e HTTPError) Error() string { return fmt.Sprintf("%s HTTP %d", e.Operation, e.StatusCode) }

type Client struct {
	BaseURL, Token, WorkerID string
	HTTP                     *http.Client
	UploadTimeout            time.Duration
}

func (c *Client) request(ctx context.Context, method, path string, body io.Reader, contentType string) (*http.Response, error) {
	return c.requestWithHeaders(ctx, method, path, body, contentType, nil)
}

func (c *Client) requestWithHeaders(ctx context.Context, method, path string, body io.Reader, contentType string, extra map[string]string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.BaseURL, "/")+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("X-Worker-Id", c.WorkerID)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for key, value := range extra {
		req.Header.Set(key, value)
	}
	return c.HTTP.Do(req)
}

func (c *Client) Claim(ctx context.Context) (*model.ImageJob, error) {
	body, err := json.Marshal(model.ClaimRequest{WorkerID: c.WorkerID})
	if err != nil {
		return nil, err
	}
	resp, err := c.request(ctx, http.MethodPost, "/api/image-jobs/claim", bytes.NewReader(body), "application/json")
	if err != nil {
		return nil, retry.RetryableError{Err: err}
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, classify(HTTPError{resp.StatusCode, "claim"})
	}
	var result model.ClaimResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Job, nil
}

func (c *Client) Upload(ctx context.Context, job *model.ImageJob, body io.Reader, contentType string) (*http.Response, error) {
	ctx, cancel := context.WithTimeout(ctx, c.UploadTimeout)
	defer cancel()
	resp, err := c.requestWithHeaders(ctx, http.MethodPost, "/api/image-jobs/"+job.ID+"/upload", body, contentType, map[string]string{"X-Source-Url": job.ImageURL, "X-Product-Id": job.ProductID})
	if err != nil {
		return nil, retry.RetryableError{Err: err}
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		return nil, classify(HTTPError{resp.StatusCode, "upload"})
	}
	return resp, nil
}

func (c *Client) Complete(ctx context.Context, job *model.ImageJob, request model.CompleteRequest) error {
	return c.json(ctx, "/api/image-jobs/"+job.ID+"/complete", request)
}

func (c *Client) Fail(ctx context.Context, job *model.ImageJob, message string) error {
	return c.json(ctx, "/api/image-jobs/"+job.ID+"/fail", model.FailRequest{WorkerID: c.WorkerID, Error: message})
}

func (c *Client) json(ctx context.Context, path string, value any) error {
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}
	resp, err := c.request(ctx, http.MethodPost, path, bytes.NewReader(body), "application/json")
	if err != nil {
		return retry.RetryableError{Err: err}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		return classify(HTTPError{resp.StatusCode, path})
	}
	return nil
}

func classify(err HTTPError) error {
	if err.StatusCode == http.StatusTooManyRequests || err.StatusCode >= 500 {
		return retry.RetryableError{Err: err}
	}
	return retry.PermanentError{Err: err}
}
