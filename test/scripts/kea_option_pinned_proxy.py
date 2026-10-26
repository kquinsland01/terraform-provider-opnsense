#!/usr/bin/env python3
"""Kea acceptance entrypoint using the tested per-connection TLS-pin primitive."""
import http.server
import os
import json
import re
import threading
from pathlib import Path

from gateway_group_pinned_proxy import proxy_handler


_DELETE_PATH = re.compile(r"^/api/kea/dhcpv4/del_option/[0-9a-fA-F]{8}(?:-[0-9a-fA-F]{4}){3}-[0-9a-fA-F]{12}$")
_STATUS_LOCK = threading.Lock()


def kea_proxy_handler(upstream_uri, pin, evidence_file):
    base = proxy_handler(upstream_uri, pin)

    class Handler(base):
        def send_response(self, code, message=None):
            if self.command == "POST" and _DELETE_PATH.fullmatch(self.path):
                # Deliberately exclude headers, credentials, labels and bodies.
                record = {"method": self.command, "path": self.path, "status": code}
                with _STATUS_LOCK, Path(evidence_file).open("a", encoding="utf-8") as stream:
                    stream.write(json.dumps(record) + "\n")
            super().send_response(code, message)

    return Handler


def main():
    handler = kea_proxy_handler(os.environ["STORY31_0N_UPSTREAM_URI"], os.environ["STORY31_0N_TLS_SHA256"], Path(os.environ["STORY31_0N_EVIDENCE_DIR"], "option-delete-status.jsonl"))
    server = http.server.ThreadingHTTPServer(("127.0.0.1", 19534), handler)
    Path(os.environ["STORY31_0N_EVIDENCE_DIR"], "proxy.ready").touch(mode=0o600)
    server.serve_forever()


if __name__ == "__main__":
    main()
