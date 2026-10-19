// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package unbound_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/matthew-on-git/terraform-provider-opnsense/internal/acctest"
	"github.com/matthew-on-git/terraform-provider-opnsense/pkg/opnsense"
)

func TestAccUnboundDomainOverride_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		PreCheck:                 func() { acctest.PreCheck(t) },
		CheckDestroy:             acctest.CheckResourceDestroyed(t, "opnsense_unbound_domain_override", opnsense.ReqOpts{GetEndpoint: "/api/unbound/settings/get_forward", Monad: "dot"}),
		Steps: []resource.TestStep{
			// Step 1: Create and verify.
			{
				Config: testAccUnboundDomainOverrideConfig("example.com", "query", "10.0.0.1", 53, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("opnsense_unbound_domain_override.test", "id"),
					resource.TestCheckResourceAttr("opnsense_unbound_domain_override.test", "domain", "example.com"),
					resource.TestCheckResourceAttr("opnsense_unbound_domain_override.test", "server", "10.0.0.1"),
					resource.TestCheckResourceAttr("opnsense_unbound_domain_override.test", "type", "query"),
					resource.TestCheckResourceAttr("opnsense_unbound_domain_override.test", "server_port", "53"),
					resource.TestCheckResourceAttr("opnsense_unbound_domain_override.test", "enabled", "true"),
				),
			},
			// Step 2: Import and verify state matches.
			{
				ResourceName:      "opnsense_unbound_domain_override.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Step 3: Change endpoint family in place and verify DoT state.
			{
				Config: testAccUnboundDomainOverrideConfig("example.com", "dot", "10.0.0.2", 853, "dns.example.com"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("opnsense_unbound_domain_override.test", "domain", "example.com"),
					resource.TestCheckResourceAttr("opnsense_unbound_domain_override.test", "server", "10.0.0.2"),
					resource.TestCheckResourceAttr("opnsense_unbound_domain_override.test", "type", "dot"),
					resource.TestCheckResourceAttr("opnsense_unbound_domain_override.test", "server_port", "853"),
					resource.TestCheckResourceAttr("opnsense_unbound_domain_override.test", "verify_cn", "dns.example.com"),
				),
			},
			// Step 4: Blank CN is a valid explicit insecure choice.
			{
				Config: testAccUnboundDomainOverrideConfig("example.com", "dot", "10.0.0.2", 853, ""),
				Check:  resource.TestCheckResourceAttr("opnsense_unbound_domain_override.test", "verify_cn", ""),
			},
		},
	})
}

func testAccUnboundDomainOverrideConfig(domain, forwardType, server string, port int, verifyCN string) string {
	return fmt.Sprintf(`
resource "opnsense_unbound_domain_override" "test" {
  domain = %[1]q
  type = %[2]q
  server = %[3]q
  server_port = %[4]d
  verify_cn = %[5]q
}
`, domain, forwardType, server, port, verifyCN)
}
