"""Check checksums, archive contents and native executable metadata offline."""
import hashlib
import json
from pathlib import Path
import platform
import subprocess
import tarfile
import tempfile

root = Path('dist')
archives = sorted(root.glob('jflow_*.tar.gz'))
expected_targets = {'linux_amd64', 'linux_arm64', 'darwin_amd64', 'darwin_arm64'}
assert len(archives) == 4, f'Expected four archives: {archives}'
checksums = {}
for line in (root / 'checksums.txt').read_text().splitlines():
    digest, name = line.split(maxsplit=1)
    checksums[name.strip().lstrip('*')] = digest
assert set(checksums) == {p.name for p in archives}, 'Checksum inventory mismatch'
native = platform.system().lower() + '_' + {'aarch64': 'arm64', 'x86_64': 'amd64'}.get(platform.machine(), platform.machine())
native_checked = False
required = {'jflow', 'README.md', 'INSTALL.md', 'DISTRIBUTION-STATUS.md', 'CHANGELOG.md', 'extras/CLI-REFERENCE.md', 'extras/THIRD-PARTY-NOTICES.txt', 'extras/jflow.bash', 'extras/_jflow', 'extras/jflow.fish'}
for archive in archives:
    target = '_'.join(archive.name.removesuffix('.tar.gz').split('_')[-2:])
    assert target in expected_targets, target
    expected_targets.remove(target)
    assert hashlib.sha256(archive.read_bytes()).hexdigest() == checksums[archive.name], archive
    with tarfile.open(archive) as tar:
        members = {m.name: m for m in tar.getmembers() if m.isfile()}
        assert required <= members.keys(), f'Missing files in {archive}: {required - members.keys()}'
        assert members['jflow'].mode & 0o111, 'Binary is not executable'
        if target == native:
            with tempfile.TemporaryDirectory() as directory:
                binary = Path(directory) / 'jflow'
                binary.write_bytes(tar.extractfile(members['jflow']).read())
                binary.chmod(0o700)
                data = json.loads(subprocess.check_output([str(binary), 'version', '--format=json']))
                assert data['ok'] and data['schema_version'] == 1, data
                metadata = data['data']
                assert metadata['os'] + '_' + metadata['arch'] == native, metadata
                assert metadata['version'] not in ('dev', ''), metadata
                assert metadata['commit'] == subprocess.check_output(['git', 'rev-parse', 'HEAD'], text=True).strip(), metadata
                subprocess.run([str(binary), '--help'], check=True, stdout=subprocess.DEVNULL)
                print(f'Native smoke passed: {native}, {metadata}')
                native_checked = True
    print(f'Verified {archive.name}')
assert not expected_targets
assert native_checked, f'No native archive was executed for {native}'
