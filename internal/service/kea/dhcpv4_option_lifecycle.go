// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package kea

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/matthew-on-git/terraform-provider-opnsense/pkg/opnsense"
)

var dhcpv4OptionReqOpts = opnsense.ReqOpts{
	AddEndpoint: "/api/kea/dhcpv4/add_option", GetEndpoint: "/api/kea/dhcpv4/get_option", UpdateEndpoint: "/api/kea/dhcpv4/set_option", DeleteEndpoint: "/api/kea/dhcpv4/del_option", SearchEndpoint: "/api/kea/dhcpv4/search_option", ReconfigureEndpoint: "/api/kea/service/reconfigure", Monad: "option", StrictMissingObjectResponse: true, StrictDeleteResponse: true, StrictReconfigureResponse: true,
}

func optionRequest(ctx context.Context, c *opnsense.Client, method, endpoint string, payload any) ([]byte, error) {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL()+endpoint, body)
	if err != nil {
		return nil, err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	transport, ok := c.HTTPClient().Transport.(*retryablehttp.RoundTripper)
	if !ok || transport.Client == nil || transport.Client.HTTPClient == nil {
		return nil, errors.New("generic Kea DHCPv4 option requires the authenticated single-attempt transport")
	}
	// Bypass shared automatic retries for non-idempotent option writes. Copy the
	// underlying authenticated client without changing any shared client settings.
	singleAttempt := *transport.Client.HTTPClient
	singleAttempt.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	requester := &singleAttempt
	if method == http.MethodGet || endpoint == dhcpv4OptionReqOpts.SearchEndpoint {
		// Copy retry policy and transports; never mutate the shared client's settings.
		configured := transport.Client
		retryPolicy := &retryablehttp.Client{HTTPClient: &singleAttempt, Logger: configured.Logger, RetryWaitMin: configured.RetryWaitMin, RetryWaitMax: configured.RetryWaitMax, RetryMax: configured.RetryMax, RequestLogHook: configured.RequestLogHook, ResponseLogHook: configured.ResponseLogHook, CheckRetry: configured.CheckRetry, Backoff: configured.Backoff, ErrorHandler: configured.ErrorHandler, PrepareRetry: configured.PrepareRetry}
		requester = retryPolicy.StandardClient()
		requester.CheckRedirect = singleAttempt.CheckRedirect
	}
	resp, err := requester.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if err := opnsense.CheckHTTPError(resp.StatusCode, endpoint); err != nil {
		return nil, err
	}
	return io.ReadAll(resp.Body)
}

var optionStableVersion = regexp.MustCompile(`^([0-9]+)\.([0-9]+)(?:\.([0-9]+))?(?:_[0-9]+)?$`)

func requireDHCPv4Option(ctx context.Context, c *opnsense.Client) error {
	if c == nil {
		return errors.New("generic Kea DHCPv4 option client is not configured")
	}
	if err := c.AcquireRead(ctx); err != nil {
		return err
	}
	defer c.ReleaseRead()
	raw, err := optionRequest(ctx, c, http.MethodGet, "/api/core/firmware/status", nil)
	if err != nil {
		return fmt.Errorf("verify Kea DHCPv4 option firmware: %w", err)
	}
	var status struct {
		Product struct {
			Name    string `json:"product_name"`
			ID      string `json:"product_id"`
			Version string `json:"product_version"`
		} `json:"product"`
	}
	if json.Unmarshal(raw, &status) != nil || (status.Product.Name != "OPNsense" && status.Product.ID != "opnsense") {
		return errors.New("generic Kea DHCPv4 options require a verified OPNsense firmware identity")
	}
	parts := optionStableVersion.FindStringSubmatch(status.Product.Version)
	if parts == nil {
		return errors.New("generic Kea DHCPv4 options require a known stable OPNsense version, minimum 26.7.7")
	}
	version := [3]uint64{}
	for i := range version {
		if parts[i+1] != "" {
			version[i], err = strconv.ParseUint(parts[i+1], 10, 63)
			if err != nil {
				return err
			}
		}
	}
	minimum := [3]uint64{26, 7, 7}
	supported := true
	for i := range version {
		if version[i] != minimum[i] {
			supported = version[i] > minimum[i]
			break
		}
	}
	if !supported {
		return errors.New("generic Kea DHCPv4 options require stable OPNsense 26.7.7 or newer")
	}
	raw, err = optionRequest(ctx, c, http.MethodGet, dhcpv4OptionReqOpts.GetEndpoint, nil)
	if err != nil {
		return fmt.Errorf("verify Kea DHCPv4 option model: %w", err)
	}
	_, err = parseOptionTemplate(raw)
	return err
}

func parseOptionTemplate(raw []byte) (map[string]json.RawMessage, error) {
	var envelope map[string]map[string]json.RawMessage
	if json.Unmarshal(raw, &envelope) != nil || envelope["option"] == nil {
		return nil, errors.New("generic Kea option model unavailable or malformed")
	}
	fields := envelope["option"]
	if _, err := decodeOption(fields, "", true); err != nil {
		return nil, fmt.Errorf("incompatible Kea option model: %w", err)
	}
	for _, field := range []string{"code", "match_code", "encoding", "match_encoding"} {
		_, options, err := optionSelection(fields[field], field, true)
		if err != nil {
			return nil, err
		}
		if field == "code" || field == "match_code" {
			eligible := 0
			for key := range options {
				if optionCodeValid(key) {
					eligible++
				}
			}
			if eligible == 0 {
				return nil, fmt.Errorf("empty option catalog")
			}
		} else {
			for _, key := range optionEncodings {
				if _, ok := options[key]; !ok {
					return nil, fmt.Errorf("incomplete %s catalog", field)
				}
			}
		}
	}
	return fields, nil
}

func readDHCPv4Option(ctx context.Context, c *opnsense.Client, id string) (DHCPv4OptionResourceModel, error) {
	if err := c.AcquireRead(ctx); err != nil {
		return DHCPv4OptionResourceModel{}, err
	}
	defer c.ReleaseRead()
	raw, err := optionRequest(ctx, c, http.MethodGet, dhcpv4OptionReqOpts.GetEndpoint+"/"+id, nil)
	if err != nil {
		return DHCPv4OptionResourceModel{}, err
	}
	// Only the successful controller's actual [] response establishes absence.
	if strings.TrimSpace(string(raw)) == "[]" {
		return DHCPv4OptionResourceModel{}, &opnsense.NotFoundError{Message: "Kea option UUID not found"}
	}
	var envelope map[string]json.RawMessage
	if json.Unmarshal(raw, &envelope) != nil || envelope == nil {
		return DHCPv4OptionResourceModel{}, errors.New("malformed Kea option envelope")
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(envelope["option"], &fields) != nil || fields == nil {
		return DHCPv4OptionResourceModel{}, errors.New("missing or malformed option wrapper")
	}
	return decodeOption(fields, id, false)
}

func optionMissing(err error) bool {
	var missing *opnsense.NotFoundError
	return errors.As(err, &missing)
}

var optionUUIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func optionIDValid(id string) bool { return optionUUIDPattern.MatchString(id) }

func optionSnapshot(ctx context.Context, c *opnsense.Client) (map[string]string, error) {
	result := map[string]string{}
	expectedTotal := -1
	for page := 1; ; page++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := c.AcquireRead(ctx); err != nil {
			return nil, err
		}
		raw, err := optionRequest(ctx, c, http.MethodPost, dhcpv4OptionReqOpts.SearchEndpoint, map[string]any{"current": page, "rowCount": 500, "searchPhrase": ""})
		c.ReleaseRead()
		if err != nil {
			return nil, err
		}
		var response struct {
			Rows     json.RawMessage `json:"rows"`
			RowCount *int            `json:"rowCount"`
			Total    *int            `json:"total"`
			Current  *int            `json:"current"`
		}
		if json.Unmarshal(raw, &response) != nil || response.RowCount == nil || response.Total == nil || response.Current == nil {
			return nil, errors.New("malformed option search pagination")
		}
		var rows []struct {
			UUID        string `json:"uuid"`
			Description string `json:"description"`
		}
		if json.Unmarshal(response.Rows, &rows) != nil || rows == nil || *response.RowCount != len(rows) || *response.Current != page || *response.Total < 0 {
			return nil, errors.New("malformed option search page")
		}
		if expectedTotal < 0 {
			expectedTotal = *response.Total
		}
		if *response.Total != expectedTotal || len(result)+len(rows) > expectedTotal || (len(rows) == 0 && len(result) < expectedTotal) {
			return nil, errors.New("incomplete or inconsistent option search")
		}
		for _, row := range rows {
			if !optionIDValid(row.UUID) || row.Description == "" {
				return nil, errors.New("malformed option search identity")
			}
			if _, exists := result[strings.ToLower(row.UUID)]; exists {
				return nil, errors.New("duplicate option search UUID")
			}
			result[strings.ToLower(row.UUID)] = row.Description
		}
		if len(result) == expectedTotal {
			break
		}
	}
	return result, nil
}

func optionValidateCatalog(ctx context.Context, c *opnsense.Client, p optionPayload) error {
	if err := c.AcquireRead(ctx); err != nil {
		return err
	}
	defer c.ReleaseRead()
	raw, err := optionRequest(ctx, c, http.MethodGet, dhcpv4OptionReqOpts.GetEndpoint, nil)
	if err != nil {
		return err
	}
	fields, err := parseOptionTemplate(raw)
	if err != nil {
		return err
	}
	for _, field := range []string{"code", "match_code", "encoding", "match_encoding"} {
		requested, configured := p[field]
		if !configured || requested == "" {
			continue
		}
		_, choices, err := optionSelection(fields[field], field, true)
		if err != nil {
			return err
		}
		if _, ok := choices[requested]; !ok {
			return fmt.Errorf("%s selector key %q is unavailable in appliance model", field, requested)
		}
	}
	return nil
}

func createDHCPv4Option(ctx context.Context, c *opnsense.Client, p optionPayload) (string, error) {
	if err := optionValidateCatalog(ctx, c, p); err != nil {
		return "", err
	}
	before, err := optionSnapshot(ctx, c)
	if err != nil {
		return "", err
	}
	for _, description := range before {
		if description == p["description"] {
			return "", errors.New("option description already exists; import its UUID rather than recreating")
		}
	}
	raw, mutationErr := optionRequest(ctx, c, http.MethodPost, dhcpv4OptionReqOpts.AddEndpoint, map[string]any{"option": p})
	id := ""
	if mutationErr == nil {
		id, mutationErr = opnsense.ParseAddResponse(raw)
	}
	if optionIDValid(id) {
		if _, preexisting := before[strings.ToLower(id)]; !preexisting {
			return id, nil
		}
	}
	var validation *opnsense.ValidationError
	if errors.As(mutationErr, &validation) {
		return "", mutationErr
	}
	after, err := optionSnapshot(ctx, c)
	if err != nil {
		return "", fmt.Errorf("uncertain option add; manual authoritative reconciliation required: %w", err)
	}
	matches := []string{}
	for candidate, description := range after {
		if _, known := before[candidate]; known || description != p["description"] {
			continue
		}
		remote, err := readDHCPv4Option(ctx, c, candidate)
		if err != nil {
			return "", fmt.Errorf("uncertain option add; manual authoritative reconciliation required: %w", err)
		}
		if optionMatches(remote, p) {
			matches = append(matches, candidate)
		}
	}
	if len(matches) != 1 {
		return "", fmt.Errorf("uncertain option add: %d new complete payload matches; manually reconcile appliance objects before retrying", len(matches))
	}
	return matches[0], nil
}

const (
	optionRecoveryKey   = "kea_option_recovery_v1"
	optionSaved         = "saved"
	optionSaveUnknown   = "save_unknown"
	optionDeleted       = "deleted"
	optionDeleteUnknown = "delete_unknown"
)

type optionPrivate interface {
	GetKey(context.Context, string) ([]byte, diag.Diagnostics)
	SetKey(context.Context, string, []byte) diag.Diagnostics
}
type optionRecovery struct {
	ID      string        `json:"id"`
	Kind    string        `json:"kind"`
	Payload optionPayload `json:"payload,omitempty"`
}

func optionMark(ctx context.Context, p optionPrivate, id, kind string, payload optionPayload) diag.Diagnostics {
	if p == nil || reflect.ValueOf(p).IsNil() {
		var d diag.Diagnostics
		d.AddError("Missing durable Kea option recovery state", "Private state is required for mutations.")
		return d
	}
	if kind == "" {
		return p.SetKey(ctx, optionRecoveryKey, nil)
	}
	raw, err := json.Marshal(optionRecovery{ID: id, Kind: kind, Payload: payload})
	if err != nil {
		var d diag.Diagnostics
		d.AddError("Cannot encode Kea option recovery state", err.Error())
		return d
	}
	return p.SetKey(ctx, optionRecoveryKey, raw)
}

func optionPending(ctx context.Context, p optionPrivate, id string) (optionRecovery, diag.Diagnostics) {
	var r optionRecovery
	var d diag.Diagnostics
	if p == nil || reflect.ValueOf(p).IsNil() {
		return r, d
	}
	raw, diags := p.GetKey(ctx, optionRecoveryKey)
	d.Append(diags...)
	if len(raw) == 0 {
		return r, d
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&r)
	if err == nil {
		var extra any
		if decoder.Decode(&extra) != io.EOF {
			err = errors.New("trailing recovery data")
		}
	}
	// This owner has no prior private format: only bytes emitted by optionMark
	// are valid. Round-trip equality rejects duplicate keys, case aliases and
	// other ambiguous encodings accepted by encoding/json.
	canonical, canonicalErr := json.Marshal(r)
	if canonicalErr != nil || !bytes.Equal(raw, canonical) {
		err = errors.New("noncanonical recovery encoding")
	}
	valid := r.ID == id && optionIDValid(id) && (r.Kind == optionSaved || r.Kind == optionSaveUnknown || r.Kind == optionDeleted || r.Kind == optionDeleteUnknown)
	if r.Kind == optionSaveUnknown || r.Kind == optionSaved {
		valid = valid && validateOptionPayload(r.Payload, false) == nil
	} else {
		valid = valid && len(r.Payload) == 0
	}
	if err != nil || !valid {
		d.AddError("Invalid Kea option recovery state", "Corrupt, unsupported, UUID-mismatched or invalid requested wire payload; no mutation was attempted.")
	}
	return r, d
}

func recoverDHCPv4Option(ctx context.Context, c *opnsense.Client, id string, pending optionRecovery) (DHCPv4OptionResourceModel, bool, string, error) {
	remote, err := readDHCPv4Option(ctx, c, id)
	if err != nil {
		if !optionMissing(err) {
			return remote, false, "", err
		}
		if pending.Kind == optionSaveUnknown || pending.Kind == optionSaved {
			return remote, false, "", errors.New("uncertain saved option is absent; identity and requested payload retained for manual authoritative reconciliation")
		}
		if pending.Kind != "" {
			if err := activateDHCPv4Option(ctx, c); err != nil {
				return remote, false, "", err
			}
			// Activation and absence are separate: a restored UUID must retain identity.
			_, err = readDHCPv4Option(ctx, c, id)
			if !optionMissing(err) {
				return remote, false, "", errors.New("option absence not confirmed after activation; reconcile retained UUID")
			}
		}
		return remote, true, "", nil
	}
	if pending.Kind == optionDeleted || pending.Kind == optionDeleteUnknown {
		return remote, false, "Tracked UUID remains present; deletion uncertainty cleared without activation. Run a fresh normal destroy.", nil
	}
	if pending.Kind == optionSaveUnknown || pending.Kind == optionSaved {
		if !optionMatches(remote, pending.Payload) {
			return remote, false, "Authoritative values differ from the requested save; reconciled without another set or activation. Review a fresh plan.", nil
		}
		if err := activateDHCPv4Option(ctx, c); err != nil {
			return remote, false, "", err
		}
	}
	return remote, false, "", nil
}

func activateDHCPv4Option(ctx context.Context, c *opnsense.Client) error {
	raw, err := optionRequest(ctx, c, http.MethodPost, dhcpv4OptionReqOpts.ReconfigureEndpoint, nil)
	if err != nil {
		return err
	}
	var response struct {
		Status string `json:"status"`
	}
	if json.Unmarshal(raw, &response) != nil || response.Status != "ok" {
		return errors.New("kea reconfigure did not return status=ok")
	}
	return nil
}
