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
	_ resource.Resource                = &routeMapResource{}
	_ resource.ResourceWithImportState = &routeMapResource{}
)

var routeMapReqOpts = opnsense.ReqOpts{
	AddEndpoint:         "/api/quagga/bgp/add_routemap",
	GetEndpoint:         "/api/quagga/bgp/get_routemap",
	UpdateEndpoint:      "/api/quagga/bgp/set_routemap",
	DeleteEndpoint:      "/api/quagga/bgp/del_routemap",
	SearchEndpoint:      "/api/quagga/bgp/search_routemap",
	ReconfigureEndpoint: "/api/quagga/service/reconfigure",
	Monad:               "routemap",
}

type routeMapResource struct{ client *opnsense.Client }

func newRouteMapResource() resource.Resource { return &routeMapResource{} }

func (r *routeMapResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_quagga_route_map"
}

func (r *routeMapResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *routeMapResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	bgpCreate[RouteMapResourceModel, routeMapAPIResponse, routeMapAPIRequest](ctx, r.client, routeMapReqOpts, req, resp, func(m *RouteMapResourceModel) *routeMapAPIRequest { return m.toAPI(ctx) }, func(m *RouteMapResourceModel, a *routeMapAPIResponse, id string) { m.fromAPI(ctx, a, id) })
}

func (r *routeMapResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	bgpRead[RouteMapResourceModel, routeMapAPIResponse](ctx, r.client, routeMapReqOpts, req, resp, func(m *RouteMapResourceModel, a *routeMapAPIResponse, id string) { m.fromAPI(ctx, a, id) })
}

func (r *routeMapResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	bgpUpdate[RouteMapResourceModel, routeMapAPIResponse, routeMapAPIRequest](ctx, r.client, routeMapReqOpts, req, resp, func(m *RouteMapResourceModel) *routeMapAPIRequest { return m.toAPI(ctx) }, func(m *RouteMapResourceModel, a *routeMapAPIResponse, id string) { m.fromAPI(ctx, a, id) })
}

func (r *routeMapResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	bgpDelete[RouteMapResourceModel](ctx, r.client, routeMapReqOpts, req, resp)
}

func (r *routeMapResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
