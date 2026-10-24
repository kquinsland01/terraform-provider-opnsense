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
	_ resource.Resource                = &bgpNeighborResource{}
	_ resource.ResourceWithImportState = &bgpNeighborResource{}
)

var bgpNeighborReqOpts = opnsense.ReqOpts{
	AddEndpoint:         "/api/quagga/bgp/add_neighbor",
	GetEndpoint:         "/api/quagga/bgp/get_neighbor",
	UpdateEndpoint:      "/api/quagga/bgp/set_neighbor",
	DeleteEndpoint:      "/api/quagga/bgp/del_neighbor",
	SearchEndpoint:      "/api/quagga/bgp/search_neighbor",
	ReconfigureEndpoint: "/api/quagga/service/reconfigure",
	Monad:               "neighbor",
}

type bgpNeighborResource struct{ client *opnsense.Client }

func newBGPNeighborResource() resource.Resource { return &bgpNeighborResource{} }

func (r *bgpNeighborResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_quagga_bgp_neighbor"
}

func (r *bgpNeighborResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *bgpNeighborResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	bgpCreate[BGPNeighborResourceModel, bgpNeighborAPIResponse, bgpNeighborAPIRequest](ctx, r.client, bgpNeighborReqOpts, req, resp, func(m *BGPNeighborResourceModel) *bgpNeighborAPIRequest { return m.toAPI(ctx) }, func(m *BGPNeighborResourceModel, a *bgpNeighborAPIResponse, id string) { m.fromAPI(ctx, a, id) })
}

func (r *bgpNeighborResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	bgpRead[BGPNeighborResourceModel, bgpNeighborAPIResponse](ctx, r.client, bgpNeighborReqOpts, req, resp, func(m *BGPNeighborResourceModel, a *bgpNeighborAPIResponse, id string) { m.fromAPI(ctx, a, id) })
}

func (r *bgpNeighborResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	bgpUpdate[BGPNeighborResourceModel, bgpNeighborAPIResponse, bgpNeighborAPIRequest](ctx, r.client, bgpNeighborReqOpts, req, resp, func(m *BGPNeighborResourceModel) *bgpNeighborAPIRequest { return m.toAPI(ctx) }, func(m *BGPNeighborResourceModel, a *bgpNeighborAPIResponse, id string) { m.fromAPI(ctx, a, id) })
}

func (r *bgpNeighborResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	bgpDelete[BGPNeighborResourceModel](ctx, r.client, bgpNeighborReqOpts, req, resp)
}

func (r *bgpNeighborResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
