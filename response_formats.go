package flashdata

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// Accept selects the outer response type. Its zero value selects JSON.
type Accept string

const (
	AcceptJSON Accept = "application/json"
	AcceptText Accept = "text/plain"
)

// Representation selects the content representation. Its zero value preserves
// the original results. Only content responses support Markdown.
type Representation string

const RespondMarkdown Representation = "markdown"

// ResponseOptions configures receipt or status responses for one call.
type ResponseOptions struct {
	Accept Accept
}

// ContentOptions configures realtime or completed-result responses for one call.
// These values become headers, never query parameters or stored job settings.
type ContentOptions struct {
	Accept      Accept
	RespondWith Representation
}

func (o ContentOptions) validate() error {
	if o.Accept != "" && o.Accept != AcceptJSON && o.Accept != AcceptText {
		return &ValidationError{Field: "accept", Message: "choose application/json or text/plain"}
	}
	if o.RespondWith != "" && o.RespondWith != RespondMarkdown {
		return &ValidationError{Field: "respondWith", Message: "choose markdown or omit"}
	}
	return nil
}

// FormattedResponse contains the exact response body in JSON or Text, according
// to Accept. JSON preserves all wire fields and numbers; Text is UTF-8 text.
// JSON status responses retain the outer job object, unlike Jobs.Get.
type FormattedResponse struct {
	JSON json.RawMessage
	Text string
}

// DecodeJSON decodes JSON into a caller-chosen wire model, such as
// MarkdownQueryResult, DataQueryResponse or BatchResponse. It rejects text.
func (r *FormattedResponse) DecodeJSON(out any) error {
	if r == nil || len(r.JSON) == 0 {
		return &ValidationError{Field: "response", Message: "no JSON body; use Text for text/plain"}
	}
	return json.Unmarshal(r.JSON, out)
}

func (r *FormattedResponse) setBody(b []byte, accept Accept) error {
	if accept == AcceptText {
		r.Text = string(b)
		r.JSON = nil
		return nil
	}
	trimmed := bytes.TrimSpace(b)
	if len(trimmed) == 0 || trimmed[0] != '{' || !json.Valid(trimmed) {
		return &ProtocolError{"invalid JSON response"}
	}
	r.JSON = append(json.RawMessage(nil), b...)
	r.Text = ""
	return nil
}

// Keep compressed and decompressed bodies bounded, including error bodies.
// Bulk parts use their separate streaming/gzip reader.
func (c *Client) readFormatted(ctx context.Context, resp *http.Response) ([]byte, error) {
	switch strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Encoding"))) {
	case "", "identity":
		return c.read(resp)
	case "gzip":
		compressed := &io.LimitedReader{R: resp.Body, N: c.maxBytes + 1}
		gz, err := gzip.NewReader(compressed)
		if compressed.N <= 0 {
			return nil, ErrResponseTooLarge
		}
		if err != nil {
			return nil, resultReadError(ctx, err, "invalid gzip response")
		}
		defer gz.Close()
		b, err := c.read(&http.Response{Body: io.NopCloser(gz)})
		if compressed.N <= 0 {
			return nil, ErrResponseTooLarge
		}
		return b, err
	default:
		return nil, &ProtocolError{"unsupported response content encoding"}
	}
}

// RealtimeWithFormat runs a query with per-call response headers. The existing
// Realtime method continues to return a typed original JSON response.
func (r *QueriesService) RealtimeWithFormat(ctx context.Context, q Query, options ContentOptions) (*FormattedResponse, error) {
	if err := options.validate(); err != nil {
		return nil, err
	}
	if err := validateQuery(q, false); err != nil {
		return nil, err
	}
	var v FormattedResponse
	if err := r.c.requestWithFormat(ctx, "POST", "/v1/queries/realtime", q, &v, true, false, "", options); err != nil {
		return nil, err
	}
	if options.Accept != AcceptText {
		var result DataQueryResponse
		err := v.DecodeJSON(&result)
		valid := err == nil && result.Source == string(q.Source) && result.Status != "" && result.Results != nil
		if options.RespondWith == RespondMarkdown {
			markdown, markdownErr := v.markdown()
			valid = markdownErr == nil && markdown.Source == string(q.Source)
		}
		if !valid {
			return nil, &SubmissionUnknownError{Cause: &ProtocolError{"incomplete realtime response"}}
		}
	}
	return &v, nil
}

// SubmitWithFormat submits work and returns a JSON or plain-text receipt.
// Prefer Submit when your application needs typed job IDs for recovery.
func (r *QueriesService) SubmitWithFormat(ctx context.Context, q Query, options ResponseOptions) (*FormattedResponse, error) {
	format := ContentOptions{Accept: options.Accept}
	if err := format.validate(); err != nil {
		return nil, err
	}
	if err := validateQuery(q, true); err != nil {
		return nil, err
	}
	var v FormattedResponse
	if err := r.c.requestWithFormat(ctx, "POST", "/v1/queries", q, &v, true, false, "", format); err != nil {
		return nil, err
	}
	if options.Accept != AcceptText {
		var receipt AsyncSubmission
		err := v.DecodeJSON(&receipt)
		if err != nil || !jobPattern.MatchString(receipt.ID) || receipt.Source != string(q.Source) || receipt.Status == "" {
			return nil, &SubmissionUnknownError{Cause: &ProtocolError{"inconsistent submission response"}, JobIDs: submissionIDs([]AsyncSubmission{receipt})}
		}
	}
	return &v, nil
}

// BatchWithFormat submits a batch with JSON or plain-text receipts.
func (r *QueriesService) BatchWithFormat(ctx context.Context, queries []Query, options ResponseOptions) (*FormattedResponse, error) {
	format := ContentOptions{Accept: options.Accept}
	if err := format.validate(); err != nil {
		return nil, err
	}
	if len(queries) == 0 {
		return nil, &ValidationError{Field: "queries", Message: "provide a non-empty array; plan limits apply"}
	}
	for _, q := range queries {
		if err := validateQuery(q, true); err != nil {
			return nil, err
		}
	}
	var v FormattedResponse
	if err := r.c.requestWithFormat(ctx, "POST", "/v1/queries/batch", map[string]any{"queries": queries}, &v, true, false, "", format); err != nil {
		return nil, err
	}
	if options.Accept != AcceptText {
		var receipts BatchResponse
		err := v.DecodeJSON(&receipts)
		valid := err == nil && len(receipts.Queries) == len(queries)
		seen := map[string]bool{}
		for i, receipt := range receipts.Queries {
			if i >= len(queries) || !jobPattern.MatchString(receipt.ID) || seen[receipt.ID] || receipt.Source != string(queries[i].Source) || receipt.Status == "" {
				valid = false
			}
			seen[receipt.ID] = true
		}
		if !valid {
			return nil, &SubmissionUnknownError{Cause: &ProtocolError{"inconsistent batch response"}, JobIDs: submissionIDs(receipts.Queries)}
		}
	}
	return &v, nil
}

// GetWithFormat reads a job's full JSON or plain-text status envelope.
func (r *JobsService) GetWithFormat(ctx context.Context, id string, options ResponseOptions) (*FormattedResponse, error) {
	format := ContentOptions{Accept: options.Accept}
	if err := format.validate(); err != nil {
		return nil, err
	}
	path, err := jobPath(id, false)
	if err != nil {
		return nil, err
	}
	var v FormattedResponse
	if err := r.c.requestWithFormat(ctx, "GET", path, nil, &v, false, false, "", format); err != nil {
		return nil, err
	}
	if options.Accept != AcceptText {
		var envelope struct {
			Job *ConsoleJob `json:"job"`
		}
		err := v.DecodeJSON(&envelope)
		if err != nil || envelope.Job == nil || envelope.Job.ID != id || envelope.Job.Status == "" {
			return nil, &ProtocolError{"expected a matching job object"}
		}
	}
	return &v, nil
}

// ResultWithFormat reads a completed result without submitting another query.
// Download manifests support Markdown; media bytes keep their original format.
func (r *JobsService) ResultWithFormat(ctx context.Context, id string, options ContentOptions) (*FormattedResponse, error) {
	if err := options.validate(); err != nil {
		return nil, err
	}
	path, err := jobPath(id, false)
	if err != nil {
		return nil, err
	}
	var v FormattedResponse
	if options.Accept != AcceptText && options.RespondWith == "" {
		var raw json.RawMessage
		if err := r.c.resultJSON(ctx, path+"/results", &raw); err != nil {
			return nil, err
		}
		err = v.setBody(raw, AcceptJSON)
	} else {
		err = r.c.requestWithFormat(ctx, "GET", path+"/results", nil, &v, false, false, "", options)
	}
	if err != nil {
		return nil, err
	}
	if options.Accept != AcceptText && options.RespondWith == RespondMarkdown {
		markdown, err := v.markdown()
		if err != nil {
			return nil, err
		}
		if markdown.ID != "" && markdown.ID != id {
			return nil, &ProtocolError{"expected matching result metadata"}
		}
	}
	return &v, nil
}

func (r *FormattedResponse) markdown() (*MarkdownQueryResult, error) {
	var model MarkdownQueryResult
	var fields map[string]json.RawMessage
	if err := r.DecodeJSON(&model); err != nil {
		return nil, &ProtocolError{"invalid Markdown response"}
	}
	_ = r.DecodeJSON(&fields)
	content, hasContent := fields["content"]
	_, hasResults := fields["results"]
	if !hasContent || bytes.Equal(bytes.TrimSpace(content), []byte("null")) || hasResults || model.Source == "" || model.Status == "" {
		return nil, &ProtocolError{"expected Markdown content without results"}
	}
	return &model, nil
}
