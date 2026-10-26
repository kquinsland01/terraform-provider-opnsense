"""Runner hash failures must occur before any private helper execution."""
import hashlib
import json
import os
import shlex
import subprocess
import tempfile
import unittest
from pathlib import Path


class HelperGuardTest(unittest.TestCase):
    def test_missing_or_changed_expected_helper_hash_never_executes_helpers(self):
        for mode in ("missing", "mismatch", "changed"):
            with self.subTest(mode=mode), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                marker = root / "executed"
                helpers = [root / "guard", root / "oracle"]
                for helper in helpers:
                    helper.write_text(f"#!/bin/sh\nprintf executed > {shlex.quote(str(marker))}\n")
                    helper.chmod(0o700)
                encoded = subprocess.run(["shasum", "-a", "256", *map(str, helpers)],
                                         check=True, capture_output=True).stdout
                expected = hashlib.sha256(encoded).hexdigest()
                if mode == "changed":
                    helpers[0].write_text(helpers[0].read_text() + "# changed after review\n")
                values = dict(STORY31_0O_EXPECTED_ORACLE_DIGEST="0" * 64, STORY31_0O_GUARD=str(helpers[0]), STORY31_0O_ORACLE=str(helpers[1]),
                              STORY31_0O_PRIVATE_ROOT=directory, STORY31_0O_EXPECTED_DIGEST="0" * 64,
                              STORY31_0O_ORACLE_URL="http://127.0.0.1:19535", STORY31_0O_ORACLE_TOKEN="fixture",
                              OPNSENSE_URI="https://fixture.invalid", OPNSENSE_API_KEY="fixture",
                              OPNSENSE_API_SECRET="fixture", OPNSENSE_ALLOW_INSECURE="true")
                if mode != "missing":
                    values["STORY31_0O_EXPECTED_HELPER_DIGEST"] = expected if mode == "changed" else "0" * 64
                runtime = root / "runtime.env"
                runtime.write_text("\n".join(f"{key}={shlex.quote(value)}" for key, value in values.items()))
                runner = Path(__file__).with_name("run-story-31-0o-acceptance.sh")
                result = subprocess.run(["bash", str(runner)], capture_output=True,
                                        env={"PATH": os.environ["PATH"], "STORY31_0O_RUNTIME_ENV": str(runtime)})
                self.assertNotEqual(result.returncode, 0)
                self.assertFalse(marker.exists(), "unreviewed private helper executed")

    def test_changed_during_core_never_executes_changed_cleanup_helper(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            marker = root / "changed-helper-executed"
            actions = root / "actions"
            guard, oracle, docker = root / "guard", root / "oracle", root / "docker"
            attestation = dict(ok=True, identity_verified=True, tls_verified=True,
                               version_verified=True, isolated=True, tls_sha256="0" * 64,
                               server_sha256="0" * 64, core_version="26.7.7",
                               config_sha256="0" * 64, routes_sha256="0" * 64,
                               runtime_sha256="0" * 64, services_inactive=True,
                               unrelated_kea_equal=True)
            guard.write_text("#!/usr/bin/env python3\nimport json\nprint(json.dumps(" + repr(attestation) + "))\n")
            oracle.write_text("#!/usr/bin/env python3\nimport json,sys\nfrom pathlib import Path\n"
                              + f"with Path({str(actions)!r}).open('a') as stream: stream.write(sys.argv[1]+'\\n')\n"
                              + "print(json.dumps(" + repr(attestation) + "))\n")
            changed_source = f"#!/bin/sh\nprintf executed > {shlex.quote(str(marker))}\n"
            docker.write_text("#!/usr/bin/env python3\nimport sys\nfrom pathlib import Path\n"
                              + "if 'run-story-31-0o-container.sh' in ' '.join(sys.argv):\n"
                              + f"    Path({str(oracle)!r}).write_text({changed_source!r})\n"
                              + "    raise SystemExit(1)\nprint('0'*64)\n")
            for helper in (guard, oracle, docker):
                helper.chmod(0o700)
            encoded = subprocess.run(["shasum", "-a", "256", str(guard), str(oracle)],
                                     check=True, capture_output=True).stdout
            values = dict(STORY31_0O_GUARD=str(guard), STORY31_0O_ORACLE=str(oracle),
                          STORY31_0O_EXPECTED_HELPER_DIGEST=hashlib.sha256(encoded).hexdigest(),
                          STORY31_0O_EXPECTED_ORACLE_DIGEST="0" * 64,
                          STORY31_0O_EXPECTED_DIGEST="0" * 64, STORY31_0O_PRIVATE_ROOT=directory,
                          STORY31_0O_ORACLE_URL="http://127.0.0.1:19535", STORY31_0O_ORACLE_TOKEN="fixture",
                          OPNSENSE_URI="https://fixture.invalid", OPNSENSE_API_KEY="fixture",
                          OPNSENSE_API_SECRET="fixture", OPNSENSE_ALLOW_INSECURE="true")
            runtime = root / "runtime.env"
            runtime.write_text("\n".join(f"{key}={shlex.quote(value)}" for key, value in values.items()))
            runner = Path(__file__).with_name("run-story-31-0o-acceptance.sh")
            result = subprocess.run(["bash", str(runner)], capture_output=True,
                                    env={"PATH": directory + os.pathsep + os.environ["PATH"],
                                         "STORY31_0O_RUNTIME_ENV": str(runtime)})
            self.assertNotEqual(result.returncode, 0)
            self.assertFalse(marker.exists(), "changed privileged cleanup helper executed")
            self.assertEqual(actions.read_text().splitlines(), ["snapshot"])
            self.assertIn(b"helper integrity mismatch", result.stderr)


class DdnsObservedInputTest(unittest.TestCase):
    def test_source_symlinks_fail_closed(self):
        script = Path(__file__).with_name("story-31-0o-code-digest.sh")
        for entry in ("internal/linked.go", "docs/linked", ".devrail.yml"):
            with self.subTest(entry=entry), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                for folder in ("internal", "pkg", "tools", "templates", "examples", "docs", "test/scripts", ".github"):
                    (root / folder).mkdir(parents=True, exist_ok=True)
                copied = root / "test/scripts/story-31-0o-code-digest.sh"
                copied.write_text(script.read_text())
                target = root / "uncovered-target"
                target.write_text("fixture")
                (root / entry).symlink_to(target)
                result = subprocess.run(["bash", str(copied)], capture_output=True)
                self.assertNotEqual(result.returncode, 0)
                self.assertIn(b"rejects symlink inputs", result.stderr)
                self.assertEqual(result.stdout, b"")

    def test_each_runner_oracle_response_requires_source_attestation(self):
        runner = Path(__file__).with_name("run-story-31-0o-acceptance.sh").read_text()
        validator = runner.split("  python3 - \"$1\" \"$2\" <<'PY'\n", 1)[1].split("\nPY\n", 1)[0]
        for action in ("guard", "snapshot", "cleanup", "verify-zero", "restore", "verify-restored"):
            for source in (None, "b" * 64, "a" * 64):
                with self.subTest(action=action, source=source), tempfile.TemporaryDirectory() as directory:
                    response = dict(ok=True, identity_verified=True, tls_verified=True,
                                    version_verified=True, isolated=True, tls_sha256="0" * 64,
                                    core_version="26.7.7", config_sha256="0" * 64,
                                    routes_sha256="0" * 64, runtime_sha256="0" * 64,
                                    services_inactive=True, unrelated_kea_equal=True,
                                    config_equal=True, settings_equal=True, routes_equal=True,
                                    runtime_equal=True)
                    if source is not None:
                        response["server_sha256"] = source
                    path = Path(directory) / "response.json"
                    path.write_text(json.dumps(response))
                    result = subprocess.run(["python3", "-", action, str(path)], input=validator.encode(),
                                            capture_output=True, env={**os.environ, "STORY31_0O_EXPECTED_ORACLE_DIGEST": "a" * 64})
                    self.assertEqual(result.returncode == 0, source == "a" * 64)


if __name__ == "__main__":
    unittest.main()
