package flashdata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type formatCase struct {
	Accept      Accept          `json:"accept"`
	RespondWith Representation  `json:"respondWith"`
	Response    json.RawMessage `json:"response"`
}

func readFormatFixtures(t *testing.T, name string) map[Source][]formatCase {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	var fixtures map[Source][]formatCase
	if err = json.Unmarshal(b, &fixtures); err != nil {
		t.Fatal(err)
	}
	return fixtures
}

func sourceQueries(t *testing.T) map[Source]Query {
	t.Helper()
	b, err := os.ReadFile("testdata/source-examples.json")
	if err != nil {
		t.Fatal(err)
	}
	var examples map[Source]struct{ Request Query }
	if err = json.Unmarshal(b, &examples); err != nil {
		t.Fatal(err)
	}
	queries := map[Source]Query{}
	for source, example := range examples {
		queries[source] = example.Request
	}
	return queries
}

func serveFormat(t *testing.T, w http.ResponseWriter, r *http.Request, v formatCase) {
	t.Helper()
	if r.Header.Get("Accept") != string(v.Accept) || r.Header.Get("X-Respond-With") != string(v.RespondWith) || r.Header.Get("X-API-Key") != testKey {
		t.Errorf("unexpected format/auth headers: %v", r.Header)
	}
	w.Header().Set("Content-Type", string(v.Accept)+"; charset=utf-8")
	b := []byte(v.Response)
	if v.Accept == AcceptText {
		var text string
		if err := json.Unmarshal(b, &text); err != nil {
			t.Fatal(err)
		}
		b = []byte(text)
	}
	_, _ = w.Write(b)
}

func checkFormat(t *testing.T, result *FormattedResponse, v formatCase) {
	t.Helper()
	if result == nil {
		t.Fatal("missing formatted response")
	}
	if v.Accept == AcceptText {
		var want string
		_ = json.Unmarshal(v.Response, &want)
		if result.JSON != nil || result.Text != want {
			t.Fatal("text response changed")
		}
		if result.DecodeJSON(&map[string]any{}) == nil {
			t.Fatal("decoded text as JSON")
		}
	} else {
		var got, want any
		_ = json.Unmarshal(result.JSON, &got)
		_ = json.Unmarshal(v.Response, &want)
		if result.Text != "" || !reflect.DeepEqual(got, want) {
			t.Fatal("JSON fields changed")
		}
		if v.RespondWith == RespondMarkdown {
			var markdown MarkdownQueryResult
			if err := result.DecodeJSON(&markdown); err != nil || markdown.Content == "" {
				t.Fatal("cannot decode Markdown wire model", err)
			}
		}
	}
}

var formatHelperNames = map[Source]string{
	SourceGoogleSearch: "Search", SourceGoogleImages: "Images", SourceGoogleNews: "News",
	SourceGoogleShopping: "Shopping", SourceGoogleVideos: "Videos", SourceGooglePlaces: "Places",
	SourceYouTubeSearch: "Search", SourceYouTubeSearchExtended: "SearchExtended",
	SourceYouTubeMetadata: "Metadata", SourceYouTubeTranscript: "Transcript",
	SourceYouTubeCaptions: "Captions", SourceYouTubeTrainability: "LicenseCheck",
	SourceYouTubeChannel: "Channel", SourceYouTubeSuggestions: "Suggestions", SourceYouTubeDownload: "Download",
}

// Reflection exercises every generated helper with its actual exported Input
// type, including callback placement and the distinct content/envelope options.
func callFormatHelper(t *testing.T, c *Client, q Query, async bool, options ContentOptions) (*FormattedResponse, error) {
	t.Helper()
	service := reflect.ValueOf(c.YouTube)
	if strings.HasPrefix(string(q.Source), "google_") {
		service = reflect.ValueOf(c.Google)
	}
	name := formatHelperNames[q.Source]
	if async && q.Source != SourceYouTubeDownload {
		name += "Async"
	}
	method := service.MethodByName(name + "WithFormat")
	if !method.IsValid() {
		t.Fatal("missing helper", name)
	}
	input := reflect.New(method.Type().In(1))
	b, _ := json.Marshal(q)
	if err := json.Unmarshal(b, input.Interface()); err != nil {
		t.Fatal(err)
	}
	args := []reflect.Value{reflect.ValueOf(context.Background()), input.Elem()}
	if async {
		args = append(args, reflect.ValueOf(q.CallbackURL), reflect.ValueOf(ResponseOptions{Accept: options.Accept}))
	} else {
		args = append(args, reflect.ValueOf(options))
	}
	values := method.Call(args)
	if !values[1].IsNil() {
		return nil, values[1].Interface().(error)
	}
	return values[0].Interface().(*FormattedResponse), nil
}

func TestRealtimeFormatsAllSourcesAndHelpers(t *testing.T) {
	queries := sourceQueries(t)
	fixtures := readFormatFixtures(t, "realtime-response-formats.json")
	if len(fixtures) != 14 {
		t.Fatal("expected all realtime sources")
	}
	for source, cases := range fixtures {
		for _, v := range cases {
			for _, helper := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/%s/helper=%v", source, v.Accept, v.RespondWith, helper), func(t *testing.T) {
					var calls atomic.Int32
					c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
						calls.Add(1)
						if r.Method != "POST" || r.URL.Path != "/v1/queries/realtime" {
							t.Error("wrong route")
						}
						var body map[string]json.RawMessage
						_ = json.NewDecoder(r.Body).Decode(&body)
						if body["accept"] != nil || body["respondWith"] != nil || body["callback_url"] != nil {
							t.Error("header options leaked into input")
						}
						serveFormat(t, w, r, v)
					})
					options := ContentOptions{Accept: v.Accept, RespondWith: v.RespondWith}
					var result *FormattedResponse
					var err error
					if helper {
						result, err = callFormatHelper(t, c, queries[source], false, options)
					} else {
						result, err = c.Queries.RealtimeWithFormat(context.Background(), queries[source], options)
					}
					if err != nil {
						t.Fatal(err)
					}
					checkFormat(t, result, v)
					if calls.Load() != 1 {
						t.Fatal("query repeated")
					}
				})
			}
		}
	}
}

func TestStoredFormatsAllSources(t *testing.T) {
	fixtures := readFormatFixtures(t, "stored-result-formats.json")
	if len(fixtures) != 15 {
		t.Fatal("expected all stored sources")
	}
	for source, cases := range fixtures {
		for _, v := range cases {
			t.Run(fmt.Sprintf("%s/%s/%s", source, v.Accept, v.RespondWith), func(t *testing.T) {
				var storageCalls, apiCalls atomic.Int32
				storage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					storageCalls.Add(1)
					if r.Header.Get("X-API-Key") != "" || r.Header.Get("Authorization") != "" || r.Header.Get("X-Respond-With") != "" || r.Header.Get("Cookie") != "" {
						t.Error("credentials or representation forwarded to storage")
					}
					w.Header().Set("Content-Type", "application/json")
					w.Header().Set("Content-Encoding", "gzip")
					_, _ = w.Write(gzipBytes(t, string(v.Response)))
				}))
				defer storage.Close()
				c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
					apiCalls.Add(1)
					if r.Method != "GET" || r.URL.Path != "/v2/jobs/job_fixture0001/results" {
						t.Error("wrong result route")
					}
					if v.Accept == AcceptJSON && v.RespondWith == "" {
						w.Header().Set("Location", storage.URL+"/object?signature=private")
						w.WriteHeader(302)
					} else {
						serveFormat(t, w, r, v)
					}
				}, func(o *Options) { o.ResultOrigin = storage.URL })
				result, err := c.Jobs.ResultWithFormat(context.Background(), "job_fixture0001", ContentOptions{Accept: v.Accept, RespondWith: v.RespondWith})
				if err != nil {
					t.Fatal(err)
				}
				checkFormat(t, result, v)
				wantStorage := int32(0)
				if v.Accept == AcceptJSON && v.RespondWith == "" {
					wantStorage = 1
				}
				if storageCalls.Load() != wantStorage || apiCalls.Load() != 1 {
					t.Fatal("unexpected result requests")
				}
				if v.Accept == AcceptJSON {
					var metadata struct {
						ID, Status  string
						CompletedAt string `json:"completed_at"`
						ExpiresAt   string `json:"expires_at"`
					}
					_ = result.DecodeJSON(&metadata)
					if metadata.ID != "job_fixture0001" || metadata.Status != "completed" || metadata.CompletedAt == "" || metadata.ExpiresAt == "" {
						t.Fatal("stored metadata missing")
					}
				}
			})
		}
	}
}

func TestReceiptAndStatusFormats(t *testing.T) {
	b, err := os.ReadFile("testdata/response-formats.json")
	if err != nil {
		t.Fatal(err)
	}
	var examples []struct {
		Group    string
		Request  json.RawMessage
		Headers  map[string]string
		Response json.RawMessage
	}
	if err = json.Unmarshal(b, &examples); err != nil {
		t.Fatal(err)
	}
	for _, example := range examples {
		if example.Group == "google_search" || example.Group == "youtube_transcript" {
			continue
		}
		t.Run(example.Group+"/"+example.Headers["Accept"], func(t *testing.T) {
			v := formatCase{Accept: Accept(example.Headers["Accept"]), Response: example.Response}
			c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) { serveFormat(t, w, r, v) })
			var result *FormattedResponse
			var err error
			options := ResponseOptions{Accept: v.Accept}
			switch example.Group {
			case "async":
				var q Query
				_ = json.Unmarshal(example.Request, &q)
				result, err = c.Queries.SubmitWithFormat(context.Background(), q, options)
			case "batch":
				var body struct{ Queries []Query }
				_ = json.Unmarshal(example.Request, &body)
				result, err = c.Queries.BatchWithFormat(context.Background(), body.Queries, options)
			case "status":
				result, err = c.Jobs.GetWithFormat(context.Background(), "job_example0001", options)
			default:
				t.Fatalf("unrecognized example group %s", example.Group)
			}
			if err != nil {
				t.Fatal(err)
			}
			checkFormat(t, result, v)
		})
	}
}

func TestAsyncFormatHelpersAllSources(t *testing.T) {
	for source, q := range sourceQueries(t) {
		for _, accept := range []Accept{AcceptJSON, AcceptText} {
			t.Run(string(source)+"/"+string(accept), func(t *testing.T) {
				q.CallbackURL = "https://customer.example/hooks/flashdata"
				var expected []byte
				if accept == AcceptJSON {
					expected, _ = json.Marshal(map[string]any{"id": "job_fixture0001", "source": source, "status": "pending"})
				} else {
					expected = []byte(fmt.Sprintf("Id: \"job_fixture0001\"\nSource: %q\nStatus: \"pending\"", source))
				}
				c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path != "/v1/queries" || r.Header.Get("Accept") != string(accept) || r.Header.Get("X-Respond-With") != "" {
						t.Error("wrong submission headers or path")
					}
					var body Query
					_ = json.NewDecoder(r.Body).Decode(&body)
					if body.Source != source || body.CallbackURL != q.CallbackURL {
						t.Error("lost source/callback")
					}
					w.Header().Set("Content-Type", string(accept))
					_, _ = w.Write(expected)
				})
				result, err := callFormatHelper(t, c, q, true, ContentOptions{Accept: accept})
				if err != nil {
					t.Fatal(err)
				}
				if accept == AcceptText && result.Text != string(expected) {
					t.Fatal("receipt text changed")
				}
			})
		}
	}
}

func TestInvalidFormatOptionsBeforeIO(t *testing.T) {
	var calls atomic.Int32
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(500) })
	ctx := context.Background()
	q := Query{Source: SourceGoogleSearch, Query: "offline"}
	for _, format := range []ContentOptions{{Accept: "text/html"}, {Accept: "*/*"}, {Accept: "text/plain; charset=utf-8"}, {RespondWith: "json"}, {RespondWith: "markdown\r\nInjected: true"}} {
		if _, err := c.Queries.RealtimeWithFormat(ctx, q, format); err == nil {
			t.Fatal("invalid format accepted")
		}
		if _, err := c.Jobs.ResultWithFormat(ctx, "job_example", format); err == nil {
			t.Fatal("invalid result format accepted")
		}
	}
	for _, call := range []func() error{
		func() error {
			_, err := c.Queries.SubmitWithFormat(ctx, q, ResponseOptions{Accept: "text/html"})
			return err
		},
		func() error {
			_, err := c.Queries.BatchWithFormat(ctx, []Query{q}, ResponseOptions{Accept: "text/html"})
			return err
		},
		func() error {
			_, err := c.Jobs.GetWithFormat(ctx, "job_example", ResponseOptions{Accept: "text/html"})
			return err
		},
		func() error {
			_, err := c.Queries.RealtimeWithFormat(ctx, Query{Source: SourceYouTubeDownload}, ContentOptions{})
			return err
		},
		func() error {
			_, err := c.Queries.RealtimeWithFormat(ctx, Query{Source: q.Source, Query: q.Query, CallbackURL: "https://customer.example/hooks"}, ContentOptions{})
			return err
		},
		func() error { _, err := c.Queries.BatchWithFormat(ctx, nil, ResponseOptions{}); return err },
		func() error { _, err := c.Jobs.GetWithFormat(ctx, "../bad", ResponseOptions{}); return err },
	} {
		if call() == nil {
			t.Fatal("invalid request accepted")
		}
	}
	if calls.Load() != 0 {
		t.Fatal("invalid options sent a request")
	}
	if reflect.TypeOf(ResponseOptions{}).NumField() != 1 {
		t.Fatal("receipt options must not expose content representation")
	}
}

func TestFormattedErrorsAndBodyValidation(t *testing.T) {
	ctx := context.Background()
	for _, code := range []int{400, 401, 406, 429, 500} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			var calls atomic.Int32
			c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Content-Type", "application/problem+json")
				w.Header().Set("Retry-After", "2")
				w.WriteHeader(code)
				_, _ = w.Write([]byte(`{"code":"rejected","detail":"safe problem"}`))
			})
			_, err := c.Queries.RealtimeWithFormat(ctx, Query{Source: SourceGoogleSearch, Query: "offline"}, ContentOptions{Accept: AcceptText, RespondWith: RespondMarkdown})
			var api *APIError
			if !errors.As(err, &api) || api.StatusCode != code || api.Code != "rejected" || api.RetryAfter != "2" || calls.Load() != 1 || IsSubmissionUnknown(err) {
				t.Fatal("lost structured API rejection", err)
			}
		})
	}
	for _, body := range []string{`null`, `[]`, `{"source":"google_search","status":"done","results":[]}`, `{"source":"wrong","status":"done","content":"# text"}`, `{"source":"google_search","status":"done","content":null}`, `{"source":"google_search","status":"done","content":"# text","results":null}`} {
		c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(body))
		})
		if _, err := c.Queries.RealtimeWithFormat(ctx, Query{Source: SourceGoogleSearch, Query: "offline"}, ContentOptions{RespondWith: RespondMarkdown}); !IsSubmissionUnknown(err) {
			t.Fatal("malformed mutation treated as successful", err)
		}
	}
	for _, test := range []struct {
		name, body, contentType, encoding string
		limit                             int64
		want                              error
	}{
		{"utf8", string([]byte{0xff}), "text/plain", "", 100, nil},
		{"limit", strings.Repeat("x", 101), "text/plain", "", 100, ErrResponseTooLarge},
		{"wrong-type", "hello", "application/json", "", 100, nil},
		{"charset", "hello", "text/plain; charset=latin1", "", 100, nil},
		{"encoding", "hello", "text/plain", "br", 100, nil},
		{"gzip-bomb", strings.Repeat("x", 1000), "text/plain", "gzip", 100, ErrResponseTooLarge},
	} {
		t.Run(test.name, func(t *testing.T) {
			c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", test.contentType)
				w.Header().Set("Content-Encoding", test.encoding)
				if test.encoding == "gzip" {
					_, _ = w.Write(gzipBytes(t, test.body))
				} else {
					_, _ = w.Write([]byte(test.body))
				}
			}, func(o *Options) { o.MaxResponseBytes = test.limit })
			_, err := c.Jobs.ResultWithFormat(ctx, "job_example", ContentOptions{Accept: AcceptText})
			if err == nil || (test.want != nil && !errors.Is(err, test.want)) {
				t.Fatal("invalid response accepted", err)
			}
		})
	}
}

func TestFormatTextIsNotARedirectAndPreservesNumbers(t *testing.T) {
	for _, text := range []string{"https://private.example/looks-like-a-url", "", "null", "来源 🎬\n# Markdown\n"} {
		c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
			w.Header().Set("Content-Encoding", "gzip")
			_, _ = w.Write(gzipBytes(t, text))
		})
		result, err := c.Jobs.ResultWithFormat(context.Background(), "job_example", ContentOptions{Accept: AcceptText})
		if err != nil || result.Text != text {
			t.Fatal("text changed or treated as redirect", err)
		}
	}
	const body = `{"source":"google_search","status":"done","results":[],"future":{"large":9007199254740993}}`
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	})
	result, err := c.Queries.RealtimeWithFormat(context.Background(), Query{Source: SourceGoogleSearch, Query: "offline"}, ContentOptions{})
	if err != nil || string(result.JSON) != body {
		t.Fatal("raw JSON lost fields or numeric precision", err)
	}
}

func TestFormattedReadCancellation(t *testing.T) {
	for _, mutation := range []bool{false, true} {
		for _, deadline := range []bool{false, true} {
			t.Run(fmt.Sprintf("mutation=%v/deadline=%v", mutation, deadline), func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "text/plain")
					w.WriteHeader(200)
					w.(http.Flusher).Flush()
					if !deadline {
						cancel()
					}
					<-r.Context().Done()
				}, func(o *Options) { o.Timeout = 50 * time.Millisecond })
				var err error
				if mutation {
					_, err = c.Queries.SubmitWithFormat(ctx, Query{Source: SourceGoogleSearch, Query: "offline"}, ResponseOptions{Accept: AcceptText})
				} else {
					_, err = c.Jobs.ResultWithFormat(ctx, "job_example", ContentOptions{Accept: AcceptText})
				}
				want := context.Canceled
				if deadline {
					want = context.DeadlineExceeded
				}
				if !errors.Is(err, want) || IsSubmissionUnknown(err) != mutation {
					t.Fatal("lost cancellation or submission uncertainty", err)
				}
			})
		}
	}
}

func TestFormattedRecoveryAndMetadataValidation(t *testing.T) {
	ctx := context.Background()
	q := Query{Source: SourceGoogleSearch, Query: "offline"}
	for _, batch := range []bool{false, true} {
		for _, body := range []string{
			`{"id":"job_recoverable","source":42,"status":"pending"}`,
			`{"id":"job_recoverable","source":"wrong","status":"pending"}`,
		} {
			c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if batch {
					_, _ = fmt.Fprintf(w, `{"queries":[%s]}`, body)
				} else {
					_, _ = w.Write([]byte(body))
				}
			})
			var err error
			if batch {
				_, err = c.Queries.BatchWithFormat(ctx, []Query{q}, ResponseOptions{})
			} else {
				_, err = c.Queries.SubmitWithFormat(ctx, q, ResponseOptions{})
			}
			var unknown *SubmissionUnknownError
			if !errors.As(err, &unknown) || !reflect.DeepEqual(unknown.JobIDs, []string{"job_recoverable"}) {
				t.Fatal("lost recovery ID", err)
			}
		}
	}
	for _, body := range []string{`{}`, `{"job":{"id":"job_wrong","status":"completed"}}`} {
		c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(body))
		})
		if _, err := c.Jobs.GetWithFormat(ctx, "job_example", ResponseOptions{}); err == nil {
			t.Fatal("invalid status accepted")
		}
	}
	for _, body := range []string{`{"content":"# text"}`, `{"source":"google_search","status":"completed","content":null}`, `{"source":"google_search","status":"completed","content":"# text","results":[]}`, `{"source":"google_search","status":"completed","content":"# text","id":"job_wrong"}`} {
		c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(body))
		})
		if _, err := c.Jobs.ResultWithFormat(ctx, "job_example", ContentOptions{RespondWith: RespondMarkdown}); err == nil || IsSubmissionUnknown(err) {
			t.Fatal("invalid Markdown read accepted or marked as submission", err)
		}
	}
	for _, body := range []string{`{"source":"google_search","status":"done","results":[],"usage":{"credits":"wrong"}}`, `{"source":"google_search","status":"done","results":[],"created_at":42}`} {
		c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(body))
		})
		if _, err := c.Queries.RealtimeWithFormat(ctx, q, ContentOptions{}); !IsSubmissionUnknown(err) {
			t.Fatal("formatted original bypassed typed response validation", err)
		}
	}
	var response *FormattedResponse
	if response.DecodeJSON(&JobResult{}) == nil {
		t.Fatal("nil response decoded")
	}
}

func TestFormatPreferencesDoNotLeak(t *testing.T) {
	var formatted atomic.Bool
	formatted.Store(true)
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if formatted.Load() {
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte("formatted"))
			return
		}
		if r.Header.Get("Accept") != "application/json" || r.Header.Get("X-Respond-With") != "" {
			t.Error("format leaked across calls", r.URL.Path)
		}
		switch r.URL.Path {
		case "/v1/queries/realtime":
			respond(w, map[string]any{"source": "google_search", "status": "done", "results": []any{}})
		case "/v2/jobs/job_example":
			respond(w, map[string]any{"job": map[string]any{"id": "job_example", "status": "completed"}})
		case "/v2/api-access/balance":
			respond(w, map[string]any{"credits": 1})
		case "/v2/catalog/sources":
			respond(w, map[string]any{"data": []any{}})
		case "/v2/bulk-jobs":
			respond(w, map[string]any{"jobs": []any{}, "hasMore": false})
		case "/v2/agent-connections/capabilities":
			respond(w, map[string]any{"protocolVersion": 2, "available": false})
		default:
			t.Error("unexpected route", r.URL.Path)
		}
	})
	ctx := context.Background()
	if _, err := c.Google.SearchWithFormat(ctx, GoogleSearchInput{Query: "offline"}, ContentOptions{Accept: AcceptText, RespondWith: RespondMarkdown}); err != nil {
		t.Fatal(err)
	}
	formatted.Store(false)
	for _, call := range []func() error{
		func() error { _, err := c.Google.Search(ctx, GoogleSearchInput{Query: "offline"}); return err },
		func() error { _, err := c.Jobs.Wait(ctx, "job_example", PollOptions{}); return err },
		func() error { _, err := c.Account.Balance(ctx); return err },
		func() error { _, err := c.Public.Sources(ctx); return err },
		func() error { _, err := c.Bulk.List(ctx, BulkListParams{}); return err },
		func() error { _, err := c.Onboarding.Capabilities(ctx); return err },
	} {
		if err := call(); err != nil {
			t.Fatal(err)
		}
	}
	for _, err := range c.Google.Pages(ctx, GoogleSearchInput{Query: "offline"}, 1) {
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestFormatGuideAndRunnableExample(t *testing.T) {
	if testing.Short() {
		t.Skip("compiled example and documentation validation")
	}
	fixtures := readFormatFixtures(t, "realtime-response-formats.json")
	var active formatCase
	activeMode := "realtime"
	var activeMu sync.RWMutex
	_, server := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		activeMu.RLock()
		fixture := active
		mode := activeMode
		activeMu.RUnlock()
		if r.Method == "GET" {
			if strings.HasSuffix(r.URL.Path, "/results") {
				serveFormat(t, w, r, fixture)
			} else if mode == "status" {
				serveFormat(t, w, r, fixture)
			} else {
				respond(w, map[string]any{"job": map[string]any{"id": "job_fixture0001", "status": "completed"}})
			}
			return
		}
		if r.URL.Path == "/v1/queries" {
			if mode == "submit" {
				serveFormat(t, w, r, fixture)
			} else {
				respond(w, map[string]any{"id": "job_fixture0001", "source": "google_search", "status": "pending"})
			}
			return
		}
		serveFormat(t, w, r, fixture)
	})
	goExe := filepath.Join(runtime.GOROOT(), "bin", "go")
	if runtime.GOOS == "windows" {
		goExe += ".exe"
	}
	binary := filepath.Join(t.TempDir(), "formats")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	build := exec.Command(goExe, "build", "-o", binary, "./examples/response_formats")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	env := []string{}
	for _, item := range os.Environ() {
		if !strings.HasPrefix(strings.ToUpper(item), "FLASHDATA_") {
			env = append(env, item)
		}
	}
	env = append(env, "FLASHDATA_API_KEY="+testKey, "FLASHDATA_DATA_ORIGIN="+server.URL, "FLASHDATA_MANAGEMENT_ORIGIN="+server.URL, "GOWORK=off")
	for _, v := range fixtures[SourceGoogleSearch] {
		activeMu.Lock()
		active = v
		activeMu.Unlock()
		cmd := exec.Command(binary, "-accept", string(v.Accept), "-respond-with", string(v.RespondWith))
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("format example failed: %v\n%s", err, out)
		}
		if v.Accept == AcceptText {
			var expected string
			_ = json.Unmarshal(v.Response, &expected)
			if string(out) != expected {
				t.Fatal("example changed text")
			}
		} else {
			if !json.Valid(out) {
				t.Fatal("example emitted invalid JSON")
			}
		}
	}
	// Execute the documented submission/status flags against the API examples.
	publicBytes, err := os.ReadFile("testdata/response-formats.json")
	if err != nil {
		t.Fatal(err)
	}
	var publicExamples []struct {
		Group    string
		Request  json.RawMessage
		Headers  map[string]string
		Response json.RawMessage
	}
	if err = json.Unmarshal(publicBytes, &publicExamples); err != nil {
		t.Fatal(err)
	}
	for _, example := range publicExamples {
		operation := map[string]string{"async": "submit", "batch": "batch", "status": "status"}[example.Group]
		if operation == "" {
			continue
		}
		activeMu.Lock()
		activeMode = operation
		active = formatCase{Accept: Accept(example.Headers["Accept"]), Response: example.Response}
		activeMu.Unlock()
		args := []string{"-operation", operation, "-job", "job_example0001", "-accept", example.Headers["Accept"]}
		if operation == "submit" {
			args = append(args, "-input", string(example.Request))
		}
		if operation == "batch" {
			var body struct{ Queries json.RawMessage }
			_ = json.Unmarshal(example.Request, &body)
			args = append(args, "-input", string(body.Queries))
		}
		cmd := exec.Command(binary, args...)
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s example failed: %v\n%s", operation, err, out)
		}
	}
	for _, v := range readFormatFixtures(t, "stored-result-formats.json")[SourceGoogleSearch] {
		activeMu.Lock()
		activeMode = "result"
		active = v
		activeMu.Unlock()
		cmd := exec.Command(binary, "-operation", "result", "-job", "job_fixture0001", "-accept", string(v.Accept), "-respond-with", string(v.RespondWith))
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("result example failed: %v\n%s", err, out)
		}
	}
	guide, err := os.ReadFile("docs/response-formats.md")
	if err != nil {
		t.Fatal(err)
	}
	blocks := regexp.MustCompile("(?s)```go\\r?\\n(.*?)```").FindAllStringSubmatch(string(guide), -1)
	if len(blocks) != 3 {
		t.Fatal("missing response-format recipes")
	}
	for i, block := range blocks {
		activeMu.Lock()
		activeMode = "guide"
		active = fixtures[SourceGoogleSearch][1]
		if i == 1 {
			active = fixtures[SourceYouTubeTranscript][3]
		}
		if i == 2 {
			active = readFormatFixtures(t, "stored-result-formats.json")[SourceGoogleSearch][3]
		}
		activeMu.Unlock()
		program := `package main
import ("context";"fmt";"os";flashdata "github.com/flashdatadev/flashdata-go")
func recipe() error {
client,err:=flashdata.New(flashdata.Options{APIKey:os.Getenv("FLASHDATA_API_KEY"),DataOrigin:os.Getenv("FLASHDATA_DATA_ORIGIN"),ManagementOrigin:os.Getenv("FLASHDATA_MANAGEMENT_ORIGIN")});if err!=nil{return err};ctx:=context.Background()
` + block[1] + `
return nil
}
func main(){if err:=recipe();err!=nil{fmt.Fprintln(os.Stderr,err);os.Exit(1)}}`
		file := filepath.Join(t.TempDir(), "main.go")
		if err := os.WriteFile(file, []byte(program), 0600); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(goExe, "run", file)
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("guide recipe %d failed: %v\n%s", i, err, out)
		}
	}
}
