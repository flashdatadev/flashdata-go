package flashdata

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const testKey = "fd_test_offline_example_123456789"

func testClient(t *testing.T, h http.HandlerFunc, options ...func(*Options)) (*Client, *httptest.Server) {
	t.Helper()
	s := httptest.NewServer(h)
	t.Cleanup(s.Close)
	o := Options{APIKey: testKey, DataOrigin: s.URL, ManagementOrigin: s.URL, Timeout: time.Second}
	for _, option := range options {
		option(&o)
	}
	c, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	return c, s
}
func respond(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func TestOriginsAndConfiguration(t *testing.T) {
	for _, v := range []string{"https://user:password@example.com", "http://example.com", "https://example.com/path", "https://example.com?a=b", "https://example.com#frag", "file:///tmp/socket", "https://example.com?", ""} {
		t.Run(v, func(t *testing.T) {
			if _, err := origin(v); err == nil {
				t.Fatal("accepted unsafe origin")
			}
		})
	}
	for _, v := range []string{"https://api.flashdata.dev", "http://localhost:5000", "http://127.0.0.1:8080", "http://[::1]:8000"} {
		if _, err := origin(v); err != nil {
			t.Fatal(err)
		}
	}
	c, err := New(Options{APIKey: testKey})
	if err != nil {
		t.Fatal(err)
	}
	if c.dataOrigin != "https://data.flashdata.dev" || c.managementOrigin != "https://api.flashdata.dev" {
		t.Fatal("wrong production origins")
	}
	if strings.Contains(fmt.Sprintf("%#v", c), testKey) {
		t.Fatal("client formatting leaked key")
	}
	for _, o := range []Options{{APIKey: "secret\nheader"}, {Timeout: -1}, {MaxLineBytes: -1}, {MaxResponseBytes: -1}} {
		if _, err := New(o); err == nil {
			t.Fatal("accepted invalid options")
		}
	}
}

func TestAuthModesAndNoEnvironmentLoading(t *testing.T) {
	t.Setenv("FLASHDATA_API_KEY", "fd_must_not_autoload")
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "" || r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" {
			t.Error("public request had credentials")
		}
		respond(w, map[string]any{"data": []any{}})
	}, func(o *Options) { o.APIKey = "" })
	if _, err := c.Public.Sources(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Account.Balance(context.Background()); err == nil {
		t.Fatal("missing explicit key accepted")
	}
}

func TestSourceContractFixtures(t *testing.T) {
	b, err := os.ReadFile("testdata/source-examples.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures map[string]struct {
		Request  json.RawMessage
		Response DataQueryResponse
	}
	if err = json.Unmarshal(b, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 15 {
		t.Fatal("lost source coverage")
	}
	for source, f := range fixtures {
		t.Run(source, func(t *testing.T) {
			var q Query
			if err := json.Unmarshal(f.Request, &q); err != nil {
				t.Fatal(err)
			}
			if err := validateQuery(q, true); err != nil {
				t.Fatal(err)
			}
			c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" || r.Header.Get("X-API-Key") != testKey || r.Header.Get("Authorization") != "" {
					t.Error("wrong query authentication/method")
				}
				var got Query
				_ = json.NewDecoder(r.Body).Decode(&got)
				if got.Source != q.Source {
					t.Error("wrong source")
				}
				if strings.HasSuffix(r.URL.Path, "realtime") {
					respond(w, f.Response)
				} else {
					respond(w, map[string]any{"id": "job_example", "status": "pending", "source": source})
				}
			})
			if _, err := c.Queries.Submit(context.Background(), q); err != nil {
				t.Fatal(err)
			}
			if source != "youtube_download" {
				if _, err := c.Queries.Realtime(context.Background(), q); err != nil {
					t.Fatal(err)
				}
			} else if err := validateQuery(q, false); err == nil {
				t.Fatal("realtime download accepted")
			}
			q.Params = map[string]any{"unknown_parameter": true}
			if err := validateQuery(q, true); err == nil {
				t.Fatal("unsupported parameter accepted")
			}
		})
	}
}

func TestLocalValidationBeforeDispatch(t *testing.T) {
	var calls atomic.Int32
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(500) })
	for _, q := range []Query{
		{Source: "unknown", Query: "hello"}, {Source: SourceGoogleSearch, Query: " "},
		{Source: SourceGoogleSearch, Query: "hello", Params: map[string]any{"num": 101}},
		{Source: SourceGoogleSearch, Query: "hello", Params: map[string]any{"page": 0}},
		{Source: SourceGoogleSearch, Query: "hello", Params: map[string]any{"gl": "USA"}},
		{Source: SourceGoogleSearch, Query: "hello", VideoID: "dQw4w9WgXcQ"},
		{Source: SourceYouTubeMetadata, VideoID: "bad"},
		{Source: SourceYouTubeDownload, VideoID: "dQw4w9WgXcQ"},
		{Source: SourceYouTubeSearch, Query: "hello", Params: map[string]any{"filters": map[string]any{"duration": "invalid"}}},
		{Source: SourceGoogleSearch, Query: "hello", Params: make(chan int)},
	} {
		if _, err := c.Queries.Submit(context.Background(), q); err == nil {
			t.Errorf("accepted invalid query for %s", q.Source)
		}
	}
	if _, err := c.Queries.Batch(context.Background(), nil); err == nil {
		t.Fatal("empty batch accepted")
	}
	for _, callback := range []string{"http://example.com", "https://127.0.0.1/notify", "https://10.0.0.1/notify", "https://[::1]/notify", "https://a:b@example.com", "https://example.com:444/notify", "https://example.com/#secret", "https://192.0.2.1/"} {
		if err := validateCallback(callback); err == nil {
			t.Errorf("accepted %s", callback)
		}
	}
	if err := validateCallback("https://example.com:443/notify"); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 0 {
		t.Fatal("invalid request reached transport")
	}
}

func TestExplicitFalseAndRawPayloadNumbers(t *testing.T) {
	q := (YouTubeCaptionsInput{VideoID: "dQw4w9WgXcQ", Params: &YouTubeCaptionsParams{Auto: Ptr(false)}}).Request()
	b, _ := json.Marshal(q)
	if !bytes.Contains(b, []byte(`"auto":false`)) {
		t.Fatal("explicit false lost")
	}
	var result DataQueryResponse
	if err := json.Unmarshal([]byte(`{"source":"google_search","status":"done","results":[{"future":9007199254740993123}]}`), &result); err != nil {
		t.Fatal(err)
	}
	v, err := DecodeResults[map[string]any](&result)
	if err != nil {
		t.Fatal(err)
	}
	if v[0]["future"].(json.Number).String() != "9007199254740993123" {
		t.Fatal("large integer rounded")
	}
}

func TestRejectedMutationRedactsSecretsAndNeverRetries(t *testing.T) {
	var calls atomic.Int32
	storageSecret := "MY_PRIVATE_STORAGE_VALUE_123"
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Retry-After", "3")
		w.Header().Set("X-Request-ID", "request-123")
		w.WriteHeader(429)
		respond(w, map[string]any{"code": "rate_limited", "detail": testKey + " " + storageSecret + " https://storage.example/part?signature=private"})
	})
	q := Query{Source: SourceYouTubeDownload, VideoID: "dQw4w9WgXcQ", Params: map[string]any{"storage": map[string]any{"storage_type": "s3", "storage_url": "bucket", "access_key": "access", "secret_key": storageSecret}}}
	_, err := c.Queries.Submit(context.Background(), q)
	var api *APIError
	if !errors.As(err, &api) || api.StatusCode != 429 || api.RetryAfter != "3" {
		t.Fatal(err)
	}
	if strings.Contains(err.Error(), storageSecret) || strings.Contains(err.Error(), testKey) || strings.Contains(err.Error(), "signature=") {
		t.Fatal("secret leaked")
	}
	if calls.Load() != 1 {
		t.Fatal("mutation retried")
	}
}

func TestAmbiguousSubmissionAndPreCancellation(t *testing.T) {
	var calls atomic.Int32
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		h, ok := w.(http.Hijacker)
		if !ok {
			t.Fatal("no hijacker")
		}
		conn, _, _ := h.Hijack()
		conn.Close()
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := c.Queries.Submit(ctx, Query{Source: SourceGoogleSearch, Query: "hello"})
	if !errors.Is(err, context.Canceled) || IsSubmissionUnknown(err) {
		t.Fatal(err)
	}
	if calls.Load() != 0 {
		t.Fatal("pre-cancel dispatched")
	}
	_, err = c.Queries.Submit(context.Background(), Query{Source: SourceGoogleSearch, Query: "hello"})
	if !IsSubmissionUnknown(err) {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatal("submission replayed")
	}
}

func TestMalformedAndOversizedResponses(t *testing.T) {
	for _, body := range []string{"null", "[]", "{", "\xff", strings.Repeat(" ", 300)} {
		t.Run(fmt.Sprintf("%x", body[:1]), func(t *testing.T) {
			c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, body) }, func(o *Options) { o.MaxResponseBytes = 100 })
			if _, err := c.Account.Balance(context.Background()); err == nil {
				t.Fatal("bad response accepted")
			}
			if _, err := c.Queries.Submit(context.Background(), Query{Source: SourceGoogleSearch, Query: "hello"}); !IsSubmissionUnknown(err) {
				t.Fatal("mutation acceptance guessed", err)
			}
		})
	}
}

func gzipBytes(t *testing.T, s string) []byte {
	t.Helper()
	var b bytes.Buffer
	g := gzip.NewWriter(&b)
	_, _ = io.WriteString(g, s)
	if err := g.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
