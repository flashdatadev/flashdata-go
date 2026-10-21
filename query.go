package flashdata

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

type Source string

const (
	SourceGoogleSearch          Source = "google_search"
	SourceGoogleImages          Source = "google_images"
	SourceGoogleNews            Source = "google_news"
	SourceGoogleShopping        Source = "google_shopping"
	SourceGoogleVideos          Source = "google_videos"
	SourceGooglePlaces          Source = "google_places"
	SourceYouTubeSearch         Source = "youtube_search"
	SourceYouTubeSearchExtended Source = "youtube_search_extended"
	SourceYouTubeMetadata       Source = "youtube_metadata"
	SourceYouTubeTranscript     Source = "youtube_transcript"
	SourceYouTubeCaptions       Source = "youtube_captions"
	SourceYouTubeTrainability   Source = "youtube_trainability"
	SourceYouTubeChannel        Source = "youtube_channel"
	SourceYouTubeSuggestions    Source = "youtube_suggestions"
	SourceYouTubeDownload       Source = "youtube_download"
)

// Ptr preserves explicit false, zero, and empty values in optional parameters.
func Ptr[T any](v T) *T { return &v }

// Query is the common wire request. Prefer source-specific Input.Request()
// when available; Params also accepts a JSON-serializable map for new callers.
type Query struct {
	Source      Source `json:"source"`
	Query       string `json:"query,omitempty"`
	VideoID     string `json:"video_id,omitempty"`
	Handle      string `json:"handle,omitempty"`
	Params      any    `json:"params,omitempty"`
	CallbackURL string `json:"callback_url,omitempty"`
}

type QueriesService struct{ c *Client }
type GoogleService struct{ c *Client }
type YouTubeService struct{ c *Client }

//go:embed internal/contract/schemas.json
var sourceSchemaBytes []byte
var schemasOnce sync.Once
var sourceValidators map[string]*jsonschema.Schema
var schemasError error

func compileSchemas() {
	sourceValidators = make(map[string]*jsonschema.Schema)
	var sources map[string]map[string]any
	if schemasError = json.Unmarshal(sourceSchemaBytes, &sources); schemasError != nil {
		return
	}
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat = true
	for source, schema := range sources {
		schema["additionalProperties"] = false
		// The realtime callback restriction is checked separately.
		schema["properties"].(map[string]any)["callback_url"] = map[string]any{"type": "string", "maxLength": 2048}
		b, _ := json.Marshal(schema)
		id := "https://sdk.flashdata.dev/" + source + ".json"
		if schemasError = compiler.AddResource(id, bytes.NewReader(b)); schemasError != nil {
			return
		}
		v, err := compiler.Compile(id)
		if err != nil {
			schemasError = err
			return
		}
		sourceValidators[source] = v
	}
}

func validateQuery(q Query, async bool) error {
	schemasOnce.Do(compileSchemas)
	if schemasError != nil {
		return &ProtocolError{"embedded source contract could not compile"}
	}
	v := sourceValidators[string(q.Source)]
	if v == nil {
		return &ValidationError{Field: "source", Message: "choose one of the 15 supported sources"}
	}
	if !async && q.Source == SourceYouTubeDownload {
		return &ValidationError{Field: "source", Message: "downloads support async and batch only"}
	}
	if !async && q.CallbackURL != "" {
		return &ValidationError{Field: "callback_url", Message: "callbacks apply only to background queries"}
	}
	b, err := json.Marshal(q)
	if err != nil {
		return &ValidationError{Field: "query", Message: "values must be JSON serializable"}
	}
	var value any
	_ = json.Unmarshal(b, &value)
	if err = v.Validate(value); err != nil {
		field := "query"
		rule := "source contract"
		if e, ok := err.(*jsonschema.ValidationError); ok {
			for len(e.Causes) > 0 {
				e = e.Causes[0]
			}
			field = e.InstanceLocation
			parts := strings.Split(e.KeywordLocation, "/")
			rule = parts[len(parts)-1]
		}
		return &ValidationError{Field: field, Message: "violates " + rule + "; see source reference"}
	}
	for field, v := range map[string]string{"query": q.Query, "video_id": q.VideoID, "handle": q.Handle} {
		if v != "" && strings.TrimSpace(v) == "" {
			return &ValidationError{Field: field, Message: "must not be blank"}
		}
	}
	return validateCallback(q.CallbackURL)
}

func validateCallback(value string) error {
	if value == "" {
		return nil
	}
	u, err := url.Parse(value)
	bad := err != nil
	if !bad {
		host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
		ip := net.ParseIP(host)
		bad = u.Scheme != "https" || u.User != nil || u.Fragment != "" || (u.Port() != "" && u.Port() != "443") || !strings.Contains(host, ".") || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local")
		if ip != nil {
			bad = bad || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast()
			for _, s := range []string{"0.0.0.0/8", "240.0.0.0/4", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24"} {
				_, n, _ := net.ParseCIDR(s)
				bad = bad || n.Contains(ip)
			}
		}
	}
	if bad || len(value) > 2048 {
		return &ValidationError{Field: "callback_url", Message: "use a public HTTPS URL on port 443"}
	}
	return nil
}

func (r *QueriesService) Realtime(ctx context.Context, q Query) (*DataQueryResponse, error) {
	if err := validateQuery(q, false); err != nil {
		return nil, err
	}
	var result DataQueryResponse
	if err := r.c.request(ctx, "POST", "/v1/queries/realtime", q, &result, true, false, ""); err != nil {
		return nil, err
	}
	if result.Source != string(q.Source) || result.Status == "" || result.Results == nil {
		return nil, &SubmissionUnknownError{Cause: &ProtocolError{"incomplete realtime response"}}
	}
	return &result, nil
}
func (r *QueriesService) Submit(ctx context.Context, q Query) (*AsyncSubmission, error) {
	if err := validateQuery(q, true); err != nil {
		return nil, err
	}
	var result AsyncSubmission
	if err := r.c.request(ctx, "POST", "/v1/queries", q, &result, true, false, ""); err != nil {
		return nil, withRecoveryIDs(err, submissionIDs([]AsyncSubmission{result}))
	}
	if !jobPattern.MatchString(result.ID) || result.Source != string(q.Source) || result.Status == "" {
		return nil, &SubmissionUnknownError{Cause: &ProtocolError{"inconsistent submission response"}, JobIDs: submissionIDs([]AsyncSubmission{result})}
	}
	return &result, nil
}

type BatchResponse struct {
	Queries      []AsyncSubmission      `json:"queries"`
	TotalCredits float64                `json:"total_credits"`
	Usage        DataQueryResponseUsage `json:"usage"`
}

func (r *QueriesService) Batch(ctx context.Context, queries []Query) (*BatchResponse, error) {
	if len(queries) == 0 {
		return nil, &ValidationError{Field: "queries", Message: "provide a non-empty array; plan limits apply"}
	}
	for _, q := range queries {
		if err := validateQuery(q, true); err != nil {
			return nil, err
		}
	}
	var result BatchResponse
	err := r.c.request(ctx, "POST", "/v1/queries/batch", map[string]any{"queries": queries}, &result, true, false, "")
	if err != nil {
		return nil, withRecoveryIDs(err, submissionIDs(result.Queries))
	}
	if len(result.Queries) != len(queries) {
		return nil, &SubmissionUnknownError{Cause: &ProtocolError{"batch response has an unexpected number of submissions"}, JobIDs: submissionIDs(result.Queries)}
	}
	seen := make(map[string]bool, len(result.Queries))
	for i, q := range result.Queries {
		if !jobPattern.MatchString(q.ID) || seen[q.ID] || q.Source != string(queries[i].Source) || q.Status == "" {
			return nil, &SubmissionUnknownError{Cause: &ProtocolError{"batch response contains inconsistent submissions"}, JobIDs: submissionIDs(result.Queries)}
		}
		seen[q.ID] = true
	}
	return &result, nil
}

func submissionIDs(submissions []AsyncSubmission) []string {
	var ids []string
	seen := map[string]bool{}
	for _, v := range submissions {
		if jobPattern.MatchString(v.ID) && !seen[v.ID] {
			ids = append(ids, v.ID)
			seen[v.ID] = true
		}
	}
	return ids
}

// DecodeResults decodes source payloads into a caller-chosen type. Raw results
// are retained as json.RawMessage so new fields and large integers survive.
func DecodeResults[T any](response *DataQueryResponse) ([]T, error) {
	if response == nil {
		return nil, &ValidationError{Field: "response", Message: "must not be nil"}
	}
	values := make([]T, 0, len(response.Results))
	for i, b := range response.Results {
		var v T
		d := json.NewDecoder(bytes.NewReader(b))
		d.UseNumber()
		if err := d.Decode(&v); err != nil {
			return nil, fmt.Errorf("flashdata: result %d cannot decode into requested type", i)
		}
		values = append(values, v)
	}
	return values, nil
}
