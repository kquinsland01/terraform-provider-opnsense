"""Local-only acceptance transport regression; never contacts an appliance."""
import hashlib
import http.client
import http.server
import importlib.util
import ssl
import subprocess
import tempfile
import threading
import unittest
from pathlib import Path

SPEC = importlib.util.spec_from_file_location("pinned_proxy", Path(__file__).with_name("gateway_group_pinned_proxy.py"))
PROXY = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(PROXY)


class PinTest(unittest.TestCase):
    def test_changed_certificate_refused_before_credentials(self):
        with tempfile.TemporaryDirectory() as directory:
            paths = []
            for index in range(2):
                cert, key = Path(directory, f"cert{index}.pem"), Path(directory, f"key{index}.pem")
                subprocess.run(["openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes", "-days", "1",
                                "-subj", "/CN=localhost", "-keyout", str(key), "-out", str(cert)],
                               check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
                paths.append((cert, key))
            seen = []

            class Upstream(http.server.BaseHTTPRequestHandler):
                def do_GET(self):
                    seen.append(self.headers.get("Authorization"))
                    self.send_response(200)
                    self.end_headers()
                    self.wfile.write(b"{}")

                def log_message(self, *_args):
                    pass

            server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Upstream)
            context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
            context.load_cert_chain(*paths[0])
            server.socket = context.wrap_socket(server.socket, server_side=True)
            threading.Thread(target=server.serve_forever, daemon=True).start()
            der = ssl.PEM_cert_to_DER_cert(paths[0][0].read_text())
            pin = hashlib.sha256(der).hexdigest()
            proxy = http.server.ThreadingHTTPServer(("127.0.0.1", 0), PROXY.proxy_handler(
                f"https://127.0.0.1:{server.server_port}", pin))
            threading.Thread(target=proxy.serve_forever, daemon=True).start()
            try:
                for expected in (200, 502):
                    client = http.client.HTTPConnection("127.0.0.1", proxy.server_port)
                    client.request("GET", "/api/test", headers={"Authorization": "Basic fixture-only"})
                    response = client.getresponse()
                    self.assertEqual(response.status, expected)
                    response.read()
                    client.close()
                    self.assertEqual(seen, ["Basic fixture-only"])
                    context.load_cert_chain(*paths[1])
            finally:
                proxy.shutdown()
                proxy.server_close()
                server.shutdown()
                server.server_close()


if __name__ == "__main__":
    unittest.main()
