"""Produce a local review-only release manifest without executing a release."""
import argparse
import hashlib
import json
from pathlib import Path
import re
import stat
import zipfile

from checkpoint_policy import checkpoint_path_allowed


def sha256(path):
    digest = hashlib.sha256()
    with path.open('rb') as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b''):
            digest.update(chunk)
    return digest.hexdigest()


def local_path(root, value):
    path = Path(value)
    path = (root / path if not path.is_absolute() else path).resolve()
    if not path.is_relative_to(root):
        raise ValueError('path outside repository')
    return path


def verify_checkpoint(root, path):
    manifest = json.loads(path.read_text(encoding='utf-8-sig'))
    changes = manifest.get('changed_from_baseline')
    if not isinstance(changes, dict) or not changes:
        raise ValueError('checkpoint requires a nonempty source hash manifest')
    archive = local_path(root, path.with_suffix('.zip'))
    if not archive.is_file():
        raise ValueError('checkpoint ZIP missing')
    with zipfile.ZipFile(archive) as source:
        entries = source.infolist()
        names = [entry.filename for entry in entries]
        if len(names) != len(set(names)):
            raise ValueError('duplicate checkpoint entries')
        if any(stat.S_IFMT(entry.external_attr >> 16) == stat.S_IFLNK for entry in entries):
            raise ValueError('symlink checkpoint entries are not allowed')
        if set(names) != set(changes) | {'checkpoint-manifest.json'}:
            raise ValueError('checkpoint entry set mismatch')
        if json.loads(source.read('checkpoint-manifest.json')) != manifest:
            raise ValueError('checkpoint embedded manifest mismatch')
        for name, expected in changes.items():
            if not checkpoint_path_allowed(name):
                raise ValueError('checkpoint path outside recovery allowlist')
            if not isinstance(expected, str) or not re.fullmatch(r'[0-9a-f]{64}', expected):
                raise ValueError('invalid checkpoint source hash')
            digest = hashlib.sha256()
            with source.open(name) as stream:
                for chunk in iter(lambda: stream.read(1024 * 1024), b''):
                    digest.update(chunk)
            if digest.hexdigest() != expected:
                raise ValueError('checkpoint source hash mismatch')
    return archive, manifest


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    for option in [
        'repo',
        'checkpoint',
        'archive',
        'image-digest',
        'expected-production-version',
        'expected-production-image-digest',
        'output',
    ]:
        parser.add_argument('--' + option, required=True)
    args = parser.parse_args(argv)
    root = Path(args.repo).resolve()
    if not root.is_dir():
        raise ValueError('repository must be an existing directory')
    checkpoint, archive, output = (
        local_path(root, value) for value in [args.checkpoint, args.archive, args.output]
    )
    if not checkpoint.is_file() or not archive.is_file():
        raise ValueError('checkpoint/archive must be files')
    if output.exists() or not output.parent.is_dir():
        raise ValueError('output must be a new file in an existing directory')
    if not re.fullmatch(r'sha256:[0-9a-f]{64}', args.image_digest):
        raise ValueError('invalid image digest')
    if not re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9._+-]{0,127}', args.expected_production_version):
        raise ValueError('invalid expected production version')
    if not re.fullmatch(r'sha256:[0-9a-f]{64}', args.expected_production_image_digest):
        raise ValueError('invalid expected production image digest')
    checkpoint_zip, manifest = verify_checkpoint(root, checkpoint)
    plan = {
        'mode': 'offline-review-only',
        'source_baseline_commit': manifest.get('head_baseline'),
        'checkpoint': str(checkpoint.relative_to(root)),
        'checkpoint_manifest_sha256': sha256(checkpoint),
        'checkpoint_zip_sha256': sha256(checkpoint_zip),
        'archive': str(archive.relative_to(root)),
        'archive_sha256': sha256(archive),
        'user_provided_image_digest': args.image_digest,
        'source_to_image_mapping': 'unverified; user-provided digest only',
        'expected_production': {
            'version': args.expected_production_version,
            'image_digest': args.expected_production_image_digest,
            'source': 'operator-provided; must be independently verified at release time',
        },
        'production_identity_gate': (
            'Before any release, read the authorized production version and image digest. '
            'They must exactly match expected_production; otherwise this plan is stale and '
            'the release must stop and be rebuilt from the new baseline.'
        ),
        'current_production_mapping': 'not accessed by this offline tool',
        'execute_commands': False,
        'acceptance': [
            'Verify checkpoint, artifact hashes and independently established image provenance.',
            'Abort if the authorized production version or image digest differs from expected_production.',
            'Review configuration and database migration compatibility before release.',
            'After separate production authorization: verify container health and critical business behavior.',
            'Require TLS certificate and hostname verification; insecure TLS probes never qualify as acceptance.',
            'Require expected successful HTTP status, response content and absence of new fatal errors.',
        ],
        'rollback': [
            'Retain verified previous image identity, Compose backup and recoverable database backup.',
            'Restore compatible previous configuration/image; never reverse an irreversible migration automatically.',
            'Repeat configuration, health, strict TLS and critical business acceptance after rollback.',
            'Keep sensitive-directory ignore rules; private local checkpoints must not be published.',
        ],
    }
    with output.open('x', encoding='utf-8') as stream:
        json.dump(plan, stream, ensure_ascii=False, indent=2)
        stream.write('\n')
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
