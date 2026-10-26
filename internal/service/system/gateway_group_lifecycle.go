// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package system

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
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/matthew-on-git/terraform-provider-opnsense/pkg/opnsense"
)

var gatewayGroupReqOpts = opnsense.ReqOpts{
	AddEndpoint: "/api/routing/group_settings/add", GetEndpoint: "/api/routing/group_settings/get", UpdateEndpoint: "/api/routing/group_settings/set", DeleteEndpoint: "/api/routing/group_settings/del", SearchEndpoint: "/api/routing/group_settings/search", ReconfigureEndpoint: "/api/routing/group_settings/reconfigure", Monad: "gateway_group", StrictMissingObjectResponse: true, StrictDeleteResponse: true, StrictReconfigureResponse: true,
}

const (
	groupPendingKey    = "gateway_group_pending_v1"
	groupSaved         = `"saved"`
	groupDeleted       = `"deleted"`
	groupDeleteUnknown = `"delete_unknown"`
	groupAttemptKey    = "gateway_group_attempt_v1"
	groupSaveUnknown   = `"save_unknown"`
)

type groupPrivate interface {
	GetKey(context.Context, string) ([]byte, diag.Diagnostics)
	SetKey(context.Context, string, []byte) diag.Diagnostics
}

func groupPending(ctx context.Context, p groupPrivate) (string, diag.Diagnostics) {
	if p == nil || reflect.ValueOf(p).IsNil() {
		return "", nil
	}
	v, d := p.GetKey(ctx, groupPendingKey)
	value := string(v)
	if value != "" && value != groupSaved && value != groupDeleted && value != groupDeleteUnknown && value != groupSaveUnknown {
		d.AddError("Invalid gateway group recovery state", "Pending marker is corrupt or unsupported; no appliance mutation was attempted.")
	}
	return value, d
}

func groupMark(ctx context.Context, p groupPrivate, value string) diag.Diagnostics {
	if p == nil || reflect.ValueOf(p).IsNil() {
		return nil
	}
	if value == "" {
		d := p.SetKey(ctx, groupPendingKey, nil)
		d.Append(p.SetKey(ctx, groupAttemptKey, nil)...)
		return d
	}
	return p.SetKey(ctx, groupPendingKey, []byte(value))
}

// groupRequest is used only while holding the shared client mutation lock, or
// a read slot. It never retries an uncertain write.
func groupRequest(ctx context.Context, c *opnsense.Client, method, endpoint string, payload any) ([]byte, error) {
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
		return nil, errors.New("gateway group requires the authenticated single-attempt transport")
	}
	// Bypass shared automatic retries for non-idempotent group writes. Copy the
	// underlying authenticated client without changing any shared client settings.
	singleAttempt := *transport.Client.HTTPClient
	singleAttempt.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	requester := &singleAttempt
	if method == http.MethodGet {
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

var groupStableVersion = regexp.MustCompile(`^([0-9]+)\.([0-9]+)(?:\.([0-9]+))?(?:_[0-9]+)?$`)

func requireGatewayGroup(ctx context.Context, c *opnsense.Client) error {
	if c == nil {
		return errors.New("gateway group client is not configured")
	}
	if err := c.AcquireRead(ctx); err != nil {
		return err
	}
	defer c.ReleaseRead()
	raw, err := groupRequest(ctx, c, http.MethodGet, "/api/core/firmware/status", nil)
	if err != nil {
		return fmt.Errorf("verify gateway group firmware: %w", err)
	}
	var status struct {
		Product struct {
			Name    string `json:"product_name"`
			ID      string `json:"product_id"`
			Version string `json:"product_version"`
		} `json:"product"`
	}
	if json.Unmarshal(raw, &status) != nil || (status.Product.Name != "OPNsense" && status.Product.ID != "opnsense") {
		return errors.New("gateway groups require a verified OPNsense firmware identity")
	}
	parts := groupStableVersion.FindStringSubmatch(status.Product.Version)
	if parts == nil {
		return errors.New("gateway groups require a known stable OPNsense version, minimum 26.7.7")
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
		return errors.New("gateway groups require stable OPNsense 26.7.7 or newer")
	}
	raw, err = groupRequest(ctx, c, http.MethodGet, gatewayGroupReqOpts.GetEndpoint, nil)
	if err != nil {
		return fmt.Errorf("verify gateway group model: %w", err)
	}
	_, err = parseGatewayGroupTemplate(raw)
	return err
}

func parseGatewayGroupTemplate(raw []byte) (map[string]json.RawMessage, error) {
	var envelope map[string]map[string]json.RawMessage
	if json.Unmarshal(raw, &envelope) != nil || envelope["gateway_group"] == nil {
		return nil, errors.New("gateway group model unavailable or malformed")
	}
	model := envelope["gateway_group"]
	for _, key := range []string{"name", "descr"} {
		var scalar *string
		if json.Unmarshal(model[key], &scalar) != nil || scalar == nil {
			return nil, fmt.Errorf("gateway group model has malformed %s", key)
		}
	}
	for _, key := range []string{"item", "item2", "item3", "item4", "item5"} {
		if _, err := groupSelection(model[key], true); err != nil {
			return nil, fmt.Errorf("gateway group model %s: %w", key, err)
		}
	}
	for key, valid := range map[string][]string{"trigger": groupTriggers, "poolopts": groupPools} {
		_, err := groupEnum(model[key], valid)
		if err != nil {
			return nil, fmt.Errorf("gateway group model has unsupported %s selection", key)
		}
	}
	return model, nil
}

func readGatewayGroup(ctx context.Context, c *opnsense.Client, id string) (GatewayGroupResourceModel, error) {
	if err := c.AcquireRead(ctx); err != nil {
		return GatewayGroupResourceModel{}, err
	}
	defer c.ReleaseRead()
	raw, err := groupRequest(ctx, c, http.MethodGet, gatewayGroupReqOpts.GetEndpoint+"/"+id, nil)
	if err != nil {
		var missing *opnsense.PluginNotFoundError
		if errors.As(err, &missing) {
			return GatewayGroupResourceModel{}, &opnsense.NotFoundError{Message: "gateway group UUID not found after successful model gate"}
		}
		return GatewayGroupResourceModel{}, err
	}
	emptyArray := func(value []byte) bool {
		var values []json.RawMessage
		return json.Unmarshal(value, &values) == nil && values != nil && len(values) == 0
	}
	if emptyArray(raw) {
		return GatewayGroupResourceModel{}, &opnsense.NotFoundError{Message: "gateway group UUID not found"}
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(raw, &envelope); err != nil || envelope == nil {
		return GatewayGroupResourceModel{}, errors.New("malformed gateway group envelope")
	}
	body, exists := envelope["gateway_group"]
	if !exists {
		return GatewayGroupResourceModel{}, errors.New("missing gateway_group wrapper")
	}
	if emptyArray(body) {
		return GatewayGroupResourceModel{}, &opnsense.NotFoundError{Message: "gateway group UUID not found"}
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil || fields == nil {
		return GatewayGroupResourceModel{}, errors.New("malformed gateway group object")
	}
	return decodeGatewayGroup(ctx, fields, id)
}

func groupMarkAttempt(ctx context.Context, p groupPrivate, id string, payload gatewayGroupPayload) diag.Diagnostics {
	raw, err := json.Marshal(struct {
		ID      string              `json:"id"`
		Payload gatewayGroupPayload `json:"payload"`
	}{id, payload})
	if err != nil {
		var d diag.Diagnostics
		d.AddError("Cannot preserve attempted gateway group save", err.Error())
		return d
	}
	if p == nil || reflect.ValueOf(p).IsNil() {
		return nil
	}
	d := p.SetKey(ctx, groupAttemptKey, raw)
	d.Append(groupMark(ctx, p, groupSaveUnknown)...)
	return d
}

func groupAttempt(ctx context.Context, p groupPrivate, id string) (gatewayGroupPayload, diag.Diagnostics) {
	var d diag.Diagnostics
	if p == nil || reflect.ValueOf(p).IsNil() {
		d.AddError("Invalid gateway group attempted save", "Missing durable attempted payload.")
		return nil, d
	}
	raw, diags := p.GetKey(ctx, groupAttemptKey)
	d.Append(diags...)
	var attempted struct {
		ID      string              `json:"id"`
		Payload gatewayGroupPayload `json:"payload"`
	}
	if json.Unmarshal(raw, &attempted) != nil || attempted.ID != id || len(attempted.Payload) != 9 {
		d.AddError("Invalid gateway group attempted save", "Missing, corrupt or mismatched durable attempted payload; no activation was attempted.")
		return nil, d
	}
	model := GatewayGroupResourceModel{Name: types.StringValue(attempted.Payload["name"]), Trigger: types.StringValue(attempted.Payload["trigger"]), PoolOptions: types.StringValue(attempted.Payload["poolopts"]), Description: types.StringValue(attempted.Payload["descr"])}
	tiers := []*types.Set{&model.Tier1, &model.Tier2, &model.Tier3, &model.Tier4, &model.Tier5}
	for i, key := range []string{"item", "item2", "item3", "item4", "item5"} {
		value, exists := attempted.Payload[key]
		if !exists {
			d.AddError("Invalid gateway group attempted save", "Missing tier key.")
			return nil, d
		}
		members := []string{}
		if value != "" {
			members = strings.Split(value, ",")
		}
		set, diags := types.SetValueFrom(ctx, types.StringType, members)
		d.Append(diags...)
		*tiers[i] = set
	}
	normalized, err := model.payload(ctx)
	if err != nil || !reflect.DeepEqual(normalized, attempted.Payload) {
		d.AddError("Invalid gateway group attempted save", "Attempted payload is malformed or noncanonical.")
		return nil, d
	}
	return attempted.Payload, d
}

func groupMissing(err error) bool {
	var missing *opnsense.NotFoundError
	return errors.As(err, &missing)
}

type groupIdentity struct {
	UUID string `json:"uuid"`
	Name string `json:"name"`
}

func groupSnapshot(ctx context.Context, c *opnsense.Client) (map[string]string, error) {
	rows, err := opnsense.Search[groupIdentity](ctx, c, gatewayGroupReqOpts, opnsense.SearchParams{})
	if err != nil {
		return nil, err
	}
	result := map[string]string{}
	for _, row := range rows {
		if !groupIDValid(row.UUID) || !groupNamePattern.MatchString(row.Name) {
			return nil, errors.New("invalid gateway group search identity")
		}
		if _, exists := result[row.UUID]; exists {
			return nil, errors.New("duplicate gateway group search UUID")
		}
		result[row.UUID] = row.Name
	}
	return result, nil
}

func createGatewayGroup(ctx context.Context, c *opnsense.Client, payload gatewayGroupPayload) (string, error) {
	if err := waitGatewayGroupMembers(ctx, c, payload); err != nil {
		return "", err
	}
	before, err := groupSnapshot(ctx, c)
	if err != nil {
		return "", err
	}
	for _, name := range before {
		if name == payload["name"] {
			return "", errors.New("gateway group name already exists; import its UUID rather than recreating")
		}
	}
	raw, mutationErr := groupRequest(ctx, c, http.MethodPost, gatewayGroupReqOpts.AddEndpoint, map[string]any{"gateway_group": payload})
	id := ""
	if mutationErr == nil {
		id, mutationErr = opnsense.ParseAddResponse(raw)
	}
	if groupIDValid(id) {
		return id, nil
	}
	if id != "" {
		mutationErr = errors.New("add response returned a malformed UUID")
	}
	var validation *opnsense.ValidationError
	if errors.As(mutationErr, &validation) {
		return "", mutationErr
	}
	after, err := groupSnapshot(ctx, c)
	if err != nil {
		return "", fmt.Errorf("uncertain gateway group add; recovery search failed: %w", err)
	}
	matches := []string{}
	for candidate, name := range after {
		if _, known := before[candidate]; known || name != payload["name"] {
			continue
		}
		remote, err := readGatewayGroup(ctx, c, candidate)
		if err != nil {
			return "", fmt.Errorf("uncertain add recovery read: %w", err)
		}
		got, err := remote.payload(ctx)
		if err != nil {
			return "", err
		}
		if reflect.DeepEqual(got, payload) {
			matches = append(matches, candidate)
		}
	}
	if len(matches) != 1 {
		return "", fmt.Errorf("uncertain gateway group add: %d new complete payload matches; reconcile appliance objects before retrying; original error: %w", len(matches), mutationErr)
	}
	return matches[0], nil
}

func saveGatewayGroup(ctx context.Context, c *opnsense.Client, payload gatewayGroupPayload, id string) (bool, error) {
	if err := waitGatewayGroupMembers(ctx, c, payload); err != nil {
		return false, err
	}
	raw, err := groupRequest(ctx, c, http.MethodPost, gatewayGroupReqOpts.UpdateEndpoint+"/"+id, map[string]any{"gateway_group": payload})
	if err == nil {
		_, err = opnsense.ParseMutationResponse(raw)
	}
	if err == nil {
		return true, nil
	}
	var validation *opnsense.ValidationError
	if errors.As(err, &validation) {
		return false, err
	}
	remote, readErr := readGatewayGroup(ctx, c, id)
	if readErr != nil {
		return false, fmt.Errorf("uncertain gateway group save (%v); authoritative read failed: %w", err, readErr)
	}
	got, readErr := remote.payload(ctx)
	if readErr == nil && reflect.DeepEqual(got, payload) {
		return true, nil
	}
	return false, fmt.Errorf("gateway group save not confirmed by authoritative payload: %w", err)
}

func groupIDValid(id string) bool {
	// Identity is interpolated into a URL path: never permit path/query injection.
	return regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`).MatchString(strings.TrimSpace(id)) && id == strings.TrimSpace(id)
}

// GatewayGroupItemField consumes configd's interface gateways list, cached for
// 20 seconds on tagged 26.7.7. Route reconfigure does not invalidate that cache.
// Wait only for authoritative read-only model eligibility, never retry a save.
const (
	groupEligibilityTimeout  = 30 * time.Second
	groupEligibilityInterval = time.Second
)

type groupEligibilityError struct{ cause error }

func (e *groupEligibilityError) Error() string { return e.cause.Error() }
func (e *groupEligibilityError) Unwrap() error { return e.cause }

type groupEligibilityPolicy struct {
	timeout  time.Duration
	interval time.Duration
	now      func() time.Time
	wait     func(context.Context, time.Duration) error
}

func waitGatewayGroupMembers(ctx context.Context, c *opnsense.Client, payload gatewayGroupPayload) error {
	return waitGatewayGroupMembersWithPolicy(ctx, c, payload, groupEligibilityPolicy{timeout: groupEligibilityTimeout, interval: groupEligibilityInterval, now: time.Now, wait: waitGatewayGroupEligibility})
}

func waitGatewayGroupEligibility(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func waitGatewayGroupMembersWithPolicy(ctx context.Context, c *opnsense.Client, payload gatewayGroupPayload, policy groupEligibilityPolicy) error {
	bounded, cancel := context.WithTimeout(ctx, policy.timeout)
	defer cancel()
	deadline := policy.now().Add(policy.timeout)
	missing := []string{}
	failure := func(cause error) error {
		return &groupEligibilityError{cause: fmt.Errorf("gateway group planned gateway names are unavailable in authoritative model options (%s); read-only eligibility preflight failed: %w", strings.Join(missing, "; "), cause)}
	}
	for {
		if err := bounded.Err(); err != nil {
			return failure(err)
		}
		if !policy.now().Before(deadline) {
			return failure(context.DeadlineExceeded)
		}
		if err := c.AcquireRead(bounded); err != nil {
			return failure(err)
		}
		raw, err := groupRequest(bounded, c, http.MethodGet, gatewayGroupReqOpts.GetEndpoint, nil)
		c.ReleaseRead()
		if err != nil {
			return failure(err)
		}
		model, err := parseGatewayGroupTemplate(raw)
		if err != nil {
			return failure(err)
		}
		missing = nil
		for _, field := range []string{"item", "item2", "item3", "item4", "item5"} {
			if payload[field] == "" {
				continue
			}
			var options map[string]json.RawMessage
			if err := json.Unmarshal(model[field], &options); err != nil { // The valid empty array has no eligible names.
				options = nil
			}
			for _, name := range strings.Split(payload[field], ",") {
				if _, present := options[name]; !present {
					missing = append(missing, field+":"+name)
				}
			}
		}
		if len(missing) == 0 {
			return nil
		}
		sort.Strings(missing)
		remaining := deadline.Sub(policy.now())
		if remaining <= 0 {
			return failure(context.DeadlineExceeded)
		}
		delay := min(policy.interval, remaining)
		if err := policy.wait(bounded, delay); err != nil {
			return failure(err)
		}
	}
}
