#!/usr/bin/env python3
"""Release helpers. All mutations are restricted to the checked-out build or GitHub."""
import base64
import json
import os
import re
import subprocess
import sys
from pathlib import Path

SEMVER = re.compile(r'^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$')


def version(value):
    match = SEMVER.fullmatch(value)
    if not match:
        raise ValueError(f'invalid version: {value}')
    return tuple(map(int, match.groups()))


def semver_tags(tags):
    return [t for t in tags if t.startswith('v') and SEMVER.fullmatch(t[1:])]


def next_tag(tags, major, minor, registry_version=None):
    versions = [version(t[1:]) for t in semver_tags(tags)]
    if registry_version is not None:
        versions.append(version(registry_version))
    if any(v[:2] > (major, minor) for v in versions):
        raise ValueError('configured release series is older than an existing version')
    patches = [v[2] for v in versions if v[:2] == (major, minor)]
    return f'v{major}.{minor}.{max(patches, default=-1) + 1}'


def select_tag(tags, commit_tags, major, minor, registry_version=None):
    existing = semver_tags(commit_tags)
    if len(existing) > 1:
        raise ValueError('multiple release version tags point to this commit; manual intervention required')
    if existing:
        if version(existing[0][1:])[:2] != (major, minor):
            raise ValueError('existing commit tag does not match configured series')
        return existing[0]
    return next_tag(tags, major, minor, registry_version)


def assert_current_source(tags, source, ancestor):
    tags = semver_tags(tags)
    if tags:
        latest = max(tags, key=lambda tag: version(tag[1:]))
        if not ancestor(latest, source):
            raise ValueError(f'latest version tag {latest} is not an ancestor of source {source}')


def git_ancestor(tag, source):
    result = subprocess.run(['git', 'merge-base', '--is-ancestor', f'{tag}^{{commit}}', source])
    if result.returncode not in (0, 1):
        raise RuntimeError('git could not determine source ancestry')
    return result.returncode == 0


def check_source(source):
    # Freshly fetch tags before each check. Never force-move or delete a tag.
    run('git', 'fetch', 'origin', '--tags')
    assert_current_source(run('git', 'tag', '--list').splitlines(), source, git_ancestor)


def update_registry(data, target):
    matches = [p for p in data['plugins'] if p.get('id') == 'quota-reset']
    if len(matches) != 1:
        raise ValueError('registry must contain exactly one quota-reset plugin')
    current = version(matches[0]['version'])
    desired = version(target)
    if current >= desired:
        return False
    matches[0]['version'] = target
    return True


def verify_checksums(local, remote):
    if local.read_bytes() != remote.read_bytes():
        raise ValueError('published checksum differs from the freshly built release')


def stamp(root, value):
    version(value)
    replacements = [
        (root / 'internal/runtime/runtime.go', r'(Version:\s*")[^"]+("\s*,)', rf'\g<1>{value}\2'),
        (root / 'internal/management/page.go', r'(class="version-badge">)v[^<]+', rf'\g<1>v{value}'),
    ]
    updates = []
    for path, pattern, replacement in replacements:
        text, count = re.subn(pattern, replacement, path.read_text())
        if count != 1:
            raise ValueError(f'expected exactly one version field in {path}, got {count}')
        updates.append((path, text))
    for path, text in updates:
        path.write_text(text)


def run(*args, input=None):
    return subprocess.run(args, input=input, text=True, check=True, stdout=subprocess.PIPE).stdout.strip()


def api(path, *args, payload=None):
    cmd = ['gh', 'api', path, *args]
    if payload is not None:
        cmd += ['--input', '-']
    return json.loads(run(*cmd, input=json.dumps(payload) if payload is not None else None))


def registry_entry(data):
    matches = [p for p in data['plugins'] if p.get('id') == 'quota-reset']
    if len(matches) != 1:
        raise ValueError('registry must contain exactly one quota-reset plugin')
    return matches[0]


def registry_state():
    from urllib.parse import quote
    repo = os.environ['GITHUB_REPOSITORY']
    branch = api(f'repos/{repo}')['default_branch']
    path = f'repos/{repo}/contents/registry.json'
    meta = api(f'{path}?ref={quote(branch, safe="")}')
    return branch, path, meta, json.loads(base64.b64decode(meta['content']))


def registry(target):
    source = os.environ['GITHUB_SHA']
    branch, path, meta, data = registry_state()
    # Contents API's blob SHA is an optimistic lock. Re-read and retry conflicts;
    # an unrelated branch update is preserved by GitHub's contents API.
    for attempt in range(5):
        check_source(source)
        if attempt:
            branch, path, meta, data = registry_state()
        if not update_registry(data, target):
            print('Registry already at this version or newer; leaving it unchanged')
            return
        payload = {'message': f'chore: publish quota-reset v{target} registry',
                   'branch': branch, 'sha': meta['sha'],
                   'content': base64.b64encode((json.dumps(data, indent=2, ensure_ascii=False) + '\n').encode()).decode()}
        try:
            api(path, '--method', 'PUT', payload=payload)
            return
        except subprocess.CalledProcessError:
            if attempt == 4:
                raise
    raise RuntimeError('registry update failed')


def main():
    command = sys.argv[1]
    if command == 'select':
        major, minor = (int(x) for x in sys.argv[2:4])
        if major < 0 or minor < 0:
            raise ValueError('major and minor must be nonnegative')
        source = os.environ['GITHUB_SHA']
        check_source(source)
        tags = run('git', 'tag', '--list').splitlines()
        commit_tags = [tag for tag in semver_tags(tags)
                       if run('git', 'rev-parse', f'{tag}^{{commit}}') == source]
        _, _, _, data = registry_state()
        print(select_tag(tags, commit_tags, major, minor, registry_entry(data)['version']))
    elif command == 'stamp':
        stamp(Path('.'), sys.argv[2])
    elif command == 'registry':
        registry(sys.argv[2])
    elif command == 'verify-checksums':
        verify_checksums(Path(sys.argv[2]), Path(sys.argv[3]))
    else:
        raise ValueError(f'unknown command {command}')


if __name__ == '__main__':
    main()
