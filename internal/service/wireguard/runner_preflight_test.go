// Copyright (c) Matthew Mellor
// SPDX-License-Identifier: MPL-2.0

package wireguard_test

import (
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestRunnerRejectsUnboundEndpointBeforeCredentials(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("runner requires Python TLS preflight")
	}
	var requests atomic.Int32
	endpoint := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requests.Add(1) }))
	defer endpoint.Close()
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	for _, fingerprint := range []string{"", "invalid", strings.Repeat("0", 64)} {
		t.Run(fmt.Sprintf("fingerprint_%d", len(fingerprint)), func(t *testing.T) {
			probe := filepath.Join(t.TempDir(), "identity-probe")
			body := "#!/usr/bin/env bash\nset -euo pipefail\nprintf '%s\\n' name=opnsense-acceptance-102 uuid=fixture-uuid tls_sha256=" + fingerprint + "\n"
			if err := os.WriteFile(probe, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			probeRoot, err := os.OpenRoot(filepath.Dir(probe))
			if err != nil {
				t.Fatal(err)
			}
			if err := probeRoot.Chmod("identity-probe", 0o700); err != nil {
				t.Fatal(err)
			}
			if err := probeRoot.Close(); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("bash", "test/scripts/run-story-31-0e-acceptance.sh")
			cmd.Dir = root
			cmd.Env = append(os.Environ(), "OPNSENSE_URI="+endpoint.URL, "OPNSENSE_API_KEY=never-send-fixture-key", "OPNSENSE_API_SECRET=never-send-fixture-secret", "OPNSENSE_ALLOW_INSECURE=true", "OPNSENSE_DISPOSABLE_VM_NAME=opnsense-acceptance-102", "OPNSENSE_DISPOSABLE_VM_UUID=fixture-uuid", "OPNSENSE_WIREGUARD_IDENTITY_PROBE="+probe, "OPNSENSE_WIREGUARD_EXPECTED_DIGEST=unused")
			output, err := cmd.CombinedOutput()
			if err == nil || !strings.Contains(string(output), "TLS fingerprint") {
				t.Fatalf("unexpected rejection %v: %s", err, output)
			}
			if requests.Load() != 0 {
				t.Fatal("unattested endpoint received HTTP traffic")
			}
		})
	}
}

func TestDigestIncludesTransitiveSourceAndLogging(t *testing.T) {
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	// Work in an isolated copy so the test never edits a concurrently reviewed tree.
	dir := t.TempDir()
	src, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = src.Close() }()
	dst, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = dst.Close() }()
	for _, sub := range []string{"internal", "pkg", "templates", "examples", "docs", "test", "_bmad-output"} {
		err = fs.WalkDir(src.FS(), sub, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return dst.MkdirAll(path, 0o700)
			}
			raw, err := src.ReadFile(path)
			if err != nil {
				return err
			}
			return dst.WriteFile(path, raw, 0o600)
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{".gitleaksignore", ".devrail.yml", "Makefile", "go.mod", "go.sum", "main.go"} {
		raw, err := src.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if err = dst.WriteFile(name, raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	digest := func() string {
		t.Helper()
		cmd := exec.Command("bash", "test/scripts/story-31-0e-code-digest.sh")
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("digest: %v %s", err, out)
		}
		return strings.TrimSpace(string(out))
	}
	before := digest()
	for _, name := range []string{"test/scripts/lib/log.sh", "internal/service/unbound/create_lock.go", "internal/provider/provider.go"} {
		f, err := dst.OpenFile(name, os.O_APPEND|os.O_WRONLY, 0)
		if err != nil {
			t.Fatal(err)
		}
		_, err = f.WriteString("\n// digest sensitivity fixture\n")
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		if err != nil {
			t.Fatal(err)
		}
		after := digest()
		if after == before {
			t.Fatalf("digest omitted %s", name)
		}
		before = after
	}
}

func TestRunnerSecretScanRejectsOperationalErrors(t *testing.T) {
	repo, err := os.OpenRoot("../../..")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = repo.Close() }()
	source, err := repo.ReadFile("test/scripts/run-story-31-0e-acceptance.sh")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	start := strings.Index(text, "scan_secret_artifacts() {")
	if start < 0 {
		t.Fatal("missing runner scanner")
	}
	end := strings.Index(text[start:], "\n}\n")
	if end < 0 {
		t.Fatal("unterminated runner scanner")
	}
	function := text[start : start+end+3]
	dir := t.TempDir()
	files, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = files.Close() }()
	if err = files.Mkdir("clean", 0o700); err != nil {
		t.Fatal(err)
	}
	if err = files.Mkdir("match", 0o700); err != nil {
		t.Fatal(err)
	}
	if err = files.WriteFile("clean/package.log", []byte("public fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err = files.WriteFile("match/package.log", []byte("fixture-api-key"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []string{"clean", "match", "missing"} {
		t.Run(scenario, func(t *testing.T) {
			command := exec.Command("bash", "-s")
			command.Env = append(os.Environ(), "OPNSENSE_API_KEY=fixture-api-key", "OPNSENSE_API_SECRET=fixture-api-secret", "OPNSENSE_WIREGUARD_PRIVATE_KEY=fixture-private-key", "OPNSENSE_WIREGUARD_PSK=fixture-psk", "SCAN_FIXTURE_PATH="+filepath.Join(dir, scenario))
			command.Stdin = strings.NewReader("set -euo pipefail\ndie() { printf '%s\\n' \"$*\" >&2; exit 1; }\n" + function + "\nscan_secret_artifacts \"${SCAN_FIXTURE_PATH}\"\nprintf '%s\\n' RAW_ARTIFACT secret_scan=pass\n")
			output, err := command.CombinedOutput()
			if scenario == "clean" {
				if err != nil || !strings.Contains(string(output), "secret_scan=pass") {
					t.Fatal("clean grep exit1 rejected", err)
				}
				return
			}
			if err == nil || strings.Contains(string(output), "RAW_ARTIFACT") || strings.Contains(string(output), "secret_scan=pass") {
				t.Fatal("match/operational failure released raw artifacts")
			}
			if scenario == "missing" && !strings.Contains(string(output), "operational error") {
				t.Fatal("missing operational error diagnostic")
			}
		})
	}
}
