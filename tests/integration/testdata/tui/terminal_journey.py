"""Exercise the actual CLI/Bubble Tea loop with isolated synthetic dependencies."""
import fcntl
import json
import os
import pty
import select
import signal
import struct
import subprocess
import sys
import tempfile
import termios
import time

binary = sys.argv[1]

class Terminal:
    def __init__(self, setup=False, args=None, block_write=False):
        self.tmp = tempfile.TemporaryDirectory()
        self.log = os.path.join(self.tmp.name, "events.jsonl")
        self.master, self.slave = pty.openpty()
        fcntl.ioctl(self.slave, termios.TIOCSWINSZ, struct.pack("HHHH", 40, 120, 0, 0))
        self.before = termios.tcgetattr(self.slave)
        self.output = b""
        env = dict(os.environ, TERM="xterm-256color", NO_COLOR="1", JFLOW_CONFIG=os.path.join(self.tmp.name, "config.json"), JFLOW_PTY_LOG=self.log, JFLOW_PTY_SETUP="1" if setup else "", JFLOW_PTY_BLOCK_WRITE="1" if block_write else "")
        self.p = subprocess.Popen([binary] + (args or []), stdin=self.slave, stdout=self.slave, stderr=self.slave, env=env)
    def read(self, seconds=0.1):
        end = time.monotonic() + seconds
        while time.monotonic() < end:
            if select.select([self.master], [], [], 0.05)[0]:
                data = os.read(self.master, 65536)
                self.output += data
                if b"\x1b[6n" in data:
                    os.write(self.master, b"\x1b[1;1R")
    def wait(self, text, start=0):
        end = time.monotonic() + 10
        while text not in self.output[start:]:
            self.read()
            if time.monotonic() > end:
                raise AssertionError((text, self.output))
    def send(self, value):
        start = len(self.output)
        os.write(self.master, value)
        return start
    def events(self):
        if not os.path.exists(self.log): return []
        with open(self.log) as f: return [json.loads(line) for line in f]
    def finish(self, code=0, alternate=True):
        # Keep acting as a terminal while the child flushes its final render and
        # restores termios. Waiting without draining can fill the PTY buffer and
        # block shutdown (especially a canceled write's full-screen result).
        deadline = time.monotonic() + 10
        while self.p.poll() is None:
            self.read()
            if time.monotonic() > deadline:
                raise AssertionError(("terminal shutdown timed out", self.output))
        self.read(0.2)
        assert self.p.returncode == code, (self.p.returncode, self.output)
        if alternate:
            assert b"\x1b[?1049h" in self.output and b"\x1b[?1049l" in self.output, self.output
        after = termios.tcgetattr(self.slave)
        # Darwin may set this transient kernel flag while restoring termios.
        after[3] &= ~termios.PENDIN
        self.before[3] &= ~termios.PENDIN
        assert after == self.before, (after, self.before)
        assert b"synthetic-token" not in self.output and b"private-fixture-token" not in self.output
        os.close(self.master); os.close(self.slave); self.tmp.cleanup()
    def cleanup(self):
        if self.p.poll() is None: self.p.kill(); self.p.wait(timeout=5)

sessions = []
try:
    t = Terminal(args=["--theme=mono", "--no-record"]); sessions.append(t)
    t.wait(b"PTY issue")
    t.send(b"\r"); t.wait(b"PTY description")
    t.send(b"s"); t.wait(b"Revisar antes de aplicar")
    assert not any("write" in e for e in t.events())
    start = t.send(b"y"); t.wait(b"verified", start)
    t.send(b"\r"); t.read(0.5)
    t.send(b"d"); t.wait(b"Campo: Resolution")
    t.send(b"fixed\r"); t.read(0.5)
    # Review can exceed one screen; scroll to its end before confirming.
    t.send(b"jjjjjjjjjjjjjjjjjjjj"); t.read(0.2)
    start = t.send(b"y"); t.wait(b"verified", start)
    t.send(b"\r"); t.read(0.5)
    t.send(b"o"); t.wait(b"Navegador iniciado")
    t.send(b"\x06"); t.wait(b"Busqueda remota JQL")
    t.send(b"project = APP\r"); t.read(0.5)
    events = t.events()
    assert [e["write"] for e in events if "write" in e] == ["start", "done"], events
    assert next(e for e in events if e.get("write") == "done")["fields"] == {"resolution": {"id": "fixed"}}
    assert any(e.get("open") == "https://example.atlassian.net/browse/APP-1" for e in events), events
    assert any(e.get("search") == "project = APP" for e in events), events
    # Resize and quit through the actual event loop.
    fcntl.ioctl(t.slave, termios.TIOCSWINSZ, struct.pack("HHHH", 20, 80, 0, 0))
    t.p.send_signal(signal.SIGWINCH); t.read(0.2); t.send(b"q"); t.finish()

    for scoped in (False, True):
        t = Terminal(setup=True, args=["ui", "--theme=mono"]); sessions.append(t)
        t.wait(b"Token scoped?"); t.send(b"y\n" if scoped else b"n\n")
        t.wait(b"Profile:"); t.send(b"fixture\n")
        t.wait(b"HTTPS Site:"); t.send(b"https://example.atlassian.net\n")
        t.wait(b"Email:"); t.send(b"fixture@example.com\n")
        if scoped:
            t.wait(b"Real Cloud ID"); t.send(b"fixture-cloud\n")
        t.wait(b"API token (hidden input)"); t.read(0.1); t.send(b"private-fixture-token\r")
        t.wait(b"PTY issue")
        events = t.events()
        assert any(e.get("login") == ("api-token-scoped" if scoped else "api-token-unscoped") for e in events), events
        assert any(e.get("stored") for e in events), events
        t.send(b"q"); t.finish()

    t = Terminal(args=["ui", "--theme=mono"]); sessions.append(t)
    t.wait(b"PTY issue"); t.p.send_signal(signal.SIGTERM); t.finish(code=130)

    t = Terminal(setup=True); sessions.append(t)
    t.wait(b"Token scoped?"); t.p.send_signal(signal.SIGINT); t.finish(code=130, alternate=False)
    t = Terminal(setup=True); sessions.append(t)
    t.wait(b"Token scoped?"); t.send(b"n\n")
    t.wait(b"Profile:"); t.send(b"fixture\n")
    t.wait(b"HTTPS Site:"); t.send(b"https://example.atlassian.net\n")
    t.wait(b"Email:"); t.send(b"fixture@example.com\n")
    t.wait(b"API token (hidden input)"); t.read(0.1)
    t.send(b"\x03"); t.finish(code=130, alternate=False)
    t = Terminal(args=["ui", "--theme=mono", "--no-record"], block_write=True); sessions.append(t)
    t.wait(b"PTY issue"); t.send(b"s"); t.wait(b"Revisar antes de aplicar")
    t.send(b"y"); t.wait(b"Enviando/verificando")
    t.p.send_signal(signal.SIGTERM); t.finish(code=9)
    print("PTY journeys passed: root entry, workflow fields/confirmation, browser, remote search, resize, scoped/unscoped setup, hidden token, signals and terminal restoration")
finally:
    for t in sessions: t.cleanup()
