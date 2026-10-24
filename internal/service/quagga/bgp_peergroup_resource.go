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
	_ resource.Resource                = &bgpPeerGroupResource{}
	_ resource.ResourceWithImportState = &bgpPeerGroupResource{}
)

var bgpPeerGroupReqOpts = opnsense.ReqOpts{
	AddEndpoint:         "/api/quagga/bgp/add_peergroup",
	GetEndpoint:         "/api/quagga/bgp/get_peergroup",
	UpdateEndpoint:      "/api/quagga/bgp/set_peergroup",
	DeleteEndpoint:      "/api/quagga/bgp/del_peergroup",
	SearchEndpoint:      "/api/quagga/bgp/search_peergroup",
	ReconfigureEndpoint: "/api/quagga/service/reconfigure",
	Monad:               "peergroup",
}

type bgpPeerGroupResource struct{ client *opnsense.Client }

func newBGPPeerGroupResource() resource.Resource { return &bgpPeerGroupResource{} }

func (r *bgpPeerGroupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_quagga_bgp_peergroup"
}

func (r *bgpPeerGroupResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *bgpPeerGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	bgpCreate[BGPPeerGroupResourceModel, bgpPeerGroupAPIResponse, bgpPeerGroupAPIRequest](ctx, r.client, bgpPeerGroupReqOpts, req, resp, func(m *BGPPeerGroupResourceModel) *bgpPeerGroupAPIRequest { return m.toAPI(ctx) }, func(m *BGPPeerGroupResourceModel, a *bgpPeerGroupAPIResponse, id string) { m.fromAPI(ctx, a, id) })
}

func (r *bgpPeerGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	bgpRead[BGPPeerGroupResourceModel, bgpPeerGroupAPIResponse](ctx, r.client, bgpPeerGroupReqOpts, req, resp, func(m *BGPPeerGroupResourceModel, a *bgpPeerGroupAPIResponse, id string) { m.fromAPI(ctx, a, id) })
}

func (r *bgpPeerGroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	bgpUpdate[BGPPeerGroupResourceModel, bgpPeerGroupAPIResponse, bgpPeerGroupAPIRequest](ctx, r.client, bgpPeerGroupReqOpts, req, resp, func(m *BGPPeerGroupResourceModel) *bgpPeerGroupAPIRequest { return m.toAPI(ctx) }, func(m *BGPPeerGroupResourceModel, a *bgpPeerGroupAPIResponse, id string) { m.fromAPI(ctx, a, id) })
}

func (r *bgpPeerGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	bgpDelete[BGPPeerGroupResourceModel](ctx, r.client, bgpPeerGroupReqOpts, req, resp)
}

func (r *bgpPeerGroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
