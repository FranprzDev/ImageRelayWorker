package downloader

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"imagerelayworker/internal/retry"
)

var validContentTypes = map[string]bool{"image/jpeg": true, "image/png": true, "image/webp": true, "image/gif": true, "image/avif": true}

type Image struct {
	Body        io.ReadCloser
	ContentType string
	Size        int64
}
type Downloader struct {
	Client    *http.Client
	Timeout   time.Duration
	MaxBytes  int64
	UserAgent string
	AllowHTTP bool
}

func (d *Downloader) Validate(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil || u.User != nil || u.Hostname() == "" {
		return retry.PermanentError{Err: fmt.Errorf("invalid image URL")}
	}
	if u.Scheme != "https" && !(d.AllowHTTP && u.Scheme == "http") {
		return retry.PermanentError{Err: fmt.Errorf("only HTTPS image URLs are allowed")}
	}
	return validateHost(u.Hostname())
}

func validateHost(host string) error {
	if strings.EqualFold(host, "localhost") {
		return retry.PermanentError{Err: fmt.Errorf("blocked host")}
	}
	ips := net.ParseIP(host)
	if ips == nil {
		resolved, err := net.LookupIP(host)
		if err != nil {
			return retry.RetryableError{Err: err}
		}
		if len(resolved) == 0 {
			return retry.PermanentError{Err: fmt.Errorf("host has no addresses")}
		}
		for _, ip := range resolved {
			if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() || ip.IsLinkLocalMulticast() {
				return retry.PermanentError{Err: fmt.Errorf("blocked private address")}
			}
		}
		return nil
	}
	if ips.IsLoopback() || ips.IsPrivate() || ips.IsLinkLocalUnicast() || ips.IsUnspecified() || ips.IsLinkLocalMulticast() {
		return retry.PermanentError{Err: fmt.Errorf("blocked private address")}
	}
	return nil
}

type limitedBody struct {
	reader    io.Reader
	closer    io.Closer
	read, max int64
}

func (b *limitedBody) Read(p []byte) (int, error) {
	if b.read >= b.max {
		var one [1]byte
		n, err := b.reader.Read(one[:])
		if n > 0 {
			return 0, fmt.Errorf("image exceeds maximum size")
		}
		return 0, err
	}
	if remaining := b.max - b.read; int64(len(p)) > remaining {
		p = p[:remaining]
	}
	n, err := b.reader.Read(p)
	b.read += int64(n)
	if err == io.EOF && b.read == 0 {
		return 0, fmt.Errorf("empty image")
	}
	return n, err
}
func (b *limitedBody) Close() error { return b.closer.Close() }

func (d *Downloader) Get(ctx context.Context, rawURL string) (Image, error) {
	if err := d.Validate(rawURL); err != nil {
		return Image{}, err
	}
	client := *d.Client
	client.CheckRedirect = func(req *http.Request, _ []*http.Request) error { return d.Validate(req.URL.String()) }
	requestCtx, cancel := context.WithTimeout(ctx, d.Timeout)
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, rawURL, nil)
	if err != nil {
		cancel()
		return Image{}, retry.PermanentError{Err: err}
	}
	req.Header.Set("User-Agent", d.UserAgent)
	req.Header.Set("Accept", "image/avif,image/webp,image/png,image/jpeg,image/*")
	resp, err := client.Do(req)
	if err != nil {
		cancel()
		return Image{}, retry.RetryableError{Err: err}
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		resp.Body.Close()
		cancel()
		statusErr := fmt.Errorf("image returned HTTP %d", resp.StatusCode)
		if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound {
			return Image{}, retry.PermanentError{Err: statusErr}
		}
		return Image{}, retry.RetryableError{Err: statusErr}
	}
	if resp.ContentLength > d.MaxBytes || resp.ContentLength == 0 {
		resp.Body.Close()
		cancel()
		return Image{}, retry.PermanentError{Err: fmt.Errorf("empty or oversized image")}
	}
	contentType := strings.ToLower(strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0]))
	reader := bufio.NewReader(resp.Body)
	if contentType == "" {
		peek, peekErr := reader.Peek(512)
		if peekErr != nil && peekErr != io.EOF {
			resp.Body.Close()
			cancel()
			return Image{}, retry.RetryableError{Err: peekErr}
		}
		contentType = http.DetectContentType(peek)
	}
	if !validContentTypes[contentType] {
		resp.Body.Close()
		cancel()
		return Image{}, retry.PermanentError{Err: fmt.Errorf("invalid image content type %q", contentType)}
	}
	return Image{Body: &limitedBody{reader: reader, closer: closeOnce{resp.Body, cancel}, max: d.MaxBytes}, ContentType: contentType, Size: resp.ContentLength}, nil
}

type closeOnce struct {
	closer io.Closer
	cancel context.CancelFunc
}

func (c closeOnce) Close() error { c.cancel(); return c.closer.Close() }
