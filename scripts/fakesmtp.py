"""Minimal SMTP sink for the smoke test.

Answers just enough of RFC 5321 for services/smtpSend to complete a plain
(non-TLS, non-AUTH) delivery, and writes every message to <outdir>/mailNNN.eml.
STARTTLS is refused on purpose so the client falls back to plain text.

Usage: python fakesmtp.py <outdir> [port]
"""
import os
import socket
import sys
import threading

OUT = sys.argv[1] if len(sys.argv) > 1 else "bin/maildrop"
PORT = int(sys.argv[2]) if len(sys.argv) > 2 else 2525
os.makedirs(OUT, exist_ok=True)
counter = [0]


def handle(conn):
    f = conn.makefile("rwb")

    def send(line):
        f.write((line + "\r\n").encode())
        f.flush()

    send("220 localhost fake smtp")
    while True:
        raw = f.readline()
        if not raw:
            break
        cmd = raw.decode("utf-8", "replace").strip()
        up = cmd.upper()
        if up.startswith("EHLO") or up.startswith("HELO"):
            send("250-localhost")
            send("250 OK")
        elif up.startswith("STARTTLS"):
            send("502 not implemented")
        elif up.startswith("AUTH"):
            send("503 no auth here")
        elif up.startswith("MAIL FROM"):
            send("250 OK")
        elif up.startswith("RCPT TO"):
            send("250 OK")
        elif up == "DATA":
            send("354 end with <CRLF>.<CRLF>")
            body = []
            while True:
                line = f.readline()
                if not line or line.strip() == b".":
                    break
                body.append(line)
            counter[0] += 1
            path = os.path.join(OUT, "mail%03d.eml" % counter[0])
            with open(path, "wb") as fh:
                fh.write(b"".join(body))
            send("250 OK queued as %03d" % counter[0])
        elif up == "QUIT":
            send("221 bye")
            break
        else:
            send("250 OK")
    conn.close()


srv = socket.socket()
srv.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
srv.bind(("127.0.0.1", PORT))
srv.listen(8)
print("fake smtp listening on %d -> %s" % (PORT, OUT), flush=True)
while True:
    c, _ = srv.accept()
    threading.Thread(target=handle, args=(c,), daemon=True).start()
