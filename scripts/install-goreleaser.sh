#!/bin/sh
# Install the pinned upstream binary after checking its upstream SHA-256 manifest.
set -eu
version=${1:?GoReleaser version required}
case "$version" in v[0-9]*.[0-9]*.[0-9]*) ;; *) exit 2 ;; esac
case "$(uname -s)" in Linux) target_os=Linux ;; Darwin) target_os=Darwin ;; *) exit 2 ;; esac
case "$(uname -m)" in x86_64) target_arch=x86_64 ;; arm64|aarch64) target_arch=arm64 ;; *) exit 2 ;; esac
archive="goreleaser_${target_os}_${target_arch}.tar.gz"
tool_tmp=$(mktemp -d)
trap 'rm -rf "$tool_tmp"' EXIT HUP INT TERM
base="https://github.com/goreleaser/goreleaser/releases/download/$version"
curl --fail --location --silent --show-error "$base/$archive" -o "$tool_tmp/$archive"
curl --fail --location --silent --show-error "$base/checksums.txt" -o "$tool_tmp/checksums.txt"
python3 - "$tool_tmp" "$archive" <<'PY'
import hashlib
from pathlib import Path
import sys
import tarfile
root, name = Path(sys.argv[1]), sys.argv[2]
entries = [line.split() for line in (root / 'checksums.txt').read_text().splitlines()]
expected = next(digest for digest, filename in entries if filename == name)
archive = root / name
if hashlib.sha256(archive.read_bytes()).hexdigest() != expected:
    raise SystemExit('GoReleaser checksum mismatch')
with tarfile.open(archive) as tar:
    binary = tar.extractfile('goreleaser').read()
output = Path('.tools/goreleaser')
output.parent.mkdir(exist_ok=True)
output.write_bytes(binary)
output.chmod(0o755)
PY
.tools/goreleaser --version
