// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package quagga

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// A present empty selector is API truth. An absent field is not an empty clear.
// Store keys only; unexposed credential values never enter the public model.
type frrReadPresence struct{ fields map[string]bool }

func (p *frrReadPresence) hasField(name string) bool { return p.fields[name] }
func (p *frrReadPresence) recordFields(raw []byte, model any) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return err
	}
	v := reflect.TypeOf(model).Elem()
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		key := strings.Split(field.Tag.Get("json"), ",")[0]
		if field.Type.Kind() == reflect.String && bytes.Equal(bytes.TrimSpace(fields[key]), []byte("null")) {
			return fmt.Errorf("FRR owned scalar %s is null", key)
		}
	}
	p.fields = make(map[string]bool, len(fields))
	for name := range fields {
		p.fields[name] = true
	}
	return nil
}

func (a *bgpNeighborAPIResponse) UnmarshalJSON(raw []byte) error {
	type plain bgpNeighborAPIResponse
	if err := json.Unmarshal(raw, (*plain)(a)); err != nil {
		return err
	}
	return a.recordFields(raw, a)
}

func (a *bgpPeerGroupAPIResponse) UnmarshalJSON(raw []byte) error {
	type plain bgpPeerGroupAPIResponse
	if err := json.Unmarshal(raw, (*plain)(a)); err != nil {
		return err
	}
	return a.recordFields(raw, a)
}

func (a *routeMapAPIResponse) UnmarshalJSON(raw []byte) error {
	type plain routeMapAPIResponse
	if err := json.Unmarshal(raw, (*plain)(a)); err != nil {
		return err
	}
	return a.recordFields(raw, a)
}

func (a *prefixListAPIResponse) UnmarshalJSON(raw []byte) error {
	type plain prefixListAPIResponse
	if err := json.Unmarshal(raw, (*plain)(a)); err != nil {
		return err
	}
	return a.recordFields(raw, a)
}

func (a *bgpASPathAPIResponse) UnmarshalJSON(raw []byte) error {
	type plain bgpASPathAPIResponse
	if err := json.Unmarshal(raw, (*plain)(a)); err != nil {
		return err
	}
	return a.recordFields(raw, a)
}

func (a *bgpCommunityListAPIResponse) UnmarshalJSON(raw []byte) error {
	type plain bgpCommunityListAPIResponse
	if err := json.Unmarshal(raw, (*plain)(a)); err != nil {
		return err
	}
	return a.recordFields(raw, a)
}

func (a *bgpRedistributionAPIResponse) UnmarshalJSON(raw []byte) error {
	type plain bgpRedistributionAPIResponse
	if err := json.Unmarshal(raw, (*plain)(a)); err != nil {
		return err
	}
	return a.recordFields(raw, a)
}

func (a *bgpGlobalAPIResponse) UnmarshalJSON(raw []byte) error {
	type plain bgpGlobalAPIResponse
	if err := json.Unmarshal(raw, (*plain)(a)); err != nil {
		return err
	}
	return a.recordFields(raw, a)
}

func (a *generalAPIResponse) UnmarshalJSON(raw []byte) error {
	type plain generalAPIResponse
	if err := json.Unmarshal(raw, (*plain)(a)); err != nil {
		return err
	}
	return a.recordFields(raw, a)
}
