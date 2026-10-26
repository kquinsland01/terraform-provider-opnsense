// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package system

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	groupNamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,32}$`)
	groupTriggers    = []string{"down", "downloss", "downlatency", "downlosslatency"}
	groupPools       = []string{"", "round-robin", "round-robin sticky-address"}
)

// GatewayGroupResourceModel owns five tiers. Later-tier clears are supported;
// clearing a populated first tier is rejected before any write.
type GatewayGroupResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Tier1       types.Set    `tfsdk:"tier_1"`
	Tier2       types.Set    `tfsdk:"tier_2"`
	Tier3       types.Set    `tfsdk:"tier_3"`
	Tier4       types.Set    `tfsdk:"tier_4"`
	Tier5       types.Set    `tfsdk:"tier_5"`
	Trigger     types.String `tfsdk:"trigger"`
	PoolOptions types.String `tfsdk:"pool_options"`
	Description types.String `tfsdk:"description"`
}

type gatewayGroupPayload map[string]string

func groupContains(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}

func (m GatewayGroupResourceModel) payload(_ context.Context) (gatewayGroupPayload, error) {
	p := gatewayGroupPayload{}
	for k, v := range map[string]types.String{"name": m.Name, "trigger": m.Trigger, "poolopts": m.PoolOptions, "descr": m.Description} {
		if v.IsNull() || v.IsUnknown() {
			return nil, fmt.Errorf("%s must be known and non-null", k)
		}
		p[k] = v.ValueString()
	}
	if !groupNamePattern.MatchString(p["name"]) {
		return nil, fmt.Errorf("name must match [A-Za-z0-9_-]{1,32}")
	}
	if !groupContains(groupTriggers, p["trigger"]) || !groupContains(groupPools, p["poolopts"]) {
		return nil, fmt.Errorf("invalid trigger or pool_options")
	}
	total := 0
	for i, set := range []types.Set{m.Tier1, m.Tier2, m.Tier3, m.Tier4, m.Tier5} {
		if set.IsNull() || set.IsUnknown() {
			return nil, fmt.Errorf("tier_%d must be known and non-null", i+1)
		}
		members := []string{}
		for _, element := range set.Elements() {
			v, ok := element.(types.String)
			if !ok || v.IsNull() || v.IsUnknown() || v.ValueString() == "" || strings.TrimSpace(v.ValueString()) != v.ValueString() || strings.ContainsAny(v.ValueString(), ",\r\n\t ") {
				return nil, fmt.Errorf("tier_%d contains an invalid or unresolved gateway name", i+1)
			}
			members = append(members, v.ValueString())
		}
		sort.Strings(members)
		key := "item"
		if i > 0 {
			key = fmt.Sprintf("item%d", i+1)
		}
		p[key] = strings.Join(members, ",")
		total += len(members)
	}
	if total == 0 {
		return nil, fmt.Errorf("at least one gateway must be selected across tier_1 through tier_5")
	}
	return p, nil
}

// groupSelection accepts the actual empty-array multi-select shape, but never
// treats missing/null/scalar fields or malformed flags as an empty selection.
func groupSelection(raw json.RawMessage, allowEmpty bool) ([]string, error) {
	var empty []json.RawMessage
	if allowEmpty && json.Unmarshal(raw, &empty) == nil && empty != nil && len(empty) == 0 {
		return []string{}, nil
	}
	var options map[string]json.RawMessage
	if len(raw) == 0 || string(raw) == "null" || json.Unmarshal(raw, &options) != nil || options == nil {
		return nil, fmt.Errorf("expected selected option map")
	}
	selected := []string{}
	for key, value := range options {
		var option struct {
			Selected *int `json:"selected"`
		}
		if json.Unmarshal(value, &option) != nil || option.Selected == nil || (*option.Selected != 0 && *option.Selected != 1) {
			return nil, fmt.Errorf("invalid selected flag")
		}
		if allowEmpty && (key == "" || strings.TrimSpace(key) != key || strings.ContainsAny(key, ",\r\n\t ")) {
			return nil, fmt.Errorf("empty gateway key")
		}
		if *option.Selected == 1 {
			selected = append(selected, key)
		}
	}
	sort.Strings(selected)
	return selected, nil
}

func groupEnum(raw json.RawMessage, valid []string) (string, error) {
	selected, err := groupSelection(raw, false)
	if err != nil || len(selected) != 1 {
		return "", fmt.Errorf("expected exactly one enum selection")
	}
	var options map[string]json.RawMessage
	if err := json.Unmarshal(raw, &options); err != nil {
		return "", err
	}
	for key := range options {
		if !groupContains(valid, key) {
			return "", fmt.Errorf("unsupported enum key %q", key)
		}
	}
	return selected[0], nil
}

func decodeGatewayGroup(ctx context.Context, raw map[string]json.RawMessage, id string) (GatewayGroupResourceModel, error) {
	m := GatewayGroupResourceModel{ID: types.StringValue(id)}
	scalar := func(key string) (string, error) {
		var v *string
		if json.Unmarshal(raw[key], &v) != nil || v == nil {
			return "", fmt.Errorf("missing or malformed %s", key)
		}
		return *v, nil
	}
	name, err := scalar("name")
	if err != nil {
		return m, err
	}
	if !groupNamePattern.MatchString(name) {
		return m, fmt.Errorf("invalid immutable gateway group name")
	}
	m.Name = types.StringValue(name)
	description, err := scalar("descr")
	if err != nil {
		return m, err
	}
	m.Description = types.StringValue(description)
	tiers := []*types.Set{&m.Tier1, &m.Tier2, &m.Tier3, &m.Tier4, &m.Tier5}
	for i, dest := range tiers {
		key := "item"
		if i > 0 {
			key = fmt.Sprintf("item%d", i+1)
		}
		members, e := groupSelection(raw[key], true)
		if e != nil {
			return m, fmt.Errorf("%s: %w", key, e)
		}
		set, d := types.SetValueFrom(ctx, types.StringType, members)
		if d.HasError() {
			return m, fmt.Errorf("invalid %s", key)
		}
		*dest = set
	}
	for key, dest := range map[string]*types.String{"trigger": &m.Trigger, "poolopts": &m.PoolOptions} {
		valid := groupTriggers
		if key == "poolopts" {
			valid = groupPools
		}
		selected, e := groupEnum(raw[key], valid)
		if e != nil {
			return m, fmt.Errorf("%s: %w", key, e)
		}
		*dest = types.StringValue(selected)
	}
	_, err = m.payload(ctx)
	return m, err
}
