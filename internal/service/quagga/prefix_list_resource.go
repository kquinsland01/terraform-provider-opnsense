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
	_ resource.Resource                = &prefixListResource{}
	_ resource.ResourceWithImportState = &prefixListResource{}
)

var prefixListReqOpts = opnsense.ReqOpts{
	AddEndpoint:         "/api/quagga/bgp/add_prefixlist",
	GetEndpoint:         "/api/quagga/bgp/get_prefixlist",
	UpdateEndpoint:      "/api/quagga/bgp/set_prefixlist",
	DeleteEndpoint:      "/api/quagga/bgp/del_prefixlist",
	SearchEndpoint:      "/api/quagga/bgp/search_prefixlist",
	ReconfigureEndpoint: "/api/quagga/service/reconfigure",
	Monad:               "prefixlist",
}

type prefixListResource struct{ client *opnsense.Client }

func newPrefixListResource() resource.Resource { return &prefixListResource{} }

func (r *prefixListResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_quagga_prefix_list"
}

func (r *prefixListResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *prefixListResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	bgpCreate[PrefixListResourceModel, prefixListAPIResponse, prefixListAPIRequest](ctx, r.client, prefixListReqOpts, req, resp, func(m *PrefixListResourceModel) *prefixListAPIRequest { return m.toAPI(ctx) }, func(m *PrefixListResourceModel, a *prefixListAPIResponse, id string) { m.fromAPI(ctx, a, id) })
}

func (r *prefixListResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	bgpRead[PrefixListResourceModel, prefixListAPIResponse](ctx, r.client, prefixListReqOpts, req, resp, func(m *PrefixListResourceModel, a *prefixListAPIResponse, id string) { m.fromAPI(ctx, a, id) })
}

func (r *prefixListResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	bgpUpdate[PrefixListResourceModel, prefixListAPIResponse, prefixListAPIRequest](ctx, r.client, prefixListReqOpts, req, resp, func(m *PrefixListResourceModel) *prefixListAPIRequest { return m.toAPI(ctx) }, func(m *PrefixListResourceModel, a *prefixListAPIResponse, id string) { m.fromAPI(ctx, a, id) })
}

func (r *prefixListResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	bgpDelete[PrefixListResourceModel](ctx, r.client, prefixListReqOpts, req, resp)
}

func (r *prefixListResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
