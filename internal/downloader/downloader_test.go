package downloader

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestValidateSSRF(t *testing.T) {
	d := &Downloader{AllowHTTP: true}
	for _, u := range []string{"http://localhost/x", "http://127.0.0.1/x", "ftp://example.com/x"} {
		if d.Validate(u) == nil {
			t.Errorf("accepted %s", u)
		}
	}
}
func TestStreamImage(t *testing.T) {
	want := []byte("not really jpeg")
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write(want)
	}))
	defer s.Close()
	serverURL, _ := url.Parse(s.URL)
	client := s.Client()
	originalTransport := client.Transport
	client.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		copy := req.Clone(req.Context())
		copy.URL.Scheme = serverURL.Scheme
		copy.URL.Host = serverURL.Host
		return originalTransport.RoundTrip(copy)
	})
	d := &Downloader{Client: client, AllowHTTP: true, Timeout: time.Second, MaxBytes: 100, UserAgent: "test"}
	im, e := d.Get(context.Background(), "http://example.com/image.jpg")
	if e != nil {
		t.Fatal(e)
	}
	got, _ := io.ReadAll(im.Body)
	if !strings.EqualFold(string(got), string(want)) {
		t.Fatalf("%q", got)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
