package flashdata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestBearerCredentialCannotSelectAPIKeyMode(t *testing.T) {
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer key" || r.Header.Get("X-API-Key") != "" {
			t.Error("proof value changed authentication mode")
		}
		respond(w, OnboardingStatus{Status: "awaiting_claim"})
	})
	if _, err := c.Onboarding.Status(context.Background(), "acn_abcdefgh", "key"); err != nil {
		t.Fatal(err)
	}
}

func TestBulkReadPreservesCancellationAndRequestDeadline(t *testing.T) {
	for _, operation := range []string{"compressed", "gzip-header", "gzip-body"} {
		for _, deadline := range []bool{false, true} {
			name := operation + "/cancel"
			if deadline {
				name = operation + "/deadline"
			}
			t.Run(name, func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(200)
					if operation == "gzip-body" {
						// A complete gzip header, but no deflate data yet.
						_, _ = w.Write([]byte{0x1f, 0x8b, 8, 0, 0, 0, 0, 0, 0, 255})
					}
					w.(http.Flusher).Flush()
					if !deadline {
						cancel()
					}
					<-r.Context().Done()
				}, func(o *Options) { o.Timeout = 100 * time.Millisecond })
				var err error
				if operation == "compressed" {
					_, err = c.Bulk.Part(ctx, "bulk_abcdefgh", 0)
				} else {
					for _, e := range c.Bulk.IterRows(ctx, "bulk_abcdefgh", 0) {
						err = e
					}
				}
				want := context.Canceled
				if deadline {
					want = context.DeadlineExceeded
				}
				if !errors.Is(err, want) || IsSubmissionUnknown(err) {
					t.Fatalf("expected %v, got %v", want, err)
				}
			})
		}
	}
}

func TestSignedStorageErrorsRedactBareCredentials(t *testing.T) {
	const signature = "private/signature+value"
	storage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "" || r.Header.Get("Authorization") != "" {
			t.Error("storage request contained a credential header")
		}
		w.Header().Set("X-Request-ID", signature)
		w.Header().Set("Retry-After", url.QueryEscape(signature))
		w.WriteHeader(403)
		respond(w, map[string]any{"code": "denied", "detail": signature + " " + url.QueryEscape(signature)})
	}))
	defer storage.Close()
	target := storage.URL + "/object?X-Amz-Signature=" + url.QueryEscape(signature)
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", target)
		w.WriteHeader(302)
	}, func(o *Options) { o.ResultOrigin = storage.URL })
	check := func(err error) {
		t.Helper()
		var api *APIError
		if !errors.As(err, &api) || api.StatusCode != 403 {
			t.Fatalf("expected storage rejection, got %v", err)
		}
		text := api.Error() + api.RequestID + api.RetryAfter
		if strings.Contains(text, signature) || strings.Contains(text, url.QueryEscape(signature)) {
			t.Fatal("signed credential survived redaction")
		}
	}
	_, err := c.Jobs.Result(context.Background(), "job_example")
	check(err)
	created := &BulkJobCreated{Job: BulkJob{ID: "bulk_abcdefgh"}, Upload: BulkJobCreatedUpload{Href: target, Method: "PUT", ContentType: "text/plain; charset=utf-8", ExpiresAt: time.Now().Add(time.Minute).Format(time.RFC3339)}}
	check(c.Bulk.Upload(context.Background(), created, strings.NewReader("x"), 1))
}

func TestResultManifestMustMatchRequestedJob(t *testing.T) {
	var requests atomic.Int32
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		respond(w, BulkManifest{ID: "bulk_different", Parts: []BulkManifestPart{{Number: 0}}})
	})
	for _, err := range c.Bulk.Rows(context.Background(), "bulk_abcdefgh") {
		var protocol *ProtocolError
		if !errors.As(err, &protocol) {
			t.Fatalf("unexpected manifest error: %v", err)
		}
	}
	if requests.Load() != 1 {
		t.Fatal("read a part from an unrelated manifest")
	}
}

func TestOnboardingRejectsInconsistentAuthority(t *testing.T) {
	const id = "acn_abcdefgh"
	tests := []struct {
		name     string
		response any
		call     func(*Client) error
		mutation bool
	}{
		{"identity-account-type", APIKeyIdentity{APIKeyID: "key_example", AccountID: "acct_example", AccountType: "temporary", Ownership: "owned", Access: "ready", Scopes: []Source{SourceGoogleSearch}, VerifiedAt: "2026-10-21T00:00:00Z"}, func(c *Client) error { _, e := c.Account.Identity(context.Background()); return e }, false},
		{"identity-timestamp", APIKeyIdentity{APIKeyID: "key_example", AccountID: "acct_example", AccountType: "owned", Ownership: "owned", Access: "ready", Scopes: []Source{SourceGoogleSearch}, VerifiedAt: "now"}, func(c *Client) error { _, e := c.Account.Identity(context.Background()); return e }, false},
		{"connection-id-only", map[string]any{"connection": map[string]any{"id": id}}, func(c *Client) error { _, e := c.Onboarding.Identity(context.Background(), id, "proof"); return e }, false},
		{"handoff-confirmed-without-generation", Handoff{Status: "confirmed"}, func(c *Client) error { _, e := c.Onboarding.Handoff(context.Background(), id, "proof"); return e }, true},
		{"handoff-generation-mismatch", Handoff{Status: "delivered", Connection: &AgentConnection{ID: id, Ownership: "owned", Lifecycle: "valid", Access: "ready", Generation: 3}, Credentials: &AgentCredentials{ConnectionID: id, APIKey: "fd_offline", Generation: 2}}, func(c *Client) error { _, e := c.Onboarding.Handoff(context.Background(), id, "proof"); return e }, true},
		{"claim-missing-expiry", AgentClaim{ClaimID: "acl_abcdefgh", ClaimPath: "/connect/claim/acl_abcdefgh", ConfirmationCode: "12345678"}, func(c *Client) error { _, e := c.Onboarding.CreateClaim(context.Background(), id, "proof"); return e }, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) { respond(w, test.response) })
			err := test.call(c)
			var protocol *ProtocolError
			if !errors.As(err, &protocol) || IsSubmissionUnknown(err) != test.mutation {
				t.Fatalf("malformed authority accepted or misclassified: %v", err)
			}
		})
	}
}

func TestQueryResponsesMustMatchSubmission(t *testing.T) {
	q := Query{Source: SourceGoogleSearch, Query: "offline"}
	for _, method := range []string{"realtime", "async", "batch"} {
		t.Run(method, func(t *testing.T) {
			var requests atomic.Int32
			c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				switch method {
				case "realtime":
					respond(w, map[string]any{"source": "youtube_metadata", "status": "done", "results": []any{}})
				case "async":
					respond(w, map[string]any{"id": "job_example", "source": "youtube_metadata", "status": "pending"})
				case "batch":
					respond(w, map[string]any{"queries": []any{map[string]any{"id": "job_example", "source": q.Source, "status": "pending"}, map[string]any{"id": "job_example", "source": q.Source, "status": "pending"}}})
				}
			})
			var err error
			switch method {
			case "realtime":
				_, err = c.Queries.Realtime(context.Background(), q)
			case "async":
				_, err = c.Queries.Submit(context.Background(), q)
			case "batch":
				_, err = c.Queries.Batch(context.Background(), []Query{q, q})
			}
			if !IsSubmissionUnknown(err) || requests.Load() != 1 {
				t.Fatalf("inconsistent submission accepted or retried: %v", err)
			}
			if method != "realtime" {
				var unknown *SubmissionUnknownError
				if !errors.As(err, &unknown) || len(unknown.JobIDs) != 1 || unknown.JobIDs[0] != "job_example" {
					t.Fatal("lost a valid recovery ID from an inconsistent response")
				}
			}
		})
	}
}

func TestSizeOptionsCannotOverflowBounds(t *testing.T) {
	for _, options := range []Options{{MaxResponseBytes: int64(^uint64(0) >> 1)}, {MaxLineBytes: int(^uint(0) >> 1)}} {
		if _, err := New(options); err == nil {
			t.Fatal("accepted an overflowing response bound")
		}
	}
}

func TestSecretModelsRedactFormattingButRetainWireCredentials(t *testing.T) {
	const secret = "private-offline-credential"
	var storage Storage
	if err := json.Unmarshal([]byte(`{"storage_type":"s3","storage_url":"bucket","access_key":"`+secret+`","secret_key":"`+secret+`"}`), &storage); err != nil {
		t.Fatal(err)
	}
	var account ServiceAccount
	if err := json.Unmarshal([]byte(`{"project_id":"test","client_email":"test@example.com","private_key":"`+secret+`"}`), &account); err != nil {
		t.Fatal(err)
	}
	created := BulkJobCreated{Upload: BulkJobCreatedUpload{Href: "https://storage.example/input?signature=" + secret}}
	for _, model := range []any{storage, account, created, created.Upload} {
		for _, format := range []string{"%v", "%+v", "%#v"} {
			if strings.Contains(fmt.Sprintf(format, model), secret) {
				t.Fatal("formatting leaked a storage credential")
			}
		}
		wire, err := json.Marshal(model)
		if err != nil || !strings.Contains(string(wire), secret) {
			t.Fatal("redaction changed request serialization", err)
		}
	}
}

func TestBulkCRLFDoesNotCountTowardRowLimit(t *testing.T) {
	const row = `{"ok":true}`
	for _, ending := range []string{"", "\n", "\r\n"} {
		t.Run("ending-"+strings.ReplaceAll(strings.ReplaceAll(ending, "\n", "LF"), "\r", "CR"), func(t *testing.T) {
			part := gzipBytes(t, row+ending)
			c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(part) }, func(o *Options) { o.MaxLineBytes = len(row) })
			count := 0
			for _, err := range c.Bulk.IterRows(context.Background(), "bulk_abcdefgh", 0) {
				if err != nil {
					t.Fatal(err)
				}
				count++
			}
			if count != 1 {
				t.Fatal("lost a row at the exact line limit")
			}
		})
	}
}
