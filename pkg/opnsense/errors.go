// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package opnsense

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
)

// ValidationError is returned when a mutation response explicitly reports
// result="failed". OPNsense returns HTTP 200 on validation errors, while
// missing or unknown results remain ambiguous ordinary errors for recovery.
type ValidationError struct {
	Fields map[string]string // field name → error message
}

func (e *ValidationError) Error() string {
	if len(e.Fields) == 0 {
		return "validation failed"
	}
	// Sort keys for deterministic output.
	keys := make([]string, 0, len(e.Fields))
	for k := range e.Fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s: %s", k, e.Fields[k]))
	}
	return "validation failed: " + strings.Join(parts, "; ")
}

// NotFoundError is returned when a resource is not found (deleted out-of-band).
type NotFoundError struct {
	Message string
}

func (e *NotFoundError) Error() string { return e.Message }

// AuthError is returned for HTTP 401 or 403 responses.
type AuthError struct {
	StatusCode int
}

func (e *AuthError) Error() string {
	if e.StatusCode == http.StatusForbidden {
		return fmt.Sprintf("authorization failed (HTTP %d)", e.StatusCode)
	}
	return fmt.Sprintf("authentication failed (HTTP %d)", e.StatusCode)
}

// ServerError wraps transport/server failures after retries are exhausted.
type ServerError struct {
	Message string
	Cause   error
}

func (e *ServerError) Error() string { return e.Message }
func (e *ServerError) Unwrap() error { return e.Cause }

// MutationReconfigureError is returned when OPNsense accepted and saved a
// mutation, but the follow-up reconfigure/apply step failed.
type MutationReconfigureError struct {
	Operation  string
	Endpoint   string
	ResourceID string
	Cause      error
}

func (e *MutationReconfigureError) Error() string {
	if e.ResourceID == "" {
		return fmt.Sprintf("%s %s: mutation saved but reconfigure failed: %s", e.Operation, e.Endpoint, e.Cause)
	}
	return fmt.Sprintf("%s %s: mutation saved for %s but reconfigure failed: %s", e.Operation, e.Endpoint, e.ResourceID, e.Cause)
}

func (e *MutationReconfigureError) Unwrap() error { return e.Cause }

// PluginNotFoundError is returned for HTTP 404 on plugin API endpoints.
type PluginNotFoundError struct {
	PluginName string
}

func (e *PluginNotFoundError) Error() string {
	return fmt.Sprintf("plugin '%s' is not installed on OPNsense", e.PluginName)
}

// NewServerError creates a ServerError wrapping a transport or server failure.
func NewServerError(endpoint string, cause error) *ServerError {
	return &ServerError{
		Message: fmt.Sprintf("request to %s failed: %s", endpoint, cause),
		Cause:   cause,
	}
}

// mutationResponse is the JSON structure returned by OPNsense mutation endpoints.
type mutationResponse struct {
	Result      string            `json:"result"`
	UUID        string            `json:"uuid"`
	Validations map[string]string `json:"validations"`
}

// ParseMutationResponse parses a mutation API response body. Only an explicit
// failed result is a deterministic validation rejection. Missing or unknown
// results are ambiguous mutation outcomes so callers can run authoritative
// recovery instead of assuming that the mutation did not commit.
func ParseMutationResponse(body []byte) (string, error) {
	var resp mutationResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("failed to parse mutation response: %w", err)
	}
	switch resp.Result {
	case "saved":
		return resp.UUID, nil
	case "failed":
		return "", &ValidationError{Fields: resp.Validations}
	default:
		return "", fmt.Errorf("ambiguous mutation response: expected result %q or %q, got %q", "saved", "failed", resp.Result)
	}
}

// ParseAddResponse requires both an affirmative save and a recoverable UUID.
func ParseAddResponse(body []byte) (string, error) {
	uuid, err := ParseMutationResponse(body)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(uuid) == "" {
		return "", fmt.Errorf("mutation response reported saved but did not include a non-empty UUID")
	}
	return uuid, nil
}

// ParseDeleteResponse parses a delete API response body. OPNsense delete
// endpoints commonly return result="deleted" instead of result="saved", but
// failed delete attempts still use result="failed" with validations.
func ParseDeleteResponse(body []byte) error {
	if len(strings.TrimSpace(string(body))) == 0 {
		return nil
	}
	var resp mutationResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("failed to parse delete response: %w", err)
	}
	if resp.Result == "" || resp.Result == "deleted" || resp.Result == "saved" {
		return nil
	}
	return &ValidationError{Fields: resp.Validations}
}

// ParseStrictDeleteResponse requires source-backed affirmative deletion.
func ParseStrictDeleteResponse(body []byte) error {
	var resp mutationResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("failed to parse delete response: %w", err)
	}
	switch resp.Result {
	case "deleted", "saved":
		return nil
	case "failed":
		return &ValidationError{Fields: resp.Validations}
	default:
		return fmt.Errorf("delete response did not affirm success: unexpected result %q", resp.Result)
	}
}

// CheckHTTPError returns an appropriate error type for non-success HTTP status codes.
// Returns nil for every 2xx response. Called before body parsing.
func CheckHTTPError(statusCode int, endpoint string) error {
	if statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices {
		return nil
	}
	switch statusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return &AuthError{StatusCode: statusCode}
	case http.StatusNotFound:
		return &PluginNotFoundError{PluginName: extractPluginName(endpoint)}
	default:
		return NewServerError(endpoint, fmt.Errorf("HTTP %d", statusCode))
	}
}

// extractPluginName extracts the plugin/module name from an OPNsense API path.
// Example: "/api/haproxy/settings/getServer" → "haproxy"
func extractPluginName(endpoint string) string {
	trimmed := strings.TrimPrefix(endpoint, "/api/")
	if idx := strings.Index(trimmed, "/"); idx > 0 {
		return trimmed[:idx]
	}
	return trimmed
}
