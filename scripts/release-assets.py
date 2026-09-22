"""Generate offline CLI reference, completions, and dependency notices."""
import json
import os
from pathlib import Path
import re
import subprocess
import sys

binary = str(Path(sys.argv[1]).resolve())
out = Path('bin/release-assets')
out.mkdir(parents=True, exist_ok=True)
env = {k: v for k, v in os.environ.items() if not k.startswith('JFLOW_')}
env['NO_COLOR'] = '1'

def run(*args):
    return subprocess.check_output(args, env=env, text=True)

for shell, name in [('bash', 'jflow.bash'), ('zsh', '_jflow'), ('fish', 'jflow.fish')]:
    (out / name).write_text(run(binary, 'completion', shell))

sections = []
def help_tree(path):
    help_text = run(binary, *path, '--help')
    sections.append('## jflow' + (' ' + ' '.join(path) if path else '') + '\n\n```text\n' + help_text + '```\n')
    match = re.search(r'Available Commands:\n(.*?)(?:\n\n|\Z)', help_text, re.S)
    if match:
        for child in re.findall(r'^  ([a-z][a-z-]*)\s', match[1], re.M):
            if child != 'help':
                help_tree(path + [child])

help_tree([])
(out / 'CLI-REFERENCE.md').write_text('# Generated CLI reference\n\n' + '\n'.join(sections))
module_paths = set()
for target_os in ('linux', 'darwin'):
    for target_arch in ('amd64', 'arm64'):
        target_env = dict(env, GOOS=target_os, GOARCH=target_arch, CGO_ENABLED='0')
        paths = subprocess.check_output(['go', 'list', '-deps', '-test', '-tags=foundation', '-f', '{{if .Module}}{{.Module.Path}}{{end}}', './...'], env=target_env, text=True)
        module_paths.update(paths.split())
raw = run('go', 'list', '-m', '-json', *sorted(module_paths))
decoder = json.JSONDecoder()
modules = []
while raw.strip():
    module, end = decoder.raw_decode(raw.lstrip())
    raw = raw.lstrip()[end:]
    if not module.get('Main'):
        modules.append(module)
notices = ['# Third-party notices\n\nModules used by the four target platforms, including tests and foundation checks.\n']
for module in modules:
    directory = Path(module['Dir'])
    licenses = sorted(p for p in directory.iterdir() if p.is_file() and p.name.upper().startswith(('LICENSE', 'COPYING', 'NOTICE')))
    if not licenses:
        raise SystemExit('Missing license text: ' + module['Path'])
    notices.append('\n## ' + module['Path'] + ' ' + module['Version'] + '\n')
    for license_file in licenses:
        notices.append('\n### ' + license_file.name + '\n\n' + license_file.read_text())
notices.append('\n## Go runtime\n\n' + (Path(run('go', 'env', 'GOROOT').strip()) / 'LICENSE').read_text())
(out / 'THIRD-PARTY-NOTICES.txt').write_text('\n'.join(notices))
print(f'Generated {len(sections)} help pages, 3 completions and {len(modules)} dependency notices in {out}')
