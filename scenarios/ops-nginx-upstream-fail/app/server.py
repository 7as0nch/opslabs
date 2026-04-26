#!/usr/bin/env python3
"""极简后端 app —— 听 PORT 环境变量(默认 8080),返回固定文本"""
import os
import sys
from http.server import BaseHTTPRequestHandler, HTTPServer


class Handler(BaseHTTPRequestHandler):
    def do_GET(self):  # noqa: N802
        body = b'Hello from app\n'
        self.send_response(200)
        self.send_header('Content-Type', 'text/plain; charset=utf-8')
        self.send_header('Content-Length', str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, fmt: str, *args):  # noqa: N802
        # 把 access log 也打到 stderr,journalctl 能直接看到
        sys.stderr.write('app %s - - %s\n' % (self.address_string(), fmt % args))


def main():
    port = int(os.environ.get('PORT', '8080'))
    HTTPServer(('127.0.0.1', port), Handler).serve_forever()


if __name__ == '__main__':
    main()
