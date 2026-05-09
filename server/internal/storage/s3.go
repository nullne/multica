package storage

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type S3Storage struct {
	client       *s3.Client
	bucket       string
	publicBase   string // public URL prefix, always ends with "/"
	gcsEndpoint  string
	gcsAccessKey string
	gcsSecretKey string
	httpClient   *http.Client
	useGCSXML    bool
}

// NewS3StorageFromEnv creates an S3Storage from environment variables.
// Returns nil if S3_BUCKET is not set.
//
// Environment variables:
//   - S3_BUCKET (required)
//   - S3_REGION (default: us-west-2)
//   - S3_ENDPOINT (optional) — custom S3-compatible API endpoint
//     (e.g. https://storage.googleapis.com for GCS,
//     https://<account>.r2.cloudflarestorage.com for Cloudflare R2)
//   - S3_USE_PATH_STYLE (optional) — "true"/"false". Defaults to true when
//     S3_ENDPOINT is set (required by GCS / MinIO), false otherwise.
//   - S3_PUBLIC_URL_BASE (optional) — explicit prefix used to build public
//     URLs returned by Upload. Falls back to CLOUDFRONT_DOMAIN, then to a
//     value derived from S3_ENDPOINT and the bucket name, then to the bucket
//     name (legacy AWS behavior).
//   - CLOUDFRONT_DOMAIN (optional) — used as the public URL host when
//     S3_PUBLIC_URL_BASE is empty.
//   - AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY (optional; falls back to
//     default credential chain). For GCS use HMAC keys; for R2 use the R2
//     access key pair.
func NewS3StorageFromEnv() *S3Storage {
	bucket := os.Getenv("S3_BUCKET")
	if bucket == "" {
		slog.Info("S3_BUCKET not set, file upload disabled")
		return nil
	}

	region := os.Getenv("S3_REGION")
	if region == "" {
		region = "us-west-2"
	}

	opts := []func(*config.LoadOptions) error{
		config.WithRegion(region),
	}

	accessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	if accessKey != "" && secretKey != "" {
		opts = append(opts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		))
	}

	cfg, err := config.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		slog.Error("failed to load AWS config", "error", err)
		return nil
	}

	endpoint := strings.TrimRight(os.Getenv("S3_ENDPOINT"), "/")
	pathStyle := resolvePathStyle(endpoint, os.Getenv("S3_USE_PATH_STYLE"))

	useGCSXML := isGCSEndpoint(endpoint) && accessKey != "" && secretKey != ""
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
		o.UsePathStyle = pathStyle
	})

	publicBase := resolvePublicBase(
		os.Getenv("S3_PUBLIC_URL_BASE"),
		os.Getenv("CLOUDFRONT_DOMAIN"),
		endpoint,
		bucket,
		pathStyle,
	)

	slog.Info("S3 storage initialized",
		"bucket", bucket,
		"region", region,
		"endpoint", endpoint,
		"path_style", pathStyle,
		"public_base", publicBase,
	)
	return &S3Storage{
		client:       client,
		bucket:       bucket,
		publicBase:   publicBase,
		gcsEndpoint:  endpoint,
		gcsAccessKey: accessKey,
		gcsSecretKey: secretKey,
		httpClient:   http.DefaultClient,
		useGCSXML:    useGCSXML,
	}
}

func isGCSEndpoint(endpoint string) bool {
	host := strings.TrimPrefix(strings.TrimPrefix(endpoint, "https://"), "http://")
	host = strings.TrimSuffix(host, "/")
	return host == "storage.googleapis.com"
}

// resolvePathStyle returns whether the S3 client should use path-style
// addressing. When the user sets S3_USE_PATH_STYLE explicitly we honor it;
// otherwise we default to true if a custom endpoint is configured (most
// S3-compatible services require it) and false for AWS S3 itself.
func resolvePathStyle(endpoint, raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "true", "1", "yes":
		return true
	case "false", "0", "no":
		return false
	}
	return endpoint != ""
}

// resolvePublicBase computes the URL prefix used to build object URLs.
// The returned value always ends with "/" so callers can simply append the
// object key.
func resolvePublicBase(explicit, cdnDomain, endpoint, bucket string, pathStyle bool) string {
	if explicit != "" {
		return ensureTrailingSlash(explicit)
	}
	if cdnDomain != "" {
		return ensureTrailingSlash("https://" + cdnDomain)
	}
	if endpoint != "" {
		if pathStyle {
			return ensureTrailingSlash(endpoint + "/" + bucket)
		}
		return ensureTrailingSlash(injectBucketIntoHost(endpoint, bucket))
	}
	return ensureTrailingSlash("https://" + bucket)
}

func ensureTrailingSlash(s string) string {
	if strings.HasSuffix(s, "/") {
		return s
	}
	return s + "/"
}

// injectBucketIntoHost rewrites "https://host[/path]" into
// "https://bucket.host[/path]" for virtual-hosted addressing against a
// custom endpoint.
func injectBucketIntoHost(endpoint, bucket string) string {
	scheme := "https://"
	rest := endpoint
	switch {
	case strings.HasPrefix(rest, "https://"):
		rest = strings.TrimPrefix(rest, "https://")
	case strings.HasPrefix(rest, "http://"):
		scheme = "http://"
		rest = strings.TrimPrefix(rest, "http://")
	}
	return scheme + bucket + "." + rest
}

// sanitizeFilename removes characters that could cause header injection in Content-Disposition.
func sanitizeFilename(name string) string {
	var b strings.Builder
	b.Grow(len(name))
	for _, r := range name {
		// Strip control chars, newlines, null bytes, quotes, semicolons, backslashes
		if r < 0x20 || r == 0x7f || r == '"' || r == ';' || r == '\\' || r == '\x00' {
			b.WriteRune('_')
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// KeyFromURL extracts the object key from a public URL produced by Upload.
// Falls back to the substring after the last "/" when the prefix does not
// match (e.g. URLs produced by an older configuration).
func (s *S3Storage) KeyFromURL(rawURL string) string {
	if s.publicBase != "" && strings.HasPrefix(rawURL, s.publicBase) {
		return strings.TrimPrefix(rawURL, s.publicBase)
	}
	if i := strings.LastIndex(rawURL, "/"); i >= 0 {
		return rawURL[i+1:]
	}
	return rawURL
}

// Delete removes an object from the bucket. Errors are logged but not fatal.
func (s *S3Storage) Delete(ctx context.Context, key string) {
	if key == "" {
		return
	}
	if s.useGCSXML {
		if err := s.deleteGCSObject(ctx, key); err != nil {
			slog.Error("gcs DeleteObject failed", "key", key, "error", err)
		}
		return
	}
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		slog.Error("s3 DeleteObject failed", "key", key, "error", err)
	}
}

// DeleteKeys removes multiple objects. Best-effort, errors are logged.
func (s *S3Storage) DeleteKeys(ctx context.Context, keys []string) {
	for _, key := range keys {
		s.Delete(ctx, key)
	}
}

func (s *S3Storage) Upload(ctx context.Context, key string, data []byte, contentType string, filename string) (string, error) {
	safe := sanitizeFilename(filename)
	if s.useGCSXML {
		if err := s.uploadGCSObject(ctx, key, data, contentType, safe); err != nil {
			return "", err
		}
		return s.publicBase + key, nil
	}

	input := &s3.PutObjectInput{
		Bucket:             aws.String(s.bucket),
		Key:                aws.String(key),
		Body:               bytes.NewReader(data),
		ContentType:        aws.String(contentType),
		ContentDisposition: aws.String(fmt.Sprintf(`inline; filename="%s"`, safe)),
		CacheControl:       aws.String("max-age=432000,public"),
	}
	if s.gcsEndpoint == "" {
		input.StorageClass = types.StorageClassIntelligentTiering
	}

	_, err := s.client.PutObject(ctx, input)
	if err != nil {
		return "", fmt.Errorf("s3 PutObject: %w", err)
	}

	return s.publicBase + key, nil
}

func (s *S3Storage) uploadGCSObject(ctx context.Context, key string, data []byte, contentType string, safeFilename string) error {
	req, err := s.newGCSXMLRequest(ctx, http.MethodPut, key, bytes.NewReader(data), contentType)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, safeFilename))
	req.Header.Set("Cache-Control", "max-age=432000,public")
	return s.doGCSXML(req)
}

func (s *S3Storage) deleteGCSObject(ctx context.Context, key string) error {
	req, err := s.newGCSXMLRequest(ctx, http.MethodDelete, key, nil, "")
	if err != nil {
		return err
	}
	return s.doGCSXML(req)
}

func (s *S3Storage) newGCSXMLRequest(ctx context.Context, method string, key string, body io.Reader, contentType string) (*http.Request, error) {
	endpoint := strings.TrimRight(s.gcsEndpoint, "/")
	objectPath := escapedObjectPath(s.bucket, key)
	req, err := http.NewRequestWithContext(ctx, method, endpoint+objectPath, body)
	if err != nil {
		return nil, err
	}
	date := time.Now().UTC().Format(http.TimeFormat)
	req.Header.Set("Date", date)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	signature := s.gcsXMLSignature(method, contentType, date, objectPath)
	req.Header.Set("Authorization", "AWS "+s.gcsAccessKey+":"+signature)
	return req, nil
}

func escapedObjectPath(bucket, key string) string {
	segments := append([]string{bucket}, strings.Split(key, "/")...)
	for i, segment := range segments {
		segments[i] = url.PathEscape(segment)
	}
	return "/" + strings.Join(segments, "/")
}

func (s *S3Storage) gcsXMLSignature(method, contentType, date, objectPath string) string {
	stringToSign := method + "\n\n" + contentType + "\n" + date + "\n" + objectPath
	mac := hmac.New(sha1.New, []byte(s.gcsSecretKey))
	mac.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func (s *S3Storage) doGCSXML(req *http.Request) error {
	client := s.httpClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("gcs xml %s: %w", req.Method, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return fmt.Errorf("gcs xml %s: status %d: %s", req.Method, resp.StatusCode, strings.TrimSpace(string(body)))
}
