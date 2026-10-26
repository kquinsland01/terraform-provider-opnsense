// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package kea_test

import (
	"os/exec"
	"testing"
)

// The project normally enables Go only: explicitly register acceptance helper
// security regressions so `go test ./...` and `make check` cannot orphan them.
func TestDHCPv4OptionAcceptanceHelperSecurity(t *testing.T) {
	command := exec.Command("python3", "-m", "unittest", "discover", "-s", "../../../test/scripts", "-p", "*_test.py")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("acceptance helper security: %v\n%s", err, output)
	}
}

func TestDHCPv4OptionAcceptanceShellAndExampleChecks(t *testing.T) {
	for _, check := range []struct {
		name    string
		command *exec.Cmd
	}{
		{"shellcheck", exec.Command("shellcheck", "-x", "test/scripts/run-story-31-0n-acceptance.sh", "test/scripts/run-story-31-0n-container.sh", "test/scripts/story-31-0n-code-digest.sh")},
		{"shfmt", exec.Command("shfmt", "-d", "test/scripts/run-story-31-0n-acceptance.sh", "test/scripts/run-story-31-0n-container.sh", "test/scripts/story-31-0n-code-digest.sh")},
		{"terraform", exec.Command("terraform", "fmt", "-check", "examples/resources/opnsense_kea_dhcpv4_option", "examples/data-sources/opnsense_kea_dhcpv4_option")},
	} {
		t.Run(check.name, func(t *testing.T) {
			check.command.Dir = "../../.."
			if output, err := check.command.CombinedOutput(); err != nil {
				t.Fatalf("%s: %v\n%s", check.name, err, output)
			}
		})
	}
}
