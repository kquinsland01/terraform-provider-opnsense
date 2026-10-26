"""Kea-only status evidence excludes all request credentials and bodies."""
import http.client
import http.server
import json
import tempfile
import threading
import unittest
from pathlib import Path
from unittest.mock import patch

import kea_option_pinned_proxy as proxy


class StatusEvidenceTest(unittest.TestCase):
    def test_exact_delete_status_only(self):
        class Base(http.server.BaseHTTPRequestHandler):
            def do_POST(self):
                self.send_response(500)
                self.end_headers()

            def log_message(self, *_args):
                pass

        with tempfile.TemporaryDirectory() as directory:
            evidence = Path(directory, "status.jsonl")
            with patch.object(proxy, "proxy_handler", return_value=Base):
                handler = proxy.kea_proxy_handler("unused", "unused", evidence)
            server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), handler)
            threading.Thread(target=server.serve_forever, daemon=True).start()
            path = "/api/kea/dhcpv4/del_option/00000000-0000-0000-0000-000000000031"
            try:
                for target in ("/api/kea/dhcpv4/search_option", path):
                    client = http.client.HTTPConnection("127.0.0.1", server.server_port)
                    client.request("POST", target, body="private body", headers={"Authorization": "private credential"})
                    response = client.getresponse()
                    response.read()
                    client.close()
                self.assertEqual([json.loads(line) for line in evidence.read_text().splitlines()],
                                 [{"method": "POST", "path": path, "status": 500}])
            finally:
                server.shutdown()
                server.server_close()


if __name__ == "__main__":
    unittest.main()
