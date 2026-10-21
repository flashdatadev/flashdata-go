package flashdata

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"testing"
)

func TestWireTypeErrorsRetainRecoveryIDs(t *testing.T) {
	q := Query{Source: SourceGoogleSearch, Query: "offline"}
	for _, kind := range []string{"async", "batch", "bulk"} {
		t.Run(kind, func(t *testing.T) {
			want := "job_recoverable"
			if kind == "bulk" {
				want = "bulk_recoverable"
			}
			calls := 0
			c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				switch kind {
				case "async":
					respond(w, map[string]any{"id": want, "source": 42, "status": "pending"})
				case "batch":
					respond(w, map[string]any{"queries": []any{map[string]any{"id": want, "source": 42, "status": "pending"}}})
				case "bulk":
					respond(w, map[string]any{"job": map[string]any{"id": want}, "upload": map[string]any{"href": 42}})
				}
			})
			var err error
			switch kind {
			case "async":
				_, err = c.Queries.Submit(context.Background(), q)
			case "batch":
				_, err = c.Queries.Batch(context.Background(), []Query{q})
			case "bulk":
				_, err = c.Bulk.Create(context.Background(), CreateBulkJobRequest{Source: SourceYouTubeMetadata})
			}
			var unknown *SubmissionUnknownError
			if !errors.As(err, &unknown) || !reflect.DeepEqual(unknown.JobIDs, []string{want}) || calls != 1 {
				t.Fatalf("lost recovery evidence or replayed a submission: %#v (%d calls)", unknown, calls)
			}
		})
	}
}

func TestConfirmedHandoffMetadataMustBeConsistent(t *testing.T) {
	const id = "acn_abcdefgh"
	for _, ack := range []bool{false, true} {
		name := "handoff"
		if ack {
			name = "ack"
		}
		t.Run(name, func(t *testing.T) {
			c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				respond(w, Handoff{Status: "confirmed", Generation: 2, Connection: &AgentConnection{ID: id, Ownership: "owned", Lifecycle: "valid", Access: "ready", Generation: 3}})
			})
			var err error
			if ack {
				_, err = c.Onboarding.Acknowledge(context.Background(), id, "proof", "fd_offline")
			} else {
				_, err = c.Onboarding.Handoff(context.Background(), id, "proof")
			}
			if !IsSubmissionUnknown(err) {
				t.Fatalf("inconsistent confirmation accepted: %v", err)
			}
		})
	}
}
