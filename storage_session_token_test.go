package flashdata

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestSessionTokenStorageWorkflows(t *testing.T) {
	for _, provider := range []string{"s3", "s3_compatible", "oss", "cos", "bos", "tos"} {
		t.Run(provider, func(t *testing.T) {
			token := "fixture-temporary-session-token"
			storage := Storage{StorageType: provider, StorageURL: "bucket", AccessKey: Ptr("temp-key"), SecretKey: Ptr("temp-secret"), SessionToken: &token}
			var bodies []map[string]any
			client, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				bodies = append(bodies, body)
				receipt := map[string]any{"id": "job_example123", "source": "youtube_download", "status": "pending"}
				switch {
				case strings.HasSuffix(r.URL.Path, "/batch"):
					respond(w, map[string]any{"queries": []any{receipt}})
				case strings.HasSuffix(r.URL.Path, "/bulk-jobs"):
					respond(w, map[string]any{"job": map[string]any{"id": "bulk_example123"}, "upload": map[string]any{"href": "https://storage.example/upload"}})
				case r.Method == "PATCH":
					respond(w, map[string]any{"job": map[string]any{"id": "bulk_example123", "status": "paused"}})
				default:
					respond(w, receipt)
				}
			})
			ctx := context.Background()
			input := YouTubeDownloadInput{VideoID: "dQw4w9WgXcQ", Params: YouTubeDownloadParams{Storage: storage}}
			if _, err := client.YouTube.Download(ctx, input, ""); err != nil {
				t.Fatal(err)
			}
			if _, err := client.Queries.Batch(ctx, []Query{input.Request()}); err != nil {
				t.Fatal(err)
			}
			if _, err := client.Bulk.Create(ctx, CreateBulkJobRequest{Source: SourceYouTubeDownload, Options: map[string]any{"storage": storage}}); err != nil {
				t.Fatal(err)
			}
			if _, err := client.Bulk.Create(ctx, CreateBulkJobRequest{Source: SourceYouTubeMetadata, Options: map[string]any{"result_storage": storage}}); err != nil {
				t.Fatal(err)
			}
			if _, err := client.Bulk.Update(ctx, "bulk_example123", UpdateBulkJobRequest{Options: map[string]any{"result_storage": map[string]any{"session_token": token}}}); err != nil {
				t.Fatal(err)
			}
			if len(bodies) != 5 {
				t.Fatalf("requests = %d", len(bodies))
			}
			for _, body := range bodies {
				wire, _ := json.Marshal(body)
				if !strings.Contains(string(wire), `"session_token":"`+token+`"`) {
					t.Fatal("request lost session token")
				}
			}
			if strings.Contains(fmt.Sprintf("%#v", storage), token) {
				t.Fatal("storage formatting leaked token")
			}
		})
	}
}

func TestSessionTokenErrors(t *testing.T) {
	token := "fixture-temporary-session-token"
	storage := Storage{StorageType: "s3", StorageURL: "bucket", AccessKey: Ptr("temp-key"), SecretKey: Ptr("temp-secret"), SessionToken: &token}
	client, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		respond(w, map[string]any{"code": "expired_token", "detail": "Expired " + token, "session_token": token})
	})
	input := YouTubeDownloadInput{VideoID: "dQw4w9WgXcQ", Params: YouTubeDownloadParams{Storage: storage}}
	if _, err := client.YouTube.Download(context.Background(), input, ""); err == nil || strings.Contains(fmt.Sprintf("%#v", err), token) || strings.Contains(err.Error(), token) {
		t.Fatal("token was not redacted from failure")
	}
	if strings.Contains(safeText("session_token="+token), token) {
		t.Fatal("unstructured token was not redacted")
	}
	for _, invalid := range []string{"", strings.Repeat("x", 16385)} {
		input.Params.Storage.SessionToken = &invalid
		if err := ValidateQuery(input.Request(), true); err == nil {
			t.Fatal("invalid token accepted")
		}
	}
}
