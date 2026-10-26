#!/usr/bin/env python3
"""Acceptance-only fixed-upstream proxy: verify the pin before sending HTTP bytes."""
import hashlib
import http.client
import http.server
import os
import ssl
import urllib.parse
from pathlib import Path


class PinnedHTTPSConnection(http.client.HTTPSConnection):
    def __init__(self, host, port, expected_pin):
        # The independently attested certificate digest replaces CA/hostname
        # identity here. connect() MUST verify it before http.client sends headers.
        context = ssl.SSLContext(ssl.PROTOCOL_TLS_CLIENT)
        context.check_hostname = False
        context.verify_mode = ssl.CERT_NONE
        super().__init__(host, port, context=context, timeout=30)
        self.expected_pin = expected_pin

    def connect(self):
        super().connect()
        observed = hashlib.sha256(self.sock.getpeercert(binary_form=True)).hexdigest()
        if observed != self.expected_pin:
            self.close()
            raise ssl.SSLError("acceptance upstream certificate pin mismatch")


def proxy_handler(upstream_uri, pin):
    upstream = urllib.parse.urlsplit(upstream_uri)
    if upstream.scheme != "https" or not upstream.hostname or upstream.username or upstream.password:
        raise ValueError("fixed upstream must be credential-free HTTPS")
    if len(pin) != 64 or any(character not in "0123456789abcdef" for character in pin):
        raise ValueError("missing independently attested certificate pin")

    class Handler(http.server.BaseHTTPRequestHandler):
        def log_message(self, *_args):
            pass  # Never log request headers, credentials or bodies.

        def forward(self):
            if not self.path.startswith("/api/") or self.path.startswith("//"):
                self.send_error(400, "only fixed-upstream API paths are accepted")
                return
            connection = PinnedHTTPSConnection(upstream.hostname, upstream.port or 443, pin)
            try:
                length = int(self.headers.get("Content-Length", "0"))
                if length < 0 or length > 1024 * 1024:
                    raise ValueError("invalid request length")
                body = self.rfile.read(length)
                headers = {key: value for key, value in self.headers.items()
                           if key.lower() not in ("host", "connection", "transfer-encoding")}
                # HTTPSConnection.connect verifies the current peer certificate
                # BEFORE request() can send Authorization or any HTTP bytes.
                connection.request(self.command, self.path, body=body, headers=headers)
                response = connection.getresponse()
                if 300 <= response.status < 400:
                    raise ValueError("upstream redirects are forbidden")
                response_body = response.read()
                self.send_response(response.status)
                self.send_header("Content-Type", response.getheader("Content-Type", "application/json"))
                self.send_header("Content-Length", str(len(response_body)))
                self.end_headers()
                self.wfile.write(response_body)
            except (OSError, ValueError, http.client.HTTPException):
                self.send_error(502, "acceptance upstream connection or certificate verification failed")
            finally:
                connection.close()

        do_GET = forward
        do_POST = forward

    return Handler


def main():
    handler = proxy_handler(os.environ["STORY31_0M_UPSTREAM_URI"], os.environ["STORY31_0M_TLS_SHA256"])
    server = http.server.ThreadingHTTPServer(("127.0.0.1", 19532), handler)
    Path(os.environ["STORY31_0M_EVIDENCE_DIR"], "proxy.ready").touch(mode=0o600)
    server.serve_forever()


if __name__ == "__main__":
    main()
