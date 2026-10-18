// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

// Command render writes the deterministic human-readable coverage audit.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/matthew-on-git/terraform-provider-opnsense/internal/coverageaudit"
)

func main() {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		panic(err)
	}
	audit, err := coverageaudit.Load(root)
	if err != nil {
		panic(err)
	}
	path := filepath.Join(root, coverageaudit.ReportPath)
	if err := os.WriteFile(path, audit.Render(), 0o600); err != nil {
		panic(err)
	}
	fmt.Printf("generated %s\n", coverageaudit.ReportPath)
}
