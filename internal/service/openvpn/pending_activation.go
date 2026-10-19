// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package openvpn

import (
	"context"
	"reflect"

	"github.com/hashicorp/terraform-plugin-framework/diag"

	"github.com/matthew-on-git/terraform-provider-opnsense/pkg/opnsense"
)

const openVPNPendingActivationKey = "openvpn_pending_" + "activation_v1"
const (
	openVPNPendingMutation = `"mutation"`
	openVPNPendingDelete   = `"delete"`
)

type openVPNPrivateState interface {
	GetKey(context.Context, string) ([]byte, diag.Diagnostics)
	SetKey(context.Context, string, []byte) diag.Diagnostics
}

func getOpenVPNPending(ctx context.Context, p openVPNPrivateState) (string, diag.Diagnostics) {
	if p == nil || reflect.ValueOf(p).IsNil() {
		return "", nil
	}
	v, d := p.GetKey(ctx, openVPNPendingActivationKey)
	return string(v), d
}

func setOpenVPNPending(ctx context.Context, p openVPNPrivateState, value string) diag.Diagnostics {
	return p.SetKey(ctx, openVPNPendingActivationKey, []byte(value))
}

func clearOpenVPNPending(ctx context.Context, p openVPNPrivateState) diag.Diagnostics {
	return p.SetKey(ctx, openVPNPendingActivationKey, nil)
}

func retryOpenVPNPending(ctx context.Context, client *opnsense.Client, opts opnsense.ReqOpts) error {
	return opnsense.RetryReconfigure(ctx, client, opts)
}
