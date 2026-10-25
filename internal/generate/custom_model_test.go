// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCustomModelAndSchemaSurviveGeneration(t *testing.T) {
	t.Chdir(t.TempDir())
	dir := filepath.Join("internal", "service", "fixture")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"model", "schema"} {
		file := filepath.Join(dir, "settings_"+kind+".go")
		if err := os.WriteFile(file, []byte("handwritten fixture"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	r := &Resource{Name: "settings", GoType: "Settings", TypeName: "fixture_settings", Kind: "singleton", ID: "fixture", CustomModel: true, CustomSchema: true, TestPreCheck: "acctest.PreCheck(t)", Fields: []Field{{Name: "Enabled", TF: "enabled", JSON: "enabled", Type: "bool"}}}
	for range 2 {
		if err := generateResource("fixture", r); err != nil {
			t.Fatal(err)
		}
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()
	for _, kind := range []string{"model", "schema"} {
		content, err := root.ReadFile("settings_" + kind + ".go")
		if err != nil || string(content) != "handwritten fixture" {
			t.Fatal("handwritten file changed", err)
		}
		if _, err = os.Stat(filepath.Join(dir, "settings_"+kind+".gen.go")); !os.IsNotExist(err) {
			t.Fatal("obsolete generated file created", err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "settings_resource.gen.go")); err != nil {
		t.Fatal("lifecycle generation disabled", err)
	}
}
