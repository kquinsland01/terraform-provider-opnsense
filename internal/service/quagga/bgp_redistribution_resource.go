// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package quagga

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"

	"github.com/matthew-on-git/terraform-provider-opnsense/pkg/opnsense"
)

var (
	_ resource.Resource                = &bgpRedistributionResource{}
	_ resource.ResourceWithImportState = &bgpRedistributionResource{}
)

var bgpRedistributionReqOpts = opnsense.ReqOpts{
	AddEndpoint:         "/api/quagga/bgp/add_redistribution",
	GetEndpoint:         "/api/quagga/bgp/get_redistribution",
	UpdateEndpoint:      "/api/quagga/bgp/set_redistribution",
	DeleteEndpoint:      "/api/quagga/bgp/del_redistribution",
	SearchEndpoint:      "/api/quagga/bgp/search_redistribution",
	ReconfigureEndpoint: "/api/quagga/service/reconfigure",
	Monad:               "redistribution",
}

type bgpRedistributionResource struct{ client *opnsense.Client }

func newBGPRedistributionResource() resource.Resource { return &bgpRedistributionResource{} }

func (r *bgpRedistributionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_quagga_bgp_redistribution"
}

func (r *bgpRedistributionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*opnsense.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Provider Data", "Expected *opnsense.Client.")
		return
	}
	r.client = client
}

func (r *bgpRedistributionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	bgpCreate[BGPRedistributionResourceModel, bgpRedistributionAPIResponse, bgpRedistributionAPIRequest](ctx, r.client, bgpRedistributionReqOpts, req, resp, func(m *BGPRedistributionResourceModel) *bgpRedistributionAPIRequest { return m.toAPI(ctx) }, func(m *BGPRedistributionResourceModel, a *bgpRedistributionAPIResponse, id string) {
		m.fromAPI(ctx, a, id)
	})
}

func (r *bgpRedistributionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	bgpRead[BGPRedistributionResourceModel, bgpRedistributionAPIResponse](ctx, r.client, bgpRedistributionReqOpts, req, resp, func(m *BGPRedistributionResourceModel, a *bgpRedistributionAPIResponse, id string) {
		m.fromAPI(ctx, a, id)
	})
}

func (r *bgpRedistributionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	bgpUpdate[BGPRedistributionResourceModel, bgpRedistributionAPIResponse, bgpRedistributionAPIRequest](ctx, r.client, bgpRedistributionReqOpts, req, resp, func(m *BGPRedistributionResourceModel) *bgpRedistributionAPIRequest { return m.toAPI(ctx) }, func(m *BGPRedistributionResourceModel, a *bgpRedistributionAPIResponse, id string) {
		m.fromAPI(ctx, a, id)
	})
}

func (r *bgpRedistributionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	bgpDelete[BGPRedistributionResourceModel](ctx, r.client, bgpRedistributionReqOpts, req, resp)
}

func (r *bgpRedistributionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
