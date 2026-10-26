"""Constrain the public DDNS fixture to local oracle and single pinned upstream."""
import unittest
from pathlib import Path

from kea_ddns_pinned_proxy import ddns_proxy_handler


class DdnsProxyContractTest(unittest.TestCase):
    def test_upstream_validation_reuses_connection_pinned_primitive(self):
        for uri in ("http://fixture.invalid", "https://fixture.invalid/path", "https://user:secret@fixture.invalid", "https://fixture.invalid?query=1"):
            with self.subTest(uri=uri), self.assertRaises(ValueError):
                ddns_proxy_handler(uri, "0" * 64)

    def test_proxy_has_own_fixed_port_and_runtime_names(self):
        source = Path(__file__).with_name("kea_ddns_pinned_proxy.py").read_text()
        self.assertIn('("127.0.0.1", 19536)', source)
        self.assertIn('STORY31_0O_UPSTREAM_URI', source)
        self.assertIn('STORY31_0O_TLS_SHA256', source)
        self.assertNotIn('STORY31_0N_', source)


if __name__ == "__main__":
    unittest.main()
