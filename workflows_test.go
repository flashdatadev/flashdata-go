package flashdata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestAllTypedHelpers(t *testing.T) {
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		var q Query
		_ = json.NewDecoder(r.Body).Decode(&q)
		if strings.HasSuffix(r.URL.Path, "realtime") {
			respond(w, map[string]any{"source": q.Source, "status": "done", "results": []any{}})
		} else {
			respond(w, map[string]any{"id": "job_typed", "source": q.Source, "status": "pending"})
		}
	})
	ctx := context.Background()
	tests := []struct {
		name string
		call func() error
	}{
		{"Google.Search", func() error { _, e := c.Google.Search(ctx, GoogleSearchInput{Query: "test"}); return e }},
		{"Google.Images", func() error { _, e := c.Google.Images(ctx, GoogleImagesInput{Query: "test"}); return e }},
		{"Google.News", func() error { _, e := c.Google.News(ctx, GoogleNewsInput{Query: "test"}); return e }},
		{"Google.Shopping", func() error { _, e := c.Google.Shopping(ctx, GoogleShoppingInput{Query: "test"}); return e }},
		{"Google.Videos", func() error { _, e := c.Google.Videos(ctx, GoogleVideosInput{Query: "test"}); return e }},
		{"Google.Places", func() error { _, e := c.Google.Places(ctx, GooglePlacesInput{Query: "test"}); return e }},
		{"YouTube.Search", func() error { _, e := c.YouTube.Search(ctx, YouTubeSearchInput{Query: "test"}); return e }},
		{"YouTube.SearchExtended", func() error {
			_, e := c.YouTube.SearchExtended(ctx, YouTubeSearchExtendedInput{Query: "test"})
			return e
		}},
		{"YouTube.Metadata", func() error { _, e := c.YouTube.Metadata(ctx, YouTubeMetadataInput{VideoID: "dQw4w9WgXcQ"}); return e }},
		{"YouTube.Transcript", func() error {
			_, e := c.YouTube.Transcript(ctx, YouTubeTranscriptInput{VideoID: "dQw4w9WgXcQ"})
			return e
		}},
		{"YouTube.Captions", func() error { _, e := c.YouTube.Captions(ctx, YouTubeCaptionsInput{VideoID: "dQw4w9WgXcQ"}); return e }},
		{"YouTube.LicenseCheck", func() error {
			_, e := c.YouTube.LicenseCheck(ctx, YouTubeTrainabilityInput{VideoID: "dQw4w9WgXcQ"})
			return e
		}},
		{"YouTube.Channel", func() error {
			_, e := c.YouTube.Channel(ctx, YouTubeChannelInput{Handle: "@GoogleDevelopers"})
			return e
		}},
		{"YouTube.Suggestions", func() error { _, e := c.YouTube.Suggestions(ctx, YouTubeSuggestionsInput{Query: "test"}); return e }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.call(); err != nil {
				t.Fatal(err)
			}
		})
	}
	if _, err := c.Queries.Batch(ctx, []Query{(GoogleSearchInput{Query: "test"}).Request()}); !IsSubmissionUnknown(err) {
		t.Fatal("bad batch response accepted")
	}
}

func TestPollingTransitionsAndBudgets(t *testing.T) {
	for _, status := range []string{"completed", "failed", "paused", "cancelled", "unexpected"} {
		t.Run(status, func(t *testing.T) {
			c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				respond(w, map[string]any{"job": map[string]any{"id": "bulk_abcdefgh", "status": status}})
			})
			_, err := c.Bulk.Wait(context.Background(), "bulk_abcdefgh", PollOptions{Timeout: time.Second, Interval: time.Millisecond})
			switch status {
			case "completed":
				if err != nil {
					t.Fatal(err)
				}
			case "failed", "cancelled":
				var e *JobFailedError
				if !errors.As(err, &e) {
					t.Fatal(err)
				}
			case "paused":
				var e *JobPausedError
				if !errors.As(err, &e) {
					t.Fatal(err)
				}
			default:
				var e *ProtocolError
				if !errors.As(err, &e) {
					t.Fatal(err)
				}
			}
		})
	}
	t.Run("short client timeout survives longer polling budget", func(t *testing.T) {
		c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }, func(o *Options) { o.Timeout = 15 * time.Millisecond })
		_, err := c.Jobs.Wait(context.Background(), "job_wait", PollOptions{Timeout: time.Second, Interval: time.Millisecond})
		var poll *PollingTimeoutError
		if errors.As(err, &poll) || !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("wrong timeout classification: %v", err)
		}
	})
	t.Run("total deadline includes status read", func(t *testing.T) {
		c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() })
		_, err := c.Jobs.Wait(context.Background(), "job_wait", PollOptions{Timeout: 15 * time.Millisecond, Interval: time.Millisecond})
		var poll *PollingTimeoutError
		if !errors.As(err, &poll) || poll.JobID != "job_wait" {
			t.Fatal(err)
		}
	})
	t.Run("cancel during sleep retains accepted ID", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
			respond(w, map[string]any{"job": map[string]any{"id": "job_wait", "status": "pending"}})
			cancel()
		})
		_, err := c.Jobs.Wait(ctx, "job_wait", PollOptions{})
		var interrupted *PollingInterruptedError
		if !errors.As(err, &interrupted) || !errors.Is(err, context.Canceled) || interrupted.JobID != "job_wait" {
			t.Fatal(err)
		}
	})
	var calls atomic.Int32
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		status := "processing"
		if calls.Add(1) > 1 {
			status = "completed"
		}
		respond(w, map[string]any{"job": map[string]any{"id": "job_wait", "status": status}})
	})
	if _, err := c.Jobs.Wait(context.Background(), "job_wait", PollOptions{Interval: time.Millisecond}); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Fatal("wrong polling transitions")
	}
}

func TestSignedResultOriginAndCredentialIsolation(t *testing.T) {
	var storageCalls atomic.Int32
	storage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		storageCalls.Add(1)
		if r.Header.Get("X-API-Key") != "" || r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
			t.Error("credential forwarded to storage")
		}
		respond(w, map[string]any{"future": 9007199254740993})
	}))
	defer storage.Close()
	var advertised string = storage.URL
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != testKey {
			t.Error("management request missing key")
		}
		if r.URL.Path == "/v2/api-access" {
			respond(w, map[string]any{"sources": []string{"google_search"}, "result_origin": advertised})
			return
		}
		w.Header().Set("Location", storage.URL+"/result?signature=secret")
		w.WriteHeader(302)
	})
	if _, err := c.Jobs.Result(context.Background(), "job_signed"); err != nil {
		t.Fatal(err)
	}
	if storageCalls.Load() != 1 {
		t.Fatal("storage was not read once")
	}
	bad, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/api-access" {
			respond(w, map[string]any{"result_origin": "https://trusted.example"})
			return
		}
		w.Header().Set("Location", storage.URL+"/result")
		w.WriteHeader(302)
	})
	if _, err := bad.Jobs.Result(context.Background(), "job_signed"); err == nil {
		t.Fatal("untrusted result origin accepted")
	}
	if storageCalls.Load() != 1 {
		t.Fatal("untrusted origin contacted")
	}
	redirect, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", storage.URL)
		w.WriteHeader(307)
	})
	if _, err := redirect.Account.Balance(context.Background()); err == nil {
		t.Fatal("management redirect followed")
	}
}

func TestBulkCompleteWorkflowAndStreaming(t *testing.T) {
	const id = "bulk_abcdefgh"
	var base string
	var uploads atomic.Int32
	var starts atomic.Int32
	gz := gzipBytes(t, "\r\n{\"index\":0,\"status\":\"completed\",\"future\":9007199254740993123}\n{\"index\":1,\"status\":\"invalid\"}")
	c, s := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/api-access":
			respond(w, map[string]any{"result_origin": base})
		case "/v2/bulk-jobs":
			if r.Method == "POST" {
				respond(w, map[string]any{"job": map[string]any{"id": id, "status": "awaiting_upload"}, "upload": map[string]any{"href": base + "/upload?signature=secret", "method": "PUT", "contentType": "text/plain; charset=utf-8", "expiresAt": time.Now().Add(time.Minute).Format(time.RFC3339)}})
			} else {
				respond(w, map[string]any{"data": []any{map[string]any{"id": id}}, "pagination": map[string]any{"hasMore": false}})
			}
		case "/upload":
			if r.Header.Get("X-API-Key") != "" {
				t.Error("key leaked to upload")
			}
			b, _ := io.ReadAll(r.Body)
			if string(b) != "dQw4w9WgXcQ\n" {
				t.Error("wrong input")
			}
			uploads.Add(1)
			w.WriteHeader(200)
		case "/v2/bulk-jobs/" + id + "/start":
			starts.Add(1)
			respond(w, map[string]any{"job": map[string]any{"id": id, "status": "running"}})
		case "/v2/bulk-jobs/" + id:
			respond(w, map[string]any{"job": map[string]any{"id": id, "status": "completed"}})
		case "/v2/bulk-jobs/" + id + "/results":
			respond(w, map[string]any{"id": id, "parts": []any{map[string]any{"number": 0}}})
		case "/v2/bulk-jobs/" + id + "/results/0":
			w.Header().Set("Content-Type", "application/gzip")
			_, _ = w.Write(gz)
		default:
			t.Error("unexpected route", r.URL.Path)
			w.WriteHeader(404)
		}
	})
	base = s.URL
	ctx := context.Background()
	created, err := c.Bulk.Create(ctx, CreateBulkJobRequest{Source: SourceYouTubeMetadata})
	if err != nil {
		t.Fatal(err)
	}
	if err = c.Bulk.Upload(ctx, created, strings.NewReader("dQw4w9WgXcQ\n"), 12); err != nil {
		t.Fatal(err)
	}
	if _, err = c.Bulk.Start(ctx, id); err != nil {
		t.Fatal(err)
	}
	if _, err = c.Bulk.Wait(ctx, id, PollOptions{}); err != nil {
		t.Fatal(err)
	}
	count := 0
	for row, err := range c.Bulk.Rows(ctx, id) {
		if err != nil {
			t.Fatal(err)
		}
		if count == 0 && string(row["future"]) != "9007199254740993123" {
			t.Fatal("rounded result")
		}
		count++
	}
	if count != 2 || starts.Load() != 1 || uploads.Load() != 1 {
		t.Fatal("bulk workflow incomplete")
	}
	if b, err := c.Bulk.Part(ctx, id, 0); err != nil || len(b) != len(gz) {
		t.Fatal("part bytes mismatch", err)
	}
	count = 0
	for _, err := range c.Bulk.IterJobs(ctx, BulkListParams{}, 2) {
		if err != nil {
			t.Fatal(err)
		}
		count++
	}
	if count != 1 {
		t.Fatal("listing missing")
	}
}

func TestBulkMalformedPartsAndBounds(t *testing.T) {
	for _, tc := range []struct {
		name, plain string
		max         int64
		line        int
		truncate    bool
	}{
		{"non-object", "[]\n", 1000, 100, false}, {"null", "null\n", 1000, 100, false}, {"invalid-json", "{\n", 1000, 100, false},
		{"invalid-utf8", "{\"x\":\"\xff\"}\n", 1000, 100, false}, {"huge-row", "{\"x\":\"" + strings.Repeat("x", 200) + "\"}\n", 1000, 40, false},
		{"gzip-bomb", strings.Repeat(" \n", 300), 100, 100, false}, {"truncated", "{\"x\":1}\n", 1000, 100, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gz := gzipBytes(t, tc.plain)
			if tc.truncate {
				gz = gz[:len(gz)-4]
			}
			c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(gz) }, func(o *Options) { o.MaxResponseBytes = tc.max; o.MaxLineBytes = tc.line })
			var got error
			for _, err := range c.Bulk.IterRows(context.Background(), "bulk_abcdefgh", 0) {
				if err != nil {
					got = err
				}
			}
			if got == nil {
				t.Fatal("malformed/oversized part accepted")
			}
		})
	}
}

func TestBulkValidationAndRecoveryRequests(t *testing.T) {
	var calls atomic.Int32
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		respond(w, map[string]any{"job": map[string]any{"id": "bulk_abcdefgh", "status": "running"}})
	})
	for _, req := range []CreateBulkJobRequest{{Source: SourceGoogleSearch}, {Source: SourceYouTubeDownload}, {Source: SourceYouTubeMetadata, Options: []any{}}, {Source: SourceYouTubeMetadata, Options: map[string]any{"unknown": true}}, {Source: SourceYouTubeDownload, Options: map[string]any{"result_storage": map[string]any{}}}} {
		if _, err := c.Bulk.Create(context.Background(), req); err == nil {
			t.Fatal("invalid bulk creation accepted")
		}
	}
	if calls.Load() != 0 {
		t.Fatal("invalid creation dispatched")
	}
	for _, call := range []func() error{
		func() error {
			_, e := c.Bulk.Update(context.Background(), "bulk_abcdefgh", UpdateBulkJobRequest{RemoveCallback: true})
			return e
		},
		func() error { _, e := c.Bulk.Pause(context.Background(), "bulk_abcdefgh"); return e },
		func() error { _, e := c.Bulk.Resume(context.Background(), "bulk_abcdefgh"); return e },
		func() error { _, e := c.Bulk.Cancel(context.Background(), "bulk_abcdefgh"); return e },
	} {
		if err := call(); err != nil {
			t.Fatal(err)
		}
	}
	if calls.Load() != 4 {
		t.Fatal("missing recovery operation")
	}
}

func TestPaginationBoundsAndCursorCycles(t *testing.T) {
	var calls atomic.Int32
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if strings.HasPrefix(r.URL.Path, "/v1/") {
			respond(w, map[string]any{"source": "google_search", "status": "done", "results": []any{}})
		} else {
			respond(w, map[string]any{"data": []any{}, "pagination": map[string]any{"hasMore": true, "nextCursor": "same"}})
		}
	})
	for _, err := range c.Google.Pages(context.Background(), GoogleSearchInput{Query: "test"}, 3) {
		if err != nil {
			t.Fatal(err)
		}
		break
	}
	if calls.Load() != 1 {
		t.Fatal("early break dispatched more pages")
	}
	var got error
	for _, err := range c.Bulk.IterJobs(context.Background(), BulkListParams{}, 5) {
		if err != nil {
			got = err
		}
	}
	if got == nil {
		t.Fatal("cursor cycle accepted")
	}
	for _, err := range c.Google.Pages(context.Background(), GoogleSearchInput{Query: "test"}, 0) {
		if err == nil {
			t.Fatal("unbounded pages accepted")
		}
	}
}

func TestOnboardingMachineCredentialSeparation(t *testing.T) {
	proof, _ := NewRegistrationProof()
	if len(proof) != 43 {
		t.Fatal("proof lacks 256-bit encoding")
	}
	const id = "acn_abcdefgh"
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "" || r.Header.Get("Cookie") != "" {
			t.Error("ambiguous machine credentials")
		}
		if strings.HasSuffix(r.URL.Path, "capabilities") {
			if r.Header.Get("Authorization") != "" {
				t.Error("public capabilities authenticated")
			}
			respond(w, map[string]any{"protocolVersion": 2, "available": true})
			return
		}
		if r.Header.Get("Authorization") == "" {
			t.Error("missing purpose-specific bearer")
		}
		connection := map[string]any{"id": id, "ownership": "owned", "lifecycle": "valid", "access": "ready", "installed": false, "generation": 2}
		switch r.URL.Path {
		case "/v2/agent-connections/registrations":
			if r.Header.Get("Authorization") != "Bearer "+proof {
				t.Error("wrong registration proof")
			}
			connection["ownership"], connection["generation"] = "unowned", 1
			respond(w, map[string]any{"connection": connection, "credentials": map[string]any{"connectionId": id, "apiKey": "fd_temporary", "setupCredential": "setup", "handoffProof": "handoff", "generation": 1}, "dataOrigin": "https://data.flashdata.dev"})
		case "/v2/agent-connections/" + id, "/v2/agent-connections/" + id + "/identity":
			respond(w, map[string]any{"connection": connection})
		case "/v2/agent-connections/" + id + "/claims":
			respond(w, map[string]any{"claimId": "acl_abcdefgh", "claimPath": "/connect/claim/acl_abcdefgh", "confirmationCode": "12345678", "expiresAt": "2026-10-22T00:00:00Z"})
		case "/v2/agent-connections/" + id + "/onboarding":
			respond(w, map[string]any{"status": "handoff_ready", "connection": connection})
		case "/v2/agent-connections/" + id + "/handoff":
			respond(w, map[string]any{"status": "delivered", "connection": connection, "credentials": map[string]any{"connectionId": id, "apiKey": "fd_owner", "generation": 2}})
		case "/v2/agent-connections/" + id + "/handoff/ack":
			var b map[string]string
			_ = json.NewDecoder(r.Body).Decode(&b)
			if b["apiKey"] != "fd_owner" {
				t.Error("wrong candidate")
			}
			respond(w, map[string]any{"status": "confirmed", "generation": 2})
		default:
			t.Error(r.URL.Path)
			w.WriteHeader(404)
		}
	})
	ctx := context.Background()
	if _, err := c.Onboarding.Capabilities(ctx); err != nil {
		t.Fatal(err)
	}
	reg, err := c.Onboarding.Register(ctx, proof, RegistrationRequest{Name: "Go client", Scopes: []Source{SourceGoogleSearch}, Transport: "cli"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(fmt.Sprintf("%#v", reg), "fd_temporary") {
		t.Fatal("credentials leaked in formatting")
	}
	if _, err = c.Onboarding.TemporaryStatus(ctx, id, "setup"); err != nil {
		t.Fatal(err)
	}
	claim, err := c.Onboarding.CreateClaim(ctx, id, "setup")
	if err != nil {
		t.Fatal(err)
	}
	link, err := ClaimURL("https://flashdata.dev", claim)
	if err != nil || strings.Contains(link, claim.ConfirmationCode) {
		t.Fatal("unsafe claim URL", err)
	}
	if _, err = c.Onboarding.Status(ctx, id, "handoff"); err != nil {
		t.Fatal(err)
	}
	handoff, err := c.Onboarding.Handoff(ctx, id, "handoff")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.Onboarding.Identity(ctx, id, handoff.Credentials.APIKey); err != nil {
		t.Fatal(err)
	}
	if _, err = c.Onboarding.Acknowledge(ctx, id, "handoff", handoff.Credentials.APIKey); err != nil {
		t.Fatal(err)
	}
}

func TestIdentityFailsClosed(t *testing.T) {
	for _, value := range []any{map[string]any{}, map[string]any{"apiKeyId": "key_abc", "accountId": "ata_abc", "ownership": "unowned", "accountType": "temporary", "access": "ready", "installed": true, "recurringAllowed": true, "verifiedAt": "now"}} {
		c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) { respond(w, value) })
		if _, err := c.Account.Identity(context.Background()); err == nil {
			t.Fatal("untrusted ownership accepted")
		}
	}
}

func TestConcurrentSignedResultsDiscoverOriginOnce(t *testing.T) {
	var discoveries atomic.Int32
	var base string
	c, s := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/api-access":
			discoveries.Add(1)
			respond(w, map[string]any{"result_origin": base})
		case "/signed":
			if r.Header.Get("X-API-Key") != "" {
				t.Error("concurrent signed request leaked key")
			}
			respond(w, map[string]any{"ok": true})
		default:
			w.Header().Set("Location", base+"/signed")
			w.WriteHeader(302)
		}
	})
	base = s.URL
	var wg sync.WaitGroup
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := c.Jobs.Result(context.Background(), "job_concurrent"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if discoveries.Load() != 1 {
		t.Fatal("result origin was not safely cached", discoveries.Load())
	}
}

func TestWaitingForOriginDiscoveryHonorsCancellation(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		respond(w, map[string]any{"result_origin": "https://storage.example"})
	})
	done := make(chan struct{})
	go func() { defer close(done); _, _ = c.trustedResultOrigin(context.Background()) }()
	<-started
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Millisecond)
	defer cancel()
	finished := make(chan error, 1)
	go func() { _, err := c.trustedResultOrigin(ctx); finished <- err }()
	select {
	case err := <-finished:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Error(err)
		}
	case <-time.After(time.Second):
		t.Error("origin wait ignored cancellation")
	}
	close(release)
	<-done
}
