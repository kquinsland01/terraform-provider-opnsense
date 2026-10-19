// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package kea

import (
	"context"
	"reflect"

	"github.com/hashicorp/terraform-plugin-framework/diag"

	"github.com/matthew-on-git/terraform-provider-opnsense/pkg/opnsense"
)

const keaPendingActivationKey = "kea_pending_activation_v1"

const (
	keaPendingMutation = `"mutation"`
	keaPendingDelete   = `"delete"`
)

type keaPrivateState interface {
	GetKey(context.Context, string) ([]byte, diag.Diagnostics)
	SetKey(context.Context, string, []byte) diag.Diagnostics
}

func getKeaPendingActivation(ctx context.Context, private keaPrivateState) (string, diag.Diagnostics) {
	if private == nil || reflect.ValueOf(private).IsNil() {
		return "", nil
	}
	value, diagnostics := private.GetKey(ctx, keaPendingActivationKey)
	return string(value), diagnostics
}

func setKeaPendingActivation(ctx context.Context, private keaPrivateState, operation string) diag.Diagnostics {
	return private.SetKey(ctx, keaPendingActivationKey, []byte(operation))
}

func clearKeaPendingActivation(ctx context.Context, private keaPrivateState) diag.Diagnostics {
	return private.SetKey(ctx, keaPendingActivationKey, nil)
}

func retryKeaPendingActivation(ctx context.Context, client *opnsense.Client, opts opnsense.ReqOpts, operation string) error {
	if operation == "" {
		return nil
	}
	return opnsense.RetryReconfigure(ctx, client, opts)
}
