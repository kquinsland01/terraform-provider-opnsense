#!/usr/bin/env python3
"""DDNS acceptance entrypoint using the tested per-connection TLS-pin primitive."""
import http.server
import os
import urllib.parse
from pathlib import Path

from gateway_group_pinned_proxy import proxy_handler


def ddns_proxy_handler(upstream_uri, pin):
    upstream = urllib.parse.urlsplit(upstream_uri)
    if upstream.path not in ("", "/") or upstream.query or upstream.fragment:
        raise ValueError("DDNS acceptance requires a fixed HTTPS origin, not a path/query/fragment")
    return proxy_handler(upstream_uri, pin)


def main():
    handler = ddns_proxy_handler(os.environ["STORY31_0O_UPSTREAM_URI"], os.environ["STORY31_0O_TLS_SHA256"])
    server = http.server.ThreadingHTTPServer(("127.0.0.1", 19536), handler)
    Path(os.environ["STORY31_0O_EVIDENCE_DIR"], "proxy.ready").touch(mode=0o600)
    server.serve_forever()


if __name__ == "__main__":
    main()
