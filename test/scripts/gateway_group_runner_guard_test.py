"""Runner hash failures must occur before any private helper execution."""
import hashlib
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
                values = dict(STORY31_0M_GUARD=str(helpers[0]), STORY31_0M_ORACLE=str(helpers[1]),
                              STORY31_0M_PRIVATE_ROOT=directory, STORY31_0M_EXPECTED_DIGEST="0" * 64,
                              STORY31_0M_ORACLE_URL="http://127.0.0.1:19531", STORY31_0M_ORACLE_TOKEN="fixture",
                              OPNSENSE_URI="https://fixture.invalid", OPNSENSE_API_KEY="fixture",
                              OPNSENSE_API_SECRET="fixture", OPNSENSE_ALLOW_INSECURE="true",
                              OPNSENSE_GATEWAY_GROUP_INTERFACE="fixture",
                              OPNSENSE_GATEWAY_GROUP_ADDRESS_A="192.0.2.1",
                              OPNSENSE_GATEWAY_GROUP_ADDRESS_B="192.0.2.2")
                if mode != "missing":
                    values["STORY31_0M_EXPECTED_HELPER_DIGEST"] = expected if mode == "changed" else "0" * 64
                runtime = root / "runtime.env"
                runtime.write_text("\n".join(f"{key}={shlex.quote(value)}" for key, value in values.items()))
                runner = Path(__file__).with_name("run-story-31-0m-acceptance.sh")
                result = subprocess.run(["bash", str(runner)], capture_output=True,
                                        env={"PATH": os.environ["PATH"], "STORY31_0M_RUNTIME_ENV": str(runtime)})
                self.assertNotEqual(result.returncode, 0)
                self.assertFalse(marker.exists(), "unreviewed private helper executed")


if __name__ == "__main__":
    unittest.main()
