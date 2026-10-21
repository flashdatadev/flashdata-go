package flashdata

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"iter"

	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type BulkService struct{ c *Client }
type CreateBulkJobRequest struct {
	Source Source `json:"source"`
	// Options is a source Params struct or a map; data sources may additionally
	// include result_storage. Downloads require storage in this object.
	Options     any    `json:"options,omitempty"`
	CallbackURL string `json:"callback_url,omitempty"`
}
type UpdateBulkJobRequest struct {
	Options        map[string]any
	CallbackURL    *string
	RemoveCallback bool
}

// ValidateBulkCreate validates a bulk configuration without creating a job.
func ValidateBulkCreate(in CreateBulkJobRequest) error {
	switch in.Source {
	case SourceYouTubeMetadata, SourceYouTubeTranscript, SourceYouTubeCaptions, SourceYouTubeTrainability, SourceYouTubeDownload:
	default:
		return &ValidationError{Field: "source", Message: "choose one of the five YouTube bulk sources"}
	}
	b, err := json.Marshal(in.Options)
	if err != nil {
		return &ValidationError{Field: "options", Message: "must be JSON serializable"}
	}
	options := map[string]any{}
	if in.Options != nil {
		if err = json.Unmarshal(b, &options); err != nil || options == nil {
			return &ValidationError{Field: "options", Message: "must be an object"}
		}
	}
	if storage, ok := options["result_storage"]; ok {
		if in.Source == SourceYouTubeDownload {
			return &ValidationError{Field: "result_storage", Message: "downloads use options.storage"}
		}
		if storage != nil {
			// Validate the same storage shape without creating or submitting a download.
			if err = validateQuery(Query{Source: SourceYouTubeDownload, VideoID: "dQw4w9WgXcQ", Params: map[string]any{"storage": storage}}, true); err != nil {
				return err
			}
		}
		delete(options, "result_storage")
	}
	if err = validateQuery(Query{Source: in.Source, VideoID: "dQw4w9WgXcQ", Params: options}, true); err != nil {
		return err
	}
	if err = validateCallback(in.CallbackURL); err != nil {
		return err
	}
	return nil
}

func (r *BulkService) Create(ctx context.Context, in CreateBulkJobRequest) (*BulkJobCreated, error) {
	if err := ValidateBulkCreate(in); err != nil {
		return nil, err
	}
	var v BulkJobCreated
	if err := r.c.request(ctx, "POST", "/bulk-jobs", in, &v, false, false, ""); err != nil {
		if bulkPattern.MatchString(v.Job.ID) {
			err = withRecoveryIDs(err, []string{v.Job.ID})
		}
		return nil, err
	}
	if !bulkPattern.MatchString(v.Job.ID) || v.Upload.Href == "" {
		e := &SubmissionUnknownError{Cause: &ProtocolError{"incomplete bulk creation response"}}
		if bulkPattern.MatchString(v.Job.ID) {
			e.JobIDs = []string{v.Job.ID}
		}
		return nil, e
	}
	return &v, nil
}
func (r *BulkService) state(ctx context.Context, method, id, suffix string, body any) (*BulkJob, error) {
	path, err := jobPath(id, true)
	if err != nil {
		return nil, err
	}
	var v struct {
		Job *BulkJob `json:"job"`
	}
	if err = r.c.request(ctx, method, path+suffix, body, &v, false, false, ""); err != nil {
		return nil, err
	}
	if v.Job == nil || v.Job.ID != id || v.Job.Status == "" {
		e := &ProtocolError{"expected a matching bulk job object"}
		if method != "GET" {
			return nil, &SubmissionUnknownError{Cause: e, JobIDs: []string{id}}
		}
		return nil, e
	}
	return v.Job, nil
}
func (r *BulkService) Get(ctx context.Context, id string) (*BulkJob, error) {
	return r.state(ctx, "GET", id, "", nil)
}
func (r *BulkService) Start(ctx context.Context, id string) (*BulkJob, error) {
	return r.state(ctx, "POST", id, "/start", nil)
}
func (r *BulkService) Pause(ctx context.Context, id string) (*BulkJob, error) {
	return r.state(ctx, "POST", id, "/pause", nil)
}
func (r *BulkService) Resume(ctx context.Context, id string) (*BulkJob, error) {
	return r.state(ctx, "POST", id, "/resume", nil)
}
func (r *BulkService) Cancel(ctx context.Context, id string) (*BulkJob, error) {
	return r.state(ctx, "DELETE", id, "", nil)
}
func (r *BulkService) Update(ctx context.Context, id string, in UpdateBulkJobRequest) (*BulkJob, error) {
	if in.CallbackURL != nil && in.RemoveCallback {
		return nil, &ValidationError{Field: "callback", Message: "choose a replacement or removal"}
	}
	if in.CallbackURL != nil {
		if err := validateCallback(*in.CallbackURL); err != nil {
			return nil, err
		}
	}
	body := map[string]any{}
	if in.Options != nil {
		body["options"] = in.Options
	}
	if in.CallbackURL != nil {
		body["callback_url"] = *in.CallbackURL
	}
	if in.RemoveCallback {
		body["callback_url"] = nil
	}
	if len(body) == 0 {
		return nil, &ValidationError{Field: "update", Message: "provide options or callback change"}
	}
	return r.state(ctx, "PATCH", id, "", body)
}

type BulkListParams struct {
	Cursor string
	Limit  int
}

func (r *BulkService) List(ctx context.Context, p BulkListParams) (*BulkJobPage, error) {
	if p.Limit == 0 {
		p.Limit = 20
	}
	if p.Limit < 1 || p.Limit > 50 {
		return nil, &ValidationError{Field: "limit", Message: "must be between 1 and 50"}
	}
	q := url.Values{"limit": {strconv.Itoa(p.Limit)}}
	if p.Cursor != "" {
		q.Set("cursor", p.Cursor)
	}
	var v BulkJobPage
	err := r.c.request(ctx, "GET", "/bulk-jobs?"+q.Encode(), nil, &v, false, false, "")
	return &v, err
}
func (r *BulkService) Wait(ctx context.Context, id string, options PollOptions) (*BulkJob, error) {
	return poll(ctx, id, true, options, r.Get, func(v *BulkJob) string { return v.Status })
}

// Upload streams the signed upload exactly once. Reader must contain exactly
// size bytes (1 byte through 2 GiB). The API key is never sent to object storage.
func (r *BulkService) Upload(ctx context.Context, created *BulkJobCreated, input io.Reader, size int64) error {
	if created == nil || input == nil || size < 1 || size > 2<<30 {
		return &ValidationError{Field: "upload", Message: "provide a creation response and 1 byte through 2 GiB"}
	}
	if _, err := jobPath(created.Job.ID, true); err != nil {
		return err
	}
	u, err := url.Parse(created.Upload.Href)
	if err != nil || u.User != nil || u.Fragment != "" || u.Host == "" {
		return &ProtocolError{"invalid upload URL"}
	}
	uploadOrigin, err := origin(u.Scheme + "://" + u.Host)
	if err != nil {
		return &ProtocolError{"invalid upload URL origin"}
	}
	trusted, err := r.c.trustedResultOrigin(ctx)
	if err != nil {
		return err
	}
	if uploadOrigin != trusted {
		return &ProtocolError{"untrusted signed upload origin"}
	}
	if created.Upload.Method != "PUT" || created.Upload.ContentType != "text/plain; charset=utf-8" {
		return &ProtocolError{"unexpected upload method or content type"}
	}
	expires, err := time.Parse(time.RFC3339Nano, created.Upload.ExpiresAt)
	if err != nil || !time.Now().Before(expires) {
		return &ValidationError{Field: "upload", Message: "signed upload URL has expired or has no valid expiry"}
	}
	resp, cancel, err := r.c.raw(ctx, "PUT", created.Upload.Href, io.LimitReader(input, size), size, created.Upload.ContentType, requestAuth{}, true, 15*time.Minute)
	if err != nil {
		return err
	}
	defer cancel()
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, err := r.c.read(resp)
		if err != nil {
			return err
		}
		return r.c.apiError(resp, b, signedURLSecrets(u)...)
	}
	return nil
}
func (r *BulkService) UploadFile(ctx context.Context, created *BulkJobCreated, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return &ValidationError{Field: "file", Message: "could not open input file"}
	}
	defer f.Close()
	s, err := f.Stat()
	if err != nil || !s.Mode().IsRegular() {
		return &ValidationError{Field: "file", Message: "input must be a regular file"}
	}
	return r.Upload(ctx, created, f, s.Size())
}

type BulkManifest struct {
	ID          string             `json:"id"`
	Source      Source             `json:"source"`
	Status      string             `json:"status"`
	Counts      map[string]int64   `json:"counts"`
	Parts       []BulkManifestPart `json:"parts"`
	CompletedAt string             `json:"completed_at"`
	ExpiresAt   string             `json:"expires_at"`
}
type BulkManifestPart struct {
	Number     int    `json:"number"`
	FirstIndex int64  `json:"first_index"`
	LastIndex  int64  `json:"last_index"`
	ItemCount  int64  `json:"item_count"`
	Href       string `json:"href"`
	Method     string `json:"method"`
}

// BulkResultRow preserves all per-item fields, including large numeric values.
type BulkResultRow map[string]json.RawMessage

func (r *BulkService) Manifest(ctx context.Context, id string) (*BulkManifest, error) {
	path, err := jobPath(id, true)
	if err != nil {
		return nil, err
	}
	var v BulkManifest
	if err = r.c.resultJSON(ctx, path+"/results", &v); err != nil {
		return nil, err
	}
	if v.ID != id || v.Parts == nil {
		return nil, &ProtocolError{"manifest must match the requested job and contain a parts list"}
	}
	seen := map[int]bool{}
	for _, p := range v.Parts {
		if p.Number < 0 || seen[p.Number] {
			return nil, &ProtocolError{"invalid or duplicate manifest part number"}
		}
		seen[p.Number] = true
	}
	return &v, nil
}
func (r *BulkService) partStream(ctx context.Context, id string, part int) (io.ReadCloser, error) {
	path, err := jobPath(id, true)
	if err != nil {
		return nil, err
	}
	if part < 0 {
		return nil, &ValidationError{Field: "part", Message: "must be nonnegative"}
	}
	return r.c.resultStream(ctx, path+"/results/"+strconv.Itoa(part))
}

// Part returns the bounded compressed bytes. IterRows decompresses and streams.
func (r *BulkService) Part(ctx context.Context, id string, part int) ([]byte, error) {
	s, err := r.partStream(ctx, id, part)
	if err != nil {
		return nil, err
	}
	defer s.Close()
	return r.c.readBinary(s)
}
func (c *Client) readBinary(r io.Reader) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, c.maxBytes+1))
	if err != nil {
		if cause := transportCause(err); cause != ErrTransport {
			return nil, cause
		}
		return nil, &ProtocolError{"incomplete result part"}
	}
	if int64(len(b)) > c.maxBytes {
		return nil, ErrResponseTooLarge
	}
	return b, nil
}

func (r *BulkService) IterRows(ctx context.Context, id string, part int) iter.Seq2[BulkResultRow, error] {
	return func(yield func(BulkResultRow, error) bool) {
		s, err := r.partStream(ctx, id, part)
		if err != nil {
			yield(nil, err)
			return
		}
		defer s.Close()
		// Bound both compressed and decompressed bytes, including blank rows.
		compressed := &io.LimitedReader{R: s, N: r.c.maxBytes + 1}
		gz, err := gzip.NewReader(compressed)
		if err != nil {
			yield(nil, resultReadError(ctx, err, "invalid gzip part"))
			return
		}
		defer gz.Close()
		plain := &io.LimitedReader{R: gz, N: r.c.maxBytes + 1}
		reader := bufio.NewReaderSize(plain, min(r.c.maxLine, 64<<10))
		var pending []byte
		for {
			if err = ctx.Err(); err != nil {
				yield(nil, err)
				return
			}
			line, readErr := reader.ReadSlice('\n')
			pending = append(pending, line...)
			if plain.N <= 0 || compressed.N <= 0 {
				yield(nil, ErrResponseTooLarge)
				return
			}
			if len(pending) > r.c.maxLine+2 {
				yield(nil, ErrResponseTooLarge)
				return
			}
			if readErr == bufio.ErrBufferFull {
				continue
			}
			if len(strings.TrimSuffix(strings.TrimSuffix(string(pending), "\n"), "\r")) > r.c.maxLine {
				yield(nil, ErrResponseTooLarge)
				return
			}
			if readErr != nil && readErr != io.EOF {
				yield(nil, resultReadError(ctx, readErr, "invalid or truncated gzip part"))
				return
			}
			text := strings.TrimSpace(string(pending))
			pending = nil
			if text != "" {
				var row BulkResultRow
				if !utf8.ValidString(text) || json.Unmarshal([]byte(text), &row) != nil || row == nil {
					yield(nil, &ProtocolError{"expected a UTF-8 NDJSON object row"})
					return
				}
				if !yield(row, nil) {
					return
				}
			}
			if readErr == io.EOF {
				return
			}
		}
	}
}

func resultReadError(ctx context.Context, err error, message string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if cause := transportCause(err); cause != ErrTransport {
		return cause
	}
	return &ProtocolError{message}
}
func (r *BulkService) Rows(ctx context.Context, id string) iter.Seq2[BulkResultRow, error] {
	return func(yield func(BulkResultRow, error) bool) {
		m, err := r.Manifest(ctx, id)
		if err != nil {
			yield(nil, err)
			return
		}
		for _, p := range m.Parts {
			for row, err := range r.IterRows(ctx, id, p.Number) {
				if !yield(row, err) || err != nil {
					return
				}
			}
		}
	}
}
func (r *BulkService) IterJobs(ctx context.Context, p BulkListParams, maxPages int) iter.Seq2[*BulkJob, error] {
	return func(yield func(*BulkJob, error) bool) {
		if maxPages < 1 || maxPages > 1000 {
			yield(nil, &ValidationError{Field: "maxPages", Message: "must be between 1 and 1000"})
			return
		}
		seen := map[string]bool{}
		if p.Cursor != "" {
			seen[p.Cursor] = true
		}
		for range maxPages {
			page, err := r.List(ctx, p)
			if err != nil {
				yield(nil, err)
				return
			}
			if page.Data == nil {
				yield(nil, &ProtocolError{"expected a bulk listing array"})
				return
			}
			for i := range page.Data {
				if !yield(&page.Data[i], nil) {
					return
				}
			}
			if !page.Pagination.HasMore {
				return
			}
			if page.Pagination.NextCursor == nil || *page.Pagination.NextCursor == "" || seen[*page.Pagination.NextCursor] {
				yield(nil, &ProtocolError{"invalid or repeated bulk cursor"})
				return
			}
			p.Cursor = *page.Pagination.NextCursor
			seen[p.Cursor] = true
		}
	}
}
