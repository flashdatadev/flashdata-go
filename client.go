// Package flashdata provides API-key access to FlashData search, YouTube, jobs,
// bulk processing and optional agent onboarding. See https://flashdata.dev.
// Clients are safe for concurrent use. No method loads environment files,
// retries submissions, creates accounts implicitly, or writes credentials to disk.
package flashdata

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const Version = "1.0.0"

// Options configures a client. APIKey must be passed explicitly for key APIs.
// Empty APIKey permits public discovery and proof-authenticated onboarding only.
type Options struct {
	APIKey           string
	DataOrigin       string
	ManagementOrigin string
	// ResultOrigin optionally pins an operator-provided signed-result origin.
	// Otherwise it is discovered from the authenticated Account.Access endpoint.
	ResultOrigin     string
	HTTPClient       *http.Client
	Timeout          time.Duration
	MaxResponseBytes int64
	MaxLineBytes     int
}

type Client struct {
	key, dataOrigin, managementOrigin string
	http                              *http.Client
	timeout                           time.Duration
	maxBytes                          int64
	maxLine                           int
	resultMu                          sync.Mutex
	resultLoading                     chan struct{}
	resultOrigin                      string
	Queries                           *QueriesService
	Google                            *GoogleService
	YouTube                           *YouTubeService
	Jobs                              *JobsService
	Bulk                              *BulkService
	Account                           *AccountService
	Public                            *PublicService
	Onboarding                        *OnboardingService
}

func (c *Client) String() string   { return "flashdata.Client{credentials:redacted}" }
func (c *Client) GoString() string { return c.String() }

func origin(value string) (string, error) {
	u, err := url.Parse(value)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || (u.Path != "" && u.Path != "/") || u.Opaque != "" {
		return "", &ValidationError{Field: "origin", Message: "use an HTTPS origin without credentials, path, query or fragment"}
	}
	ip := net.ParseIP(u.Hostname())
	local := u.Hostname() == "localhost" || (ip != nil && ip.IsLoopback())
	if u.Scheme != "https" && !(u.Scheme == "http" && local) {
		return "", &ValidationError{Field: "origin", Message: "HTTPS is required except on loopback"}
	}
	if port := u.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return "", &ValidationError{Field: "origin", Message: "invalid port"}
		}
	}
	if (u.Scheme == "https" && u.Port() == "443") || (u.Scheme == "http" && u.Port() == "80") {
		host := u.Hostname()
		if strings.Contains(host, ":") {
			host = "[" + host + "]"
		}
		u.Host = host
	}
	return strings.ToLower(u.Scheme + "://" + u.Host), nil
}

func New(options Options) (*Client, error) {
	if options.DataOrigin == "" {
		options.DataOrigin = "https://data.flashdata.dev"
	}
	if options.ManagementOrigin == "" {
		options.ManagementOrigin = "https://api.flashdata.dev"
	}
	data, err := origin(options.DataOrigin)
	if err != nil {
		return nil, err
	}
	management, err := origin(options.ManagementOrigin)
	if err != nil {
		return nil, err
	}
	if options.ResultOrigin != "" {
		options.ResultOrigin, err = origin(options.ResultOrigin)
		if err != nil {
			return nil, err
		}
	}
	if options.APIKey != "" && !credentialPattern.MatchString(options.APIKey) {
		return nil, &ValidationError{Field: "apiKey", Message: "invalid credential characters"}
	}
	if options.Timeout < 0 || options.MaxResponseBytes < 0 || options.MaxResponseBytes == int64(^uint64(0)>>1) || options.MaxLineBytes < 0 || options.MaxLineBytes >= int(^uint(0)>>1)-1 {
		return nil, &ValidationError{Field: "options", Message: "limits must be positive and leave room for overflow detection"}
	}
	if options.Timeout == 0 {
		options.Timeout = 120 * time.Second
	}
	if options.MaxResponseBytes == 0 {
		options.MaxResponseBytes = 64 << 20
	}
	if options.MaxLineBytes == 0 {
		options.MaxLineBytes = 16 << 20
	}
	h := &http.Client{}
	if options.HTTPClient != nil {
		*h = *options.HTTPClient
	}
	// Neither session cookies nor redirects may transfer credentials to storage.
	h.Jar = nil
	h.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	c := &Client{key: options.APIKey, dataOrigin: data, managementOrigin: management, resultOrigin: options.ResultOrigin, http: h, timeout: options.Timeout, maxBytes: options.MaxResponseBytes, maxLine: options.MaxLineBytes}
	c.Queries = &QueriesService{c}
	c.Google = &GoogleService{c}
	c.YouTube = &YouTubeService{c}
	c.Jobs = &JobsService{c}
	c.Bulk = &BulkService{c}
	c.Account = &AccountService{c}
	c.Public = &PublicService{c}
	c.Onboarding = &OnboardingService{c}
	return c, nil
}

var credentialPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,256}$`)

// Keep the authentication mode separate from the credential value: any valid
// proof, including the literal "key", must stay a bearer credential.
type requestAuth struct {
	apiKey bool
	bearer string
}

// raw never follows redirects. Bearer credentials replace API-key authentication.
func (c *Client) raw(ctx context.Context, method, target string, body io.Reader, size int64, contentType string, auth requestAuth, mutation bool, budgets ...time.Duration) (*http.Response, context.CancelFunc, error) {
	return c.rawWithFormat(ctx, method, target, body, size, contentType, auth, mutation, ContentOptions{}, budgets...)
}

func (c *Client) rawWithFormat(ctx context.Context, method, target string, body io.Reader, size int64, contentType string, auth requestAuth, mutation bool, format ContentOptions, budgets ...time.Duration) (*http.Response, context.CancelFunc, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	timeout := c.timeout
	if len(budgets) > 0 {
		timeout = budgets[0]
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		cancel()
		return nil, nil, &ProtocolError{"could not create request"}
	}
	if size >= 0 {
		req.ContentLength = size
	}
	req.Header.Set("Accept", "application/json")
	if format.Accept != "" {
		req.Header.Set("Accept", string(format.Accept))
	}
	if format.RespondWith != "" {
		req.Header.Set("X-Respond-With", string(format.RespondWith))
	}
	// Result parts are gzip files themselves. Request identity encoding so the
	// HTTP transport cannot silently decompress a part before IterRows sees it.
	req.Header.Set("Accept-Encoding", "identity")
	req.Header.Set("User-Agent", "flashdata-go/"+Version)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if auth.apiKey {
		if c.key == "" {
			cancel()
			return nil, nil, &ValidationError{Field: "apiKey", Message: "pass APIKey explicitly to New"}
		}
		req.Header.Set("X-API-Key", c.key)
	} else if auth.bearer != "" {
		req.Header.Set("Authorization", "Bearer "+auth.bearer)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		cause := ctx.Err()
		cancel()
		if cause == nil {
			cause = transportCause(err)
		}
		if mutation {
			return nil, nil, &SubmissionUnknownError{Cause: cause}
		}
		return nil, nil, cause
	}
	return resp, cancel, nil
}

func (c *Client) read(resp *http.Response) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(resp.Body, c.maxBytes+1))
	if err != nil {
		if cause := transportCause(err); cause != ErrTransport {
			return nil, cause
		}
		return nil, &ProtocolError{"could not read complete response"}
	}
	if int64(len(b)) > c.maxBytes {
		return nil, ErrResponseTooLarge
	}
	if !utf8.Valid(b) {
		return nil, &ProtocolError{"response is not UTF-8"}
	}
	return b, nil
}

func transportCause(err error) error {
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}
	var timed interface{ Timeout() bool }
	if errors.As(err, &timed) && timed.Timeout() {
		return context.DeadlineExceeded
	}
	return ErrTransport
}

func (c *Client) apiError(resp *http.Response, b []byte, secrets ...string) error {
	var p struct {
		Code    string `json:"code"`
		Detail  string `json:"detail"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal(b, &p)
	if p.Detail == "" {
		p.Detail = p.Message
	}
	if p.Detail == "" {
		p.Detail = http.StatusText(resp.StatusCode)
	}
	clean := func(s string) string {
		for _, v := range append(secrets, c.key) {
			if v != "" {
				s = strings.ReplaceAll(s, v, "[redacted]")
			}
		}
		return safeText(s)
	}
	return &APIError{StatusCode: resp.StatusCode, Code: clean(p.Code), Message: clean(p.Detail), RequestID: clean(resp.Header.Get("X-Request-ID")), RetryAfter: clean(resp.Header.Get("Retry-After"))}
}

func (c *Client) request(ctx context.Context, method, path string, body any, out any, data, public bool, bearer string) error {
	return c.requestWithFormat(ctx, method, path, body, out, data, public, bearer, ContentOptions{})
}

func (c *Client) requestWithFormat(ctx context.Context, method, path string, body any, out any, data, public bool, bearer string, format ContentOptions) error {
	base := c.managementOrigin + "/v2"
	if data {
		base = c.dataOrigin
	}
	auth := requestAuth{apiKey: true}
	if public {
		auth = requestAuth{}
	}
	if bearer != "" {
		auth = requestAuth{bearer: bearer}
	}
	var reader io.Reader
	size := int64(0)
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return &ValidationError{Field: "body", Message: "must be JSON serializable"}
		}
		reader = bytes.NewReader(b)
		size = int64(len(b))
	}
	mutation := method != "GET" && method != "HEAD"
	resp, cancel, err := c.rawWithFormat(ctx, method, base+path, reader, size, "application/json", auth, mutation, format)
	if err != nil {
		return err
	}
	defer cancel()
	defer resp.Body.Close()
	formatted, isFormatted := out.(*FormattedResponse)
	var b []byte
	if isFormatted {
		b, err = c.readFormatted(ctx, resp)
	} else {
		b, err = c.read(resp)
	}
	if err != nil {
		if mutation && resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return &SubmissionUnknownError{Cause: err}
		}
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.apiError(resp, b, append(secretValues(body), bearer)...)
	}
	if out == nil {
		return nil
	}
	if isFormatted {
		expected := string(format.Accept)
		if expected == "" {
			expected = string(AcceptJSON)
		}
		media, params, parseErr := mime.ParseMediaType(resp.Header.Get("Content-Type"))
		if parseErr != nil || media != expected || (params["charset"] != "" && !strings.EqualFold(params["charset"], "utf-8")) {
			err = &ProtocolError{"unexpected response content type"}
		} else {
			err = formatted.setBody(b, format.Accept)
		}
		if err != nil && mutation {
			return &SubmissionUnknownError{Cause: err}
		}
		return err
	}
	if len(b) == 0 || b[0] == '[' || strings.TrimSpace(string(b)) == "null" {
		err = fmt.Errorf("not an object")
	} else {
		err = json.Unmarshal(b, out)
	}
	if err != nil {
		if mutation {
			return &SubmissionUnknownError{Cause: &ProtocolError{"invalid JSON response"}}
		}
		return &ProtocolError{"invalid JSON response"}
	}
	return nil
}

func (c *Client) trustedResultOrigin(ctx context.Context) (string, error) {
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		c.resultMu.Lock()
		if c.resultOrigin != "" {
			o := c.resultOrigin
			c.resultMu.Unlock()
			return o, nil
		}
		if pending := c.resultLoading; pending != nil {
			c.resultMu.Unlock()
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-pending:
				continue
			}
		}
		c.resultLoading = make(chan struct{})
		c.resultMu.Unlock()
		a, err := c.Account.Access(ctx)
		o := ""
		if err == nil {
			if a.ResultOrigin == nil || *a.ResultOrigin == "" {
				err = &ProtocolError{"server did not provide a signed-result origin"}
			} else {
				o, err = origin(*a.ResultOrigin)
				if err != nil {
					err = &ProtocolError{"server provided an invalid result origin"}
				}
			}
		}
		c.resultMu.Lock()
		if err == nil {
			c.resultOrigin = o
		}
		close(c.resultLoading)
		c.resultLoading = nil
		c.resultMu.Unlock()
		return o, err
	}
}

// resultStream authenticates only the management hop, then checks the exact
// server-approved storage origin before sending an unsigned request there.
func (c *Client) resultStream(ctx context.Context, path string) (*cancelReader, error) {
	resp, cancel, err := c.raw(ctx, "GET", c.managementOrigin+"/v2"+path, nil, 0, "", requestAuth{apiKey: true}, false)
	if err != nil {
		return nil, err
	}
	var secrets []string
	if resp.StatusCode == http.StatusFound {
		location := resp.Header.Get("Location")
		resp.Body.Close()
		cancel()
		u, parseErr := url.Parse(location)
		if parseErr != nil || u.User != nil || u.Fragment != "" {
			return nil, &ProtocolError{"invalid result redirect"}
		}
		trusted, err := c.trustedResultOrigin(ctx)
		if err != nil {
			return nil, err
		}
		o, err := origin(u.Scheme + "://" + u.Host)
		if err != nil || o != trusted {
			return nil, &ProtocolError{"untrusted result redirect origin"}
		}
		secrets = signedURLSecrets(u)
		resp, cancel, err = c.raw(ctx, "GET", location, nil, 0, "", requestAuth{}, false)
		if err != nil {
			return nil, err
		}
	}
	if resp.StatusCode != http.StatusOK {
		defer cancel()
		defer resp.Body.Close()
		b, err := c.readFormatted(ctx, resp)
		if err != nil {
			return nil, err
		}
		return nil, c.apiError(resp, b, secrets...)
	}
	return &cancelReader{ReadCloser: resp.Body, cancel: cancel, response: resp}, nil
}

type cancelReader struct {
	io.ReadCloser
	cancel   context.CancelFunc
	response *http.Response
}

func (r *cancelReader) Close() error { err := r.ReadCloser.Close(); r.cancel(); return err }

func (c *Client) resultJSON(ctx context.Context, path string, out any) error {
	r, err := c.resultStream(ctx, path)
	if err != nil {
		return err
	}
	defer r.Close()
	b, err := c.readFormatted(ctx, r.response)
	if err != nil {
		return err
	}
	if err = json.Unmarshal(b, out); err != nil || strings.TrimSpace(string(b)) == "null" {
		return &ProtocolError{"invalid result JSON"}
	}
	return nil
}
