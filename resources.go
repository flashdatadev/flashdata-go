package flashdata

import (
	"context"
	"encoding/json"
	"errors"
	"iter"
	"net/url"
	"regexp"
	"strconv"
	"time"
)

type AccountService struct{ c *Client }
type PublicService struct{ c *Client }
type JobsService struct{ c *Client }

func (r *AccountService) Access(ctx context.Context) (*APIAccess, error) {
	var v APIAccess
	err := r.c.request(ctx, "GET", "/api-access", nil, &v, false, false, "")
	return &v, err
}
func (r *AccountService) Balance(ctx context.Context) (*APIAccessBalance, error) {
	var v APIAccessBalance
	err := r.c.request(ctx, "GET", "/api-access/balance", nil, &v, false, false, "")
	return &v, err
}
func (r *AccountService) Limits(ctx context.Context) (*APIAccessLimits, error) {
	var v APIAccessLimits
	err := r.c.request(ctx, "GET", "/api-access/limits", nil, &v, false, false, "")
	return &v, err
}

type PricingParams struct {
	Source       Source
	RequestCount int
}

func (r *AccountService) Pricing(ctx context.Context, p PricingParams) (*APIAccessPricing, error) {
	if p.RequestCount == 0 {
		p.RequestCount = 1
	}
	if p.RequestCount < 1 || p.RequestCount > 100000 {
		return nil, &ValidationError{Field: "requestCount", Message: "must be between 1 and 100000"}
	}
	if err := optionalSource(p.Source); err != nil {
		return nil, err
	}
	q := url.Values{"request_count": {strconv.Itoa(p.RequestCount)}}
	if p.Source != "" {
		q.Set("source", string(p.Source))
	}
	var v APIAccessPricing
	err := r.c.request(ctx, "GET", "/api-access/pricing?"+q.Encode(), nil, &v, false, false, "")
	return &v, err
}

type UsageParams struct {
	Source   Source
	From, To time.Time
}

func (r *AccountService) Usage(ctx context.Context, p UsageParams) (*APIKeyUsage, error) {
	if err := optionalSource(p.Source); err != nil {
		return nil, err
	}
	if !p.From.IsZero() && !p.To.IsZero() && !p.From.Before(p.To) {
		return nil, &ValidationError{Field: "from", Message: "must precede to"}
	}
	q := url.Values{}
	if p.Source != "" {
		q.Set("source", string(p.Source))
	}
	if !p.From.IsZero() {
		q.Set("from", p.From.UTC().Format(time.RFC3339Nano))
	}
	if !p.To.IsZero() {
		q.Set("to", p.To.UTC().Format(time.RFC3339Nano))
	}
	var v APIKeyUsage
	err := r.c.request(ctx, "GET", "/usage?"+q.Encode(), nil, &v, false, false, "")
	return &v, err
}
func optionalSource(s Source) error {
	if s == "" {
		return nil
	}
	schemasOnce.Do(compileSchemas)
	if sourceValidators[string(s)] == nil {
		return &ValidationError{Field: "source", Message: "unsupported source"}
	}
	return nil
}

func (r *PublicService) Sources(ctx context.Context) ([]DataSource, error) {
	var v struct {
		Data []DataSource `json:"data"`
	}
	err := r.c.request(ctx, "GET", "/catalog/sources", nil, &v, false, true, "")
	return v.Data, err
}
func (r *PublicService) Plans(ctx context.Context) (*PlanCatalog, error) {
	var v PlanCatalog
	err := r.c.request(ctx, "GET", "/plans", nil, &v, false, true, "")
	return &v, err
}
func (r *PublicService) Health(ctx context.Context) (*Health, error) {
	var v Health
	err := r.c.request(ctx, "GET", "/health", nil, &v, false, true, "")
	return &v, err
}
func (r *PublicService) Status(ctx context.Context) (*PublicStatus, error) {
	var v PublicStatus
	err := r.c.request(ctx, "GET", "/status", nil, &v, false, true, "")
	return &v, err
}

var jobPattern = regexp.MustCompile(`^job_[A-Za-z0-9_-]+$`)
var bulkPattern = regexp.MustCompile(`^bulk_[A-Za-z0-9_-]{8,}$`)

func jobPath(id string, bulk bool) (string, error) {
	pattern := jobPattern
	prefix := "/jobs/"
	if bulk {
		pattern = bulkPattern
		prefix = "/bulk-jobs/"
	}
	if !pattern.MatchString(id) {
		return "", &ValidationError{Field: "id", Message: "invalid job identifier"}
	}
	return prefix + id, nil
}
func (r *JobsService) Get(ctx context.Context, id string) (*ConsoleJob, error) {
	path, err := jobPath(id, false)
	if err != nil {
		return nil, err
	}
	var v struct {
		Job *ConsoleJob `json:"job"`
	}
	if err = r.c.request(ctx, "GET", path, nil, &v, false, false, ""); err != nil {
		return nil, err
	}
	if v.Job == nil || v.Job.ID != id || v.Job.Status == "" {
		return nil, &ProtocolError{"expected a matching job object"}
	}
	return v.Job, nil
}

// JobResult retains every source-specific result field without numeric loss.
type JobResult map[string]json.RawMessage

func (r *JobsService) Result(ctx context.Context, id string) (JobResult, error) {
	path, err := jobPath(id, false)
	if err != nil {
		return nil, err
	}
	var v JobResult
	err = r.c.resultJSON(ctx, path+"/results", &v)
	return v, err
}

type PollOptions struct{ Timeout, Interval time.Duration }
type PollingInterruptedError struct {
	JobID string
	Cause error
}

func (e *PollingInterruptedError) Error() string {
	return "flashdata: polling interrupted; job " + e.JobID + " may continue"
}
func (e *PollingInterruptedError) Unwrap() error { return e.Cause }

func poll[T any](ctx context.Context, id string, bulk bool, options PollOptions, get func(context.Context, string) (*T, error), status func(*T) string) (*T, error) {
	if _, err := jobPath(id, bulk); err != nil {
		return nil, err
	}
	if options.Timeout < 0 || options.Interval < 0 {
		return nil, &ValidationError{Field: "pollOptions", Message: "durations must be positive"}
	}
	if options.Timeout == 0 {
		options.Timeout = 5 * time.Minute
		if bulk {
			options.Timeout = time.Hour
		}
	}
	if options.Interval == 0 {
		options.Interval = 2 * time.Second
		if bulk {
			options.Interval = 5 * time.Second
		}
	}
	pollctx, cancel := context.WithTimeout(ctx, options.Timeout)
	defer cancel()
	interrupted := func() error {
		if ctx.Err() != nil {
			return &PollingInterruptedError{JobID: id, Cause: ctx.Err()}
		}
		return &PollingTimeoutError{JobID: id}
	}
	for {
		if pollctx.Err() != nil {
			return nil, interrupted()
		}
		v, err := get(pollctx, id)
		if err != nil {
			if pollctx.Err() != nil {
				return nil, interrupted()
			}
			return nil, err
		}
		if pollctx.Err() != nil {
			return nil, interrupted()
		}
		s := status(v)
		switch s {
		case "completed":
			return v, nil
		case "failed", "cancelled":
			return nil, &JobFailedError{JobID: id, Status: s}
		case "paused":
			if bulk {
				return nil, &JobPausedError{JobID: id}
			}
			return nil, &ProtocolError{"unexpected job status"}
		case "awaiting_upload", "running", "finalizing":
			if !bulk {
				return nil, &ProtocolError{"unexpected job status"}
			}
		case "pending", "processing":
			if bulk {
				return nil, &ProtocolError{"unexpected bulk status"}
			}
		default:
			return nil, &ProtocolError{"unexpected job status"}
		}
		timer := time.NewTimer(options.Interval)
		select {
		case <-pollctx.Done():
			timer.Stop()
			return nil, interrupted()
		case <-timer.C:
		}
	}
}
func (r *JobsService) Wait(ctx context.Context, id string, options PollOptions) (*ConsoleJob, error) {
	return poll(ctx, id, false, options, r.Get, func(v *ConsoleJob) string { return v.Status })
}

// Pages yields a bounded number of Google Search pages. Every page is a separate
// potentially billed realtime query. Early break stops further requests.
func (r *GoogleService) Pages(ctx context.Context, input GoogleSearchInput, maxPages int) iter.Seq2[*DataQueryResponse, error] {
	return func(yield func(*DataQueryResponse, error) bool) {
		if maxPages < 1 || maxPages > 100 {
			yield(nil, &ValidationError{Field: "maxPages", Message: "must be between 1 and 100"})
			return
		}
		params := GoogleSearchParams{}
		if input.Params != nil {
			params = *input.Params
		}
		start := int64(1)
		if params.Page != nil {
			start = *params.Page
		}
		if start < 1 || start+int64(maxPages)-1 > 100 {
			yield(nil, &ValidationError{Field: "page", Message: "requested pages exceed 1 through 100"})
			return
		}
		for page := start; page < start+int64(maxPages); page++ {
			params.Page = Ptr(page)
			input.Params = &params
			v, err := r.Search(ctx, input)
			if !yield(v, err) || err != nil {
				return
			}
		}
	}
}

// IsSubmissionUnknown is a convenience for callers with explicit retry policy.
func IsSubmissionUnknown(err error) bool {
	var unknown *SubmissionUnknownError
	return errors.As(err, &unknown)
}
