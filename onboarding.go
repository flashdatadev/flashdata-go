package flashdata

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type OnboardingService struct{ c *Client }

// APIKeyIdentity is verified from current server records. RecurringAllowed is
// false for temporary keys and owned connections awaiting installation ACK.
// This endpoint is added by the accompanying local backend patch; an older
// deployment can return 404. Callers must fail closed, never infer ownership.
type APIKeyIdentity struct {
	APIKeyID         string   `json:"apiKeyId"`
	AccountID        string   `json:"accountId"`
	AccountType      string   `json:"accountType"`
	Ownership        string   `json:"ownership"`
	Access           string   `json:"access"`
	Installed        bool     `json:"installed"`
	RecurringAllowed bool     `json:"recurringAllowed"`
	ConnectionID     *string  `json:"connectionId"`
	Scopes           []Source `json:"scopes"`
	ExpiresAt        *string  `json:"expiresAt"`
	VerifiedAt       string   `json:"verifiedAt"`
}

func (r *AccountService) Identity(ctx context.Context) (*APIKeyIdentity, error) {
	var v APIKeyIdentity
	if err := r.c.request(ctx, "GET", "/api-access/identity", nil, &v, false, false, ""); err != nil {
		return nil, err
	}
	owned := v.AccountType == "owned" && v.Ownership == "owned"
	temporary := v.AccountType == "temporary" && v.Ownership == "unowned" && !v.Installed
	if v.APIKeyID == "" || v.AccountID == "" || !validTimestamp(v.VerifiedAt) || (!owned && !temporary) || (v.Access != "ready" && v.Access != "blocked") || (v.Access == "ready" && len(v.Scopes) == 0) || (v.ExpiresAt != nil && !validTimestamp(*v.ExpiresAt)) || (v.ConnectionID != nil && !connectionPattern.MatchString(*v.ConnectionID)) || (temporary && v.ConnectionID == nil) || (v.RecurringAllowed && (!owned || v.Access != "ready" || !v.Installed)) {
		return nil, &ProtocolError{"incomplete or inconsistent verified key identity"}
	}
	return &v, nil
}

type OnboardingCapabilities struct {
	ProtocolVersion   int             `json:"protocolVersion"`
	Available         bool            `json:"available"`
	Flow              string          `json:"flow"`
	DataOrigin        string          `json:"dataOrigin"`
	QueryPath         string          `json:"queryPath"`
	CredentialType    string          `json:"credentialType"`
	Trial             OnboardingTrial `json:"trial"`
	RecurringRequires string          `json:"recurringRequires"`
	ReplayProtection  string          `json:"replayProtection"`
}
type OnboardingTrial struct {
	Credits           float64         `json:"credits"`
	LifetimeHours     int             `json:"lifetimeHours"`
	RequestsPerSecond int             `json:"requestsPerSecond"`
	MaxBatchSize      int             `json:"maxBatchSize"`
	RequestCounts     json.RawMessage `json:"requestCounts"`
}
type AgentConnection struct {
	ID                 string          `json:"id"`
	Name               string          `json:"name"`
	Audience           string          `json:"audience"`
	Ownership          string          `json:"ownership"`
	Lifecycle          string          `json:"lifecycle"`
	Access             string          `json:"access"`
	Scopes             []Source        `json:"scopes"`
	Generation         int             `json:"generation"`
	Installed          bool            `json:"installed"`
	TrialExpiresAt     string          `json:"trialExpiresAt"`
	FundingMode        string          `json:"fundingMode"`
	TrialCredits       float64         `json:"trialCredits"`
	TrialRequestCounts json.RawMessage `json:"trialRequestCounts"`
}
type AgentCredentials struct {
	ConnectionID    string `json:"connectionId"`
	APIKey          string `json:"apiKey"`
	SetupCredential string `json:"setupCredential,omitempty"`
	HandoffProof    string `json:"handoffProof,omitempty"`
	Generation      int    `json:"generation"`
}

func (v AgentCredentials) String() string   { return "flashdata.AgentCredentials{redacted}" }
func (v AgentCredentials) GoString() string { return v.String() }

type RegistrationRequest struct {
	Name      string   `json:"name"`
	Audience  string   `json:"audience,omitempty"`
	Scopes    []Source `json:"scopes"`
	Client    string   `json:"client,omitempty"`
	Transport string   `json:"transport,omitempty"`
}
type Registration struct {
	Connection   AgentConnection            `json:"connection"`
	Credentials  AgentCredentials           `json:"credentials"`
	DataOrigin   string                     `json:"dataOrigin"`
	DeniedScopes []Source                   `json:"deniedScopes"`
	Limits       map[string]json.RawMessage `json:"limits"`
}

func (v Registration) String() string   { return "flashdata.Registration{credentials:redacted}" }
func (v Registration) GoString() string { return v.String() }

type AgentClaim struct {
	ClaimID          string `json:"claimId"`
	ClaimPath        string `json:"claimPath"`
	ConfirmationCode string `json:"confirmationCode"`
	ExpiresAt        string `json:"expiresAt"`
	PollAfterSeconds int    `json:"pollAfterSeconds"`
}
type OnboardingStatus struct {
	Status           string           `json:"status"`
	Connection       *AgentConnection `json:"connection,omitempty"`
	NextAction       string           `json:"nextAction,omitempty"`
	ExpiresAt        string           `json:"expiresAt,omitempty"`
	PollAfterSeconds int              `json:"pollAfterSeconds,omitempty"`
}
type Handoff struct {
	Status      string            `json:"status"`
	Connection  *AgentConnection  `json:"connection,omitempty"`
	Credentials *AgentCredentials `json:"credentials,omitempty"`
	Generation  int               `json:"generation,omitempty"`
}

func (v Handoff) String() string   { return "flashdata.Handoff{credentials:redacted}" }
func (v Handoff) GoString() string { return v.String() }

// NewRegistrationProof generates 256 bits of randomness. The caller MUST save
// the proof and exact RegistrationRequest durably BEFORE calling Register.
func NewRegistrationProof() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}

var connectionPattern = regexp.MustCompile(`^acn_[A-Za-z0-9_-]{8,100}$`)
var claimPattern = regexp.MustCompile(`^acl_[A-Za-z0-9_-]{8,100}$`)
var clientPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)
var scopePattern = regexp.MustCompile(`^[a-z_]{1,80}$`)

func validTimestamp(value string) bool {
	_, err := time.Parse(time.RFC3339Nano, value)
	return err == nil
}

func validConnection(v *AgentConnection, id string) bool {
	if v == nil || v.ID != id || v.Generation < 1 || (v.Ownership != "owned" && v.Ownership != "unowned") || (v.Lifecycle != "valid" && v.Lifecycle != "expired" && v.Lifecycle != "revoked") || (v.Access != "ready" && v.Access != "blocked" && v.Access != "fenced") {
		return false
	}
	if (v.Ownership == "unowned" && v.Installed) || (v.Ownership == "owned" && v.Generation < 2) || (v.TrialExpiresAt != "" && !validTimestamp(v.TrialExpiresAt)) {
		return false
	}
	return true
}

func (r *OnboardingService) call(ctx context.Context, method, id, suffix, proof string, body, out any) error {
	if !connectionPattern.MatchString(id) {
		return &ValidationError{Field: "connectionId", Message: "invalid connection identifier"}
	}
	if !credentialPattern.MatchString(proof) {
		return &ValidationError{Field: "credential", Message: "pass the credential required by this operation"}
	}
	return r.c.request(ctx, method, "/agent-connections/"+id+suffix, body, out, false, false, proof)
}
func (r *OnboardingService) Capabilities(ctx context.Context) (*OnboardingCapabilities, error) {
	var v OnboardingCapabilities
	if err := r.c.request(ctx, "GET", "/agent-connections/capabilities", nil, &v, false, true, ""); err != nil {
		return nil, err
	}
	if v.ProtocolVersion != 2 {
		return nil, &ProtocolError{"unsupported agent onboarding protocol version"}
	}
	return &v, nil
}

// ValidateRegistration checks onboarding input without creating a trial.
func ValidateRegistration(input RegistrationRequest) error {
	name := strings.TrimSpace(input.Name)
	if name == "" || !utf8.ValidString(name) || utf8.RuneCountInString(name) > 80 || strings.ContainsFunc(name, func(c rune) bool { return unicode.IsControl(c) || unicode.In(c, unicode.Cf) }) {
		return &ValidationError{Field: "name", Message: "use 1 through 80 printable characters"}
	}
	if input.Audience != "" && input.Audience != "coding" && input.Audience != "personal" {
		return &ValidationError{Field: "audience", Message: "choose coding or personal"}
	}
	if input.Client != "" && !clientPattern.MatchString(input.Client) {
		return &ValidationError{Field: "client", Message: "use a lowercase identifier of at most 64 characters"}
	}
	if input.Transport != "" && input.Transport != "api" && input.Transport != "cli" && input.Transport != "mcp" {
		return &ValidationError{Field: "transport", Message: "choose api, cli, or mcp"}
	}
	seen := map[Source]bool{}
	supported := false
	if len(input.Scopes) == 0 || len(input.Scopes) > 30 {
		return &ValidationError{Field: "scopes", Message: "provide 1 through 30 unique source names"}
	}
	for _, s := range input.Scopes {
		if !scopePattern.MatchString(string(s)) || seen[s] {
			return &ValidationError{Field: "scopes", Message: "provide unique valid source names"}
		}
		seen[s] = true
		switch s {
		case SourceGoogleSearch, SourceYouTubeSearch, SourceYouTubeMetadata, SourceYouTubeTranscript, SourceYouTubeCaptions:
			supported = true
		}
	}
	if !supported {
		return &ValidationError{Field: "scopes", Message: "request at least one supported trial source"}
	}
	return nil
}

// Register has no automatic retry. On uncertain outcomes, recover with the
// SAME persisted proof and identical request; a different proof creates a trial.
func (r *OnboardingService) Register(ctx context.Context, proof string, input RegistrationRequest) (*Registration, error) {
	if len(proof) != 43 || !credentialPattern.MatchString(proof) {
		return nil, &ValidationError{Field: "registrationProof", Message: "use NewRegistrationProof and save it before registration"}
	}
	if err := ValidateRegistration(input); err != nil {
		return nil, err
	}
	var v Registration
	if err := r.c.request(ctx, "POST", "/agent-connections/registrations", input, &v, false, false, proof); err != nil {
		return nil, err
	}
	if !connectionPattern.MatchString(v.Connection.ID) || !validConnection(&v.Connection, v.Connection.ID) || v.Connection.Ownership != "unowned" || v.Connection.Lifecycle != "valid" || v.Connection.Access != "ready" || v.Connection.Generation != 1 || v.Credentials.ConnectionID != v.Connection.ID || !credentialPattern.MatchString(v.Credentials.APIKey) || !credentialPattern.MatchString(v.Credentials.SetupCredential) || !credentialPattern.MatchString(v.Credentials.HandoffProof) || v.Credentials.Generation != 1 {
		return nil, &SubmissionUnknownError{Cause: &ProtocolError{"incomplete registration credentials; recover using the same proof"}}
	}
	return &v, nil
}
func (r *OnboardingService) connection(ctx context.Context, id, suffix, credential string) (*AgentConnection, error) {
	var v struct {
		Connection *AgentConnection `json:"connection"`
	}
	if err := r.call(ctx, "GET", id, suffix, credential, nil, &v); err != nil {
		return nil, err
	}
	if !validConnection(v.Connection, id) {
		return nil, &ProtocolError{"expected a matching connection object"}
	}
	return v.Connection, nil
}
func (r *OnboardingService) TemporaryStatus(ctx context.Context, id, setupCredential string) (*AgentConnection, error) {
	return r.connection(ctx, id, "", setupCredential)
}
func (r *OnboardingService) Identity(ctx context.Context, id, apiKey string) (*AgentConnection, error) {
	return r.connection(ctx, id, "/identity", apiKey)
}
func (r *OnboardingService) Status(ctx context.Context, id, handoffProof string) (*OnboardingStatus, error) {
	var v OnboardingStatus
	err := r.call(ctx, "GET", id, "/onboarding", handoffProof, nil, &v)
	if err == nil && (v.Status == "" || (v.Connection != nil && !validConnection(v.Connection, id)) || v.PollAfterSeconds < 0 || (v.ExpiresAt != "" && !validTimestamp(v.ExpiresAt))) {
		err = &ProtocolError{"invalid onboarding status"}
	}
	return &v, err
}
func (r *OnboardingService) CreateClaim(ctx context.Context, id, setupCredential string) (*AgentClaim, error) {
	var v AgentClaim
	if err := r.call(ctx, "POST", id, "/claims", setupCredential, struct{}{}, &v); err != nil {
		return nil, err
	}
	if !claimPattern.MatchString(v.ClaimID) || v.ClaimPath != "/connect/claim/"+v.ClaimID || !regexp.MustCompile(`^\d{8}$`).MatchString(v.ConfirmationCode) || !validTimestamp(v.ExpiresAt) || v.PollAfterSeconds < 0 {
		return nil, &SubmissionUnknownError{Cause: &ProtocolError{"incomplete claim response"}}
	}
	return &v, nil
}

// ClaimURL uses a caller-trusted web origin and a validated path. Codes and
// proofs are never put in browser URLs. Show the confirmation code separately.
func ClaimURL(webOrigin string, claim *AgentClaim) (string, error) {
	o, err := origin(webOrigin)
	if err != nil {
		return "", err
	}
	if claim == nil || !claimPattern.MatchString(claim.ClaimID) || claim.ClaimPath != "/connect/claim/"+claim.ClaimID {
		return "", &ValidationError{Field: "claim", Message: "invalid browser claim path"}
	}
	u, _ := url.Parse(o)
	u.Path = claim.ClaimPath
	return u.String(), nil
}
func (r *OnboardingService) Handoff(ctx context.Context, id, handoffProof string) (*Handoff, error) {
	var v Handoff
	if err := r.call(ctx, "POST", id, "/handoff", handoffProof, struct{}{}, &v); err != nil {
		return nil, err
	}
	confirmed := v.Status == "confirmed" && v.Generation >= 2 && v.Credentials == nil
	delivered := v.Status == "delivered" && v.Credentials != nil && v.Credentials.ConnectionID == id && v.Credentials.Generation >= 2 && credentialPattern.MatchString(v.Credentials.APIKey)
	if (!confirmed && !delivered) || (v.Connection != nil && (!validConnection(v.Connection, id) || v.Connection.Ownership != "owned" || (delivered && v.Connection.Generation != v.Credentials.Generation) || (confirmed && v.Connection.Generation != v.Generation))) {
		return nil, &SubmissionUnknownError{Cause: &ProtocolError{"invalid handoff response"}}
	}
	return &v, nil
}

// Acknowledge confirms installation. The caller must first atomically store the
// candidate key, reload it in a fresh process, and verify Onboarding.Identity.
// This low-level method cannot attest that the caller actually performed this.
func (r *OnboardingService) Acknowledge(ctx context.Context, id, handoffProof, installedAPIKey string) (*Handoff, error) {
	if !credentialPattern.MatchString(installedAPIKey) {
		return nil, &ValidationError{Field: "apiKey", Message: "pass the installed candidate key"}
	}
	var v Handoff
	if err := r.call(ctx, "POST", id, "/handoff/ack", handoffProof, map[string]string{"apiKey": installedAPIKey}, &v); err != nil {
		return nil, err
	}
	if v.Status != "confirmed" || v.Generation < 2 || v.Credentials != nil || (v.Connection != nil && (!validConnection(v.Connection, id) || v.Connection.Ownership != "owned" || v.Connection.Generation != v.Generation)) {
		return nil, &SubmissionUnknownError{Cause: &ProtocolError{"invalid installation acknowledgement"}}
	}
	return &v, nil
}
