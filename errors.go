package flashdata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var ErrTransport = errors.New("flashdata: HTTP transport failed")
var ErrResponseTooLarge = errors.New("flashdata: response exceeds configured size limit")

type ValidationError struct{ Field, Message string }

func (e *ValidationError) Error() string { return "flashdata: invalid " + e.Field + ": " + e.Message }

type ProtocolError struct{ Message string }

func (e *ProtocolError) Error() string { return "flashdata: " + e.Message }

// APIError is an HTTP rejection; RetryAfter is the original header, never an
// instruction to retry automatically. Message and request metadata are redacted.
type APIError struct {
	StatusCode                           int
	Code, Message, RequestID, RetryAfter string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("flashdata: HTTP %d %s: %s", e.StatusCode, e.Code, e.Message)
}

// SubmissionUnknownError means the server may have accepted a mutation. Do not
// blindly resubmit. Reconcile via a saved ID or the same onboarding proof.
type SubmissionUnknownError struct {
	Cause error
	// JobIDs contains valid job identifiers recovered from an inconsistent
	// response. Preserve and reconcile them; their presence is not confirmation
	// that every submitted operation was accepted.
	JobIDs []string
}

func (e *SubmissionUnknownError) Error() string {
	return "flashdata: submission outcome unknown; reconcile before retrying"
}
func (e *SubmissionUnknownError) Unwrap() error { return e.Cause }

// A decoder may recover a valid ID even when another field has the wrong wire
// type. Keep that evidence without treating a malformed response as acceptance.
func withRecoveryIDs(err error, ids []string) error {
	var unknown *SubmissionUnknownError
	if len(ids) == 0 || !errors.As(err, &unknown) {
		return err
	}
	copy := *unknown
	copy.JobIDs = append([]string(nil), ids...)
	return &copy
}

type PollingTimeoutError struct{ JobID string }

func (e *PollingTimeoutError) Error() string {
	return "flashdata: polling deadline exceeded; job " + e.JobID + " may continue"
}
func (e *PollingTimeoutError) Unwrap() error { return context.DeadlineExceeded }

type JobFailedError struct{ JobID, Status string }

func (e *JobFailedError) Error() string { return "flashdata: job " + e.JobID + " is " + e.Status }

type JobPausedError struct{ JobID string }

func (e *JobPausedError) Error() string {
	return "flashdata: bulk job " + e.JobID + " is paused; inspect it before resuming"
}

var secretURL = regexp.MustCompile(`https?://[^\s<>"']+`)
var secretKey = regexp.MustCompile(`(?i)\bfd_[a-z0-9_-]+`)
var secretField = regexp.MustCompile(`(?i)(access_key|secret_key|api_key|apiKey|sas_token|private_key|setupCredential|handoffProof|registrationProof)["'\s:=]+[^\s,;}]+`)

func safeText(s string) string {
	s = secretURL.ReplaceAllString(s, "[redacted URL]")
	s = secretKey.ReplaceAllString(s, "[redacted key]")
	s = secretField.ReplaceAllString(s, "[redacted credential]")
	if len(s) > 1024 {
		s = s[:1024]
	}
	return s
}

// Storage services can echo a bare signature instead of the complete signed
// URL. Redact both decoded and escaped credential values from their errors.
func signedURLSecrets(u *url.URL) []string {
	var secrets []string
	for key, values := range u.Query() {
		name := strings.ToLower(key)
		if name != "sig" && !strings.Contains(name, "signature") && !strings.Contains(name, "credential") && !strings.Contains(name, "token") && !strings.Contains(name, "secret") && !strings.Contains(name, "key") {
			continue
		}
		for _, value := range values {
			if value != "" {
				secrets = append(secrets, value, url.QueryEscape(value), url.PathEscape(value))
			}
		}
	}
	return secrets
}

func secretValues(body any) []string {
	b, err := json.Marshal(body)
	if err != nil {
		return nil
	}
	var v any
	if json.Unmarshal(b, &v) != nil {
		return nil
	}
	var values []string
	var visit func(any)
	visit = func(v any) {
		switch v := v.(type) {
		case map[string]any:
			for k, x := range v {
				switch strings.ToLower(strings.ReplaceAll(k, "_", "")) {
				case "apikey", "accesskey", "secretkey", "privatekey", "sastoken", "authorization", "setupcredential", "handoffproof", "registrationproof":
					if s, ok := x.(string); ok {
						values = append(values, s)
					}
				}
				visit(x)
			}
		case []any:
			for _, x := range v {
				visit(x)
			}
		}
	}
	visit(v)
	return values
}
