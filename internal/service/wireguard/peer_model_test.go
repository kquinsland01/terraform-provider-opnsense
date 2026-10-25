// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package wireguard

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/matthew-on-git/terraform-provider-opnsense/internal/tfconv"
)

func TestPeerModelMapsServers(t *testing.T) {
	model := PeerResourceModel{
		Enabled:       types.BoolValue(true),
		Name:          types.StringValue("cross-site-peer"),
		PublicKey:     types.StringValue("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="),
		TunnelAddress: tfconv.SliceToSet([]string{"169.254.1.1/32"}),
		ServerAddress: types.StringValue("edge-01.example.invalid"),
		ServerPort:    types.Int64Value(51822),
		Keepalive:     types.Int64Value(25),
		Servers:       tfconv.SliceToSet([]string{"00000000-0000-0000-0000-000000000001"}),
	}

	_, err := model.toAPI(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// Peer associations are intentionally not sent: server.peers is authoritative.

	var read PeerResourceModel
	if err := read.fromAPI(context.Background(), &wireguardPeerAPIResponse{
		Enabled:       strictBool(true),
		Name:          "cross-site-peer",
		PublicKey:     "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
		TunnelAddress: strictSelectedMapList{"169.254.1.1/32"},
		ServerAddress: "edge-01.example.invalid",
		ServerPort:    strictOptionalInteger{Value: 51822, Set: true},
		Keepalive:     strictOptionalInteger{Value: 25, Set: true},
		Servers:       strictSelectedMapList{"00000000-0000-0000-0000-000000000001"},
	}, "peer-uuid"); err != nil {
		t.Fatal(err)
	}

	if got := tfconv.SetToCSV(context.Background(), read.Servers); got != "00000000-0000-0000-0000-000000000001" {
		t.Fatalf("fromAPI servers = %q, want %q", got, "00000000-0000-0000-0000-000000000001")
	}
}

func TestPeerModelMapsMultipleTunnelAddresses(t *testing.T) {
	var read PeerResourceModel
	if err := read.fromAPI(context.Background(), &wireguardPeerAPIResponse{
		Enabled:       strictBool(true),
		Name:          "cross-site-peer",
		PublicKey:     "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
		TunnelAddress: strictSelectedMapList{"169.254.1.1/32", "192.0.2.0/24", "198.51.100.0/24"},
		ServerAddress: "edge-01.example.invalid",
		ServerPort:    strictOptionalInteger{Value: 51822, Set: true},
		Keepalive:     strictOptionalInteger{Value: 25, Set: true},
		Servers:       strictSelectedMapList{"00000000-0000-0000-0000-000000000001"},
	}, "peer-uuid"); err != nil {
		t.Fatal(err)
	}

	want := "169.254.1.1/32,192.0.2.0/24,198.51.100.0/24"
	if got := tfconv.SetToCSV(context.Background(), read.TunnelAddress); got != want {
		t.Fatalf("fromAPI tunnel_address = %q, want %q", got, want)
	}
}

func TestPeerModelOmitsUnsetKeepalive(t *testing.T) {
	model := PeerResourceModel{
		Enabled:       types.BoolValue(true),
		Name:          types.StringValue("no-keepalive-peer"),
		PublicKey:     types.StringValue("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="),
		TunnelAddress: tfconv.SliceToSet([]string{"169.254.1.1/32"}),
		ServerAddress: types.StringValue("edge-01.example.invalid"),
		ServerPort:    types.Int64Value(51822),
		Keepalive:     types.Int64Null(),
		Servers:       tfconv.SliceToSet([]string{"00000000-0000-0000-0000-000000000001"}),
	}

	api, err := model.toAPI(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if api.Keepalive != "" {
		t.Fatalf("toAPI keepalive = %q, want empty string for unset keepalive", api.Keepalive)
	}
}

func TestPeerModelMapsEmptyKeepaliveToNull(t *testing.T) {
	var read PeerResourceModel
	if err := read.fromAPI(context.Background(), &wireguardPeerAPIResponse{
		Enabled:       strictBool(true),
		Name:          "no-keepalive-peer",
		PublicKey:     "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
		TunnelAddress: strictSelectedMapList{"169.254.1.1/32"},
		ServerAddress: "edge-01.example.invalid",
		ServerPort:    strictOptionalInteger{Value: 51822, Set: true},
		Keepalive:     strictOptionalInteger{},
		Servers:       strictSelectedMapList{"00000000-0000-0000-0000-000000000001"},
	}, "peer-uuid"); err != nil {
		t.Fatal(err)
	}

	if !read.Keepalive.IsNull() {
		t.Fatalf("fromAPI keepalive = %#v, want null for empty API keepalive", read.Keepalive)
	}
}
