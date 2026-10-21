package flashdata

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestTypedAsyncHelpers(t *testing.T) {
	var fixtures map[string]struct{ Request json.RawMessage }
	b, err := os.ReadFile("testdata/source-examples.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(b, &fixtures); err != nil {
		t.Fatal(err)
	}
	var expectedSource string
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		var q Query
		_ = json.NewDecoder(r.Body).Decode(&q)
		if r.URL.Path != "/v1/queries" || string(q.Source) != expectedSource || q.CallbackURL != "https://example.com/notify" {
			t.Error("incorrect typed async request", q.Source)
		}
		respond(w, map[string]any{"id": "job_typed", "source": q.Source, "status": "pending"})
	})
	methods := map[string]string{"google_search": "SearchAsync", "google_images": "ImagesAsync", "google_news": "NewsAsync", "google_shopping": "ShoppingAsync", "google_videos": "VideosAsync", "google_places": "PlacesAsync", "youtube_search": "SearchAsync", "youtube_search_extended": "SearchExtendedAsync", "youtube_metadata": "MetadataAsync", "youtube_transcript": "TranscriptAsync", "youtube_captions": "CaptionsAsync", "youtube_trainability": "LicenseCheckAsync", "youtube_channel": "ChannelAsync", "youtube_suggestions": "SuggestionsAsync", "youtube_download": "Download"}
	for source, method := range methods {
		t.Run(source, func(t *testing.T) {
			expectedSource = source
			receiver := reflect.ValueOf(c.YouTube)
			if strings.HasPrefix(source, "google_") {
				receiver = reflect.ValueOf(c.Google)
			}
			m := receiver.MethodByName(method)
			if !m.IsValid() {
				t.Fatal("missing typed helper", method)
			}
			input := reflect.New(m.Type().In(1))
			if err = json.Unmarshal(fixtures[source].Request, input.Interface()); err != nil {
				t.Fatal(err)
			}
			values := m.Call([]reflect.Value{reflect.ValueOf(context.Background()), input.Elem(), reflect.ValueOf("https://example.com/notify")})
			if !values[1].IsNil() {
				t.Fatal(values[1].Interface())
			}
			if values[0].Interface().(*AsyncSubmission).ID != "job_typed" {
				t.Fatal("missing job ID")
			}
		})
	}
}

func TestAccountAndPublicEndpointCoverage(t *testing.T) {
	var seen = map[string]bool{}
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		seen[r.URL.Path] = true
		if r.URL.Path == "/v2/api-access/pricing" {
			if r.URL.Query().Get("source") != "google_search" || r.URL.Query().Get("request_count") != "3" {
				t.Error("pricing query lost")
			}
		}
		if r.URL.Path == "/v2/usage" {
			if r.URL.Query().Get("from") != "2026-10-01T00:00:00Z" || r.URL.Query().Get("to") != "2026-10-02T00:00:00Z" {
				t.Error("usage range lost")
			}
		}
		if r.URL.Path == "/v2/api-access/identity" {
			respond(w, APIKeyIdentity{APIKeyID: "key_example", AccountID: "acct_example", AccountType: "owned", Ownership: "owned", Access: "ready", Installed: true, RecurringAllowed: true, Scopes: []Source{SourceGoogleSearch}, VerifiedAt: "2026-10-21T00:00:00Z"})
			return
		}
		respond(w, map[string]any{})
	})
	ctx := context.Background()
	for _, call := range []func() error{
		func() error { _, e := c.Account.Access(ctx); return e }, func() error { _, e := c.Account.Balance(ctx); return e }, func() error { _, e := c.Account.Limits(ctx); return e },
		func() error {
			_, e := c.Account.Pricing(ctx, PricingParams{Source: SourceGoogleSearch, RequestCount: 3})
			return e
		},
		func() error {
			_, e := c.Account.Usage(ctx, UsageParams{Source: SourceYouTubeMetadata, From: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)})
			return e
		},
		func() error { _, e := c.Account.Identity(ctx); return e }, func() error { _, e := c.Public.Health(ctx); return e }, func() error { _, e := c.Public.Status(ctx); return e }, func() error { _, e := c.Public.Plans(ctx); return e },
	} {
		if err := call(); err != nil {
			t.Fatal(err)
		}
	}
	if len(seen) != 9 {
		t.Fatal("missing account/public operation")
	}
	for _, p := range []PricingParams{{Source: "unsupported"}, {RequestCount: -1}, {RequestCount: 100001}} {
		if _, err := c.Account.Pricing(ctx, p); err == nil {
			t.Fatal("invalid pricing accepted")
		}
	}
}

func TestOnboardingRejectsInvalidInputsBeforeNetwork(t *testing.T) {
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("invalid onboarding reached network")
		w.WriteHeader(500)
	})
	proof, _ := NewRegistrationProof()
	valid := RegistrationRequest{Name: "Example", Scopes: []Source{SourceGoogleSearch}}
	for _, input := range []RegistrationRequest{{Name: "", Scopes: valid.Scopes}, {Name: "bad\x00name", Scopes: valid.Scopes}, {Name: "Example", Scopes: []Source{}}, {Name: "Example", Scopes: []Source{SourceGoogleSearch, SourceGoogleSearch}}, {Name: "Example", Scopes: []Source{SourceYouTubeDownload}}, {Name: "Example", Scopes: valid.Scopes, Audience: "admin"}, {Name: "Example", Scopes: valid.Scopes, Transport: "shell"}, {Name: "Example", Scopes: valid.Scopes, Client: "Bad Client"}} {
		if _, err := c.Onboarding.Register(context.Background(), proof, input); err == nil {
			t.Fatal("invalid registration accepted")
		}
	}
	if _, err := c.Onboarding.Register(context.Background(), "short", valid); err == nil {
		t.Fatal("short registration proof")
	}
	if _, err := c.Onboarding.Status(context.Background(), "../path", "proof"); err == nil {
		t.Fatal("unsafe connection ID")
	}
	if _, err := c.Onboarding.Identity(context.Background(), "acn_abcdefgh", "key\nheader"); err == nil {
		t.Fatal("invalid bearer")
	}
	if _, err := ClaimURL("https://flashdata.dev", &AgentClaim{ClaimID: "acl_abcdefgh", ClaimPath: "https://evil.example/path"}); err == nil {
		t.Fatal("untrusted browser URL")
	}
	if _, err := c.Onboarding.Acknowledge(context.Background(), "acn_abcdefgh", "proof", ""); err == nil {
		t.Fatal("missing candidate key accepted")
	}
}

func TestUploadRejectsUntrustedOrigin(t *testing.T) {
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/api-access" {
			t.Error("untrusted upload reached transport")
		}
		respond(w, map[string]any{"result_origin": "https://trusted.example"})
	})
	created := &BulkJobCreated{Job: BulkJob{ID: "bulk_abcdefgh"}, Upload: BulkJobCreatedUpload{Href: "https://untrusted.example/input", Method: "PUT", ContentType: "text/plain; charset=utf-8", ExpiresAt: time.Now().Add(time.Minute).Format(time.RFC3339)}}
	if err := c.Bulk.Upload(context.Background(), created, strings.NewReader("hello"), 5); err == nil {
		t.Fatal("untrusted upload accepted")
	}
}
