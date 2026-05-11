package storage

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestGCSXMLDownloadUsesHMACRequest verifies the new app-authenticated
// download path: the server fetches a private GCS object using its own HMAC
// credentials and returns the bytes, so agents/CLIs can read attachments
// without the bucket being publicly readable.
func TestGCSXMLDownloadUsesHMACRequest(t *testing.T) {
	const wantBody = "hello multica"
	var sawGet bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawGet = true
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/test-bucket/path/to/object.txt" {
			t.Fatalf("path = %s, want /test-bucket/path/to/object.txt", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); !strings.HasPrefix(got, "AWS test-access:") {
			t.Fatalf("authorization header = %q, want AWS HMAC header", got)
		}
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(wantBody))
	}))
	defer server.Close()

	storage := &S3Storage{
		bucket:       "test-bucket",
		publicBase:   "https://cdn.example/",
		gcsEndpoint:  server.URL,
		gcsAccessKey: "test-access",
		gcsSecretKey: "test-secret",
		httpClient:   server.Client(),
		useGCSXML:    true,
	}

	res, err := storage.Download(context.Background(), "path/to/object.txt")
	if err != nil {
		t.Fatalf("Download returned error: %v", err)
	}
	if !sawGet {
		t.Fatal("server did not receive GET request")
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(body) != wantBody {
		t.Fatalf("body = %q, want %q", body, wantBody)
	}
	if res.ContentType != "text/plain" {
		t.Fatalf("content type = %q, want text/plain", res.ContentType)
	}
}

// TestGCSXMLDownloadPropagatesError ensures a non-2xx response from the
// backend bucket surfaces as an error rather than silently returning empty
// bytes — the download handler counts on this to return 502 to the caller.
func TestGCSXMLDownloadPropagatesError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("<Error>nope</Error>"))
	}))
	defer server.Close()

	storage := &S3Storage{
		bucket:       "test-bucket",
		publicBase:   "https://cdn.example/",
		gcsEndpoint:  server.URL,
		gcsAccessKey: "test-access",
		gcsSecretKey: "test-secret",
		httpClient:   server.Client(),
		useGCSXML:    true,
	}

	if _, err := storage.Download(context.Background(), "path/to/object.txt"); err == nil {
		t.Fatal("expected error for 403 from backend, got nil")
	}
}

func TestGCSXMLUploadUsesHMACRequestWithoutAWSStorageClass(t *testing.T) {
	var sawPut bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawPut = true
		if r.Method != http.MethodPut {
			t.Fatalf("method = %s, want PUT", r.Method)
		}
		if r.URL.Path != "/test-bucket/path/to/object.txt" {
			t.Fatalf("path = %s, want /test-bucket/path/to/object.txt", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); !strings.HasPrefix(got, "AWS test-access:") {
			t.Fatalf("authorization header = %q, want AWS HMAC header", got)
		}
		if got := r.Header.Get("X-Amz-Storage-Class"); got != "" {
			t.Fatalf("X-Amz-Storage-Class = %q, want empty", got)
		}
		if got := r.Header.Get("Content-Disposition"); got != `inline; filename="bad_name.txt"` {
			t.Fatalf("Content-Disposition = %q, want sanitized filename", got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		if string(body) != "hello" {
			t.Fatalf("body = %q, want hello", string(body))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	storage := &S3Storage{
		bucket:       "test-bucket",
		publicBase:   "https://cdn.example/",
		gcsEndpoint:  server.URL,
		gcsAccessKey: "test-access",
		gcsSecretKey: "test-secret",
		httpClient:   server.Client(),
		useGCSXML:    true,
	}

	url, err := storage.Upload(context.Background(), "path/to/object.txt", []byte("hello"), "text/plain", "bad\nname.txt")
	if err != nil {
		t.Fatalf("Upload returned error: %v", err)
	}
	if !sawPut {
		t.Fatal("server did not receive PUT request")
	}
	if url != "https://cdn.example/path/to/object.txt" {
		t.Fatalf("url = %q, want public URL", url)
	}
}
