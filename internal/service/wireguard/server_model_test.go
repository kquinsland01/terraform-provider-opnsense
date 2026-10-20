// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package wireguard

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestServerResourceModelFromAPI_preservesDescriptionWhenAPIDropsIt(t *testing.T) {
	t.Parallel()

	model := ServerResourceModel{
		Description: types.StringValue("cross-site underlay"),
	}

	if err := model.fromAPI(context.Background(), &wireguardServerAPIResponse{
		Enabled:       strictBool(true),
		Instance:      "0",
		Name:          "wg0",
		Port:          strictOptionalInteger{Value: 51820, Set: true},
		TunnelAddress: strictSelectedMapList{"10.10.0.1/24"},
		Description:   "",
	}, "server-uuid"); err != nil {
		t.Fatal(err)
	}

	if got, want := model.Description.ValueString(), "cross-site underlay"; got != want {
		t.Fatalf("description = %q, want %q", got, want)
	}
}

func TestServerResourceModelFromAPIUsesReturnedDescription(t *testing.T) {
	t.Parallel()

	model := ServerResourceModel{
		Description: types.StringValue("planned description"),
	}

	if err := model.fromAPI(context.Background(), &wireguardServerAPIResponse{
		Enabled:       strictBool(true),
		Instance:      "0",
		Name:          "wg0",
		Port:          strictOptionalInteger{Value: 51820, Set: true},
		TunnelAddress: strictSelectedMapList{"10.10.0.1/24"},
		Description:   "api description",
	}, "server-uuid"); err != nil {
		t.Fatal(err)
	}

	if got, want := model.Description.ValueString(), "api description"; got != want {
		t.Fatalf("description = %q, want %q", got, want)
	}
}

func TestServerResourceModelFromAPIInitializesMissingDescription(t *testing.T) {
	t.Parallel()

	var model ServerResourceModel

	if err := model.fromAPI(context.Background(), &wireguardServerAPIResponse{
		Enabled:       strictBool(true),
		Instance:      "0",
		Name:          "wg0",
		Port:          strictOptionalInteger{Value: 51820, Set: true},
		TunnelAddress: strictSelectedMapList{"10.10.0.1/24"},
		Description:   "",
	}, "server-uuid"); err != nil {
		t.Fatal(err)
	}

	if model.Description.IsNull() || model.Description.IsUnknown() {
		t.Fatalf("description was not initialized: %#v", model.Description)
	}
	if got, want := model.Description.ValueString(), ""; got != want {
		t.Fatalf("description = %q, want %q", got, want)
	}
}
