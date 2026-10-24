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
	_ resource.Resource                = &bgpASPathResource{}
	_ resource.ResourceWithImportState = &bgpASPathResource{}
)

var bgpASPathReqOpts = opnsense.ReqOpts{
	AddEndpoint:         "/api/quagga/bgp/add_aspath",
	GetEndpoint:         "/api/quagga/bgp/get_aspath",
	UpdateEndpoint:      "/api/quagga/bgp/set_aspath",
	DeleteEndpoint:      "/api/quagga/bgp/del_aspath",
	SearchEndpoint:      "/api/quagga/bgp/search_aspath",
	ReconfigureEndpoint: "/api/quagga/service/reconfigure",
	Monad:               "aspath",
}

type bgpASPathResource struct{ client *opnsense.Client }

func newBGPASPathResource() resource.Resource { return &bgpASPathResource{} }

func (r *bgpASPathResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_quagga_bgp_aspath"
}

func (r *bgpASPathResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *bgpASPathResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	bgpCreate[BGPASPathResourceModel, bgpASPathAPIResponse, bgpASPathAPIRequest](ctx, r.client, bgpASPathReqOpts, req, resp, func(m *BGPASPathResourceModel) *bgpASPathAPIRequest { return m.toAPI(ctx) }, func(m *BGPASPathResourceModel, a *bgpASPathAPIResponse, id string) { m.fromAPI(ctx, a, id) })
}

func (r *bgpASPathResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	bgpRead[BGPASPathResourceModel, bgpASPathAPIResponse](ctx, r.client, bgpASPathReqOpts, req, resp, func(m *BGPASPathResourceModel, a *bgpASPathAPIResponse, id string) { m.fromAPI(ctx, a, id) })
}

func (r *bgpASPathResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	bgpUpdate[BGPASPathResourceModel, bgpASPathAPIResponse, bgpASPathAPIRequest](ctx, r.client, bgpASPathReqOpts, req, resp, func(m *BGPASPathResourceModel) *bgpASPathAPIRequest { return m.toAPI(ctx) }, func(m *BGPASPathResourceModel, a *bgpASPathAPIResponse, id string) { m.fromAPI(ctx, a, id) })
}

func (r *bgpASPathResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	bgpDelete[BGPASPathResourceModel](ctx, r.client, bgpASPathReqOpts, req, resp)
}

func (r *bgpASPathResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
