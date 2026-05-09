package storage

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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
