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
	_ resource.Resource                = &bgpCommunityListResource{}
	_ resource.ResourceWithImportState = &bgpCommunityListResource{}
)

var bgpCommunityListReqOpts = opnsense.ReqOpts{
	AddEndpoint:         "/api/quagga/bgp/add_communitylist",
	GetEndpoint:         "/api/quagga/bgp/get_communitylist",
	UpdateEndpoint:      "/api/quagga/bgp/set_communitylist",
	DeleteEndpoint:      "/api/quagga/bgp/del_communitylist",
	SearchEndpoint:      "/api/quagga/bgp/search_communitylist",
	ReconfigureEndpoint: "/api/quagga/service/reconfigure",
	Monad:               "communitylist",
}

type bgpCommunityListResource struct{ client *opnsense.Client }

func newBGPCommunityListResource() resource.Resource { return &bgpCommunityListResource{} }

func (r *bgpCommunityListResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_quagga_bgp_communitylist"
}

func (r *bgpCommunityListResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *bgpCommunityListResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	bgpCreate[BGPCommunityListResourceModel, bgpCommunityListAPIResponse, bgpCommunityListAPIRequest](ctx, r.client, bgpCommunityListReqOpts, req, resp, func(m *BGPCommunityListResourceModel) *bgpCommunityListAPIRequest { return m.toAPI(ctx) }, func(m *BGPCommunityListResourceModel, a *bgpCommunityListAPIResponse, id string) {
		m.fromAPI(ctx, a, id)
	})
}

func (r *bgpCommunityListResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	bgpRead[BGPCommunityListResourceModel, bgpCommunityListAPIResponse](ctx, r.client, bgpCommunityListReqOpts, req, resp, func(m *BGPCommunityListResourceModel, a *bgpCommunityListAPIResponse, id string) {
		m.fromAPI(ctx, a, id)
	})
}

func (r *bgpCommunityListResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	bgpUpdate[BGPCommunityListResourceModel, bgpCommunityListAPIResponse, bgpCommunityListAPIRequest](ctx, r.client, bgpCommunityListReqOpts, req, resp, func(m *BGPCommunityListResourceModel) *bgpCommunityListAPIRequest { return m.toAPI(ctx) }, func(m *BGPCommunityListResourceModel, a *bgpCommunityListAPIResponse, id string) {
		m.fromAPI(ctx, a, id)
	})
}

func (r *bgpCommunityListResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	bgpDelete[BGPCommunityListResourceModel](ctx, r.client, bgpCommunityListReqOpts, req, resp)
}

func (r *bgpCommunityListResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
