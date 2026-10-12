#!/usr/bin/env python3
"""Restore to a NEW directory. Stop the CRM before replacing its data directory."""
import argparse
import os
from pathlib import Path
import shutil
import sqlite3
import tarfile
import tempfile
import hashlib
import json


def restore(archive_path, destination):
    destination = Path(destination).resolve()
    if destination.exists():
        raise ValueError('Restore destination must not exist')
    destination.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(dir=destination.parent) as directory:
        target = Path(directory) / 'data'
        target.mkdir(mode=0o700)
        with tarfile.open(archive_path, 'r:gz') as archive:
            members = archive.getmembers()
            for member in members:
                parts = Path(member.name).parts
                if not parts or parts[0] not in ('crm.sqlite3', 'uploads', 'auth-secret', 'backup-manifest.json') or '..' in parts or Path(member.name).is_absolute() or not (member.isfile() or member.isdir()):
                    raise ValueError('Unsafe archive entry')
            options = {'filter': 'data'} if hasattr(tarfile, 'data_filter') else {}
            archive.extractall(target, members=members, **options)
        database = target / 'crm.sqlite3'
        if not database.is_file():
            raise ValueError('Backup does not contain a database')
        manifest = target / 'backup-manifest.json'
        if manifest.exists():
            checks = json.loads(manifest.read_text())
            if checks.get('version') != 1 or not isinstance(checks.get('files'), dict) or 'crm.sqlite3' not in checks['files']:
                raise ValueError('Backup manifest is invalid')
            actual = {path.relative_to(target).as_posix() for path in target.rglob('*') if path.is_file() and path != manifest}
            if actual != set(checks['files']):
                raise ValueError('Backup manifest is missing files or has unexpected files')
            for name, expected in checks['files'].items():
                file = target / name
                if not file.resolve().is_relative_to(target):
                    raise ValueError('Backup manifest has an unsafe file path')
                digest = hashlib.sha256()
                with file.open('rb') as source:
                    for chunk in iter(lambda: source.read(1024 * 1024), b''):
                        digest.update(chunk)
                if digest.hexdigest() != expected:
                    raise ValueError('Backup manifest checksum does not match')
        with sqlite3.connect(f'file:{database}?mode=ro', uri=True) as connection:
            if connection.execute('PRAGMA integrity_check').fetchone()[0] != 'ok':
                raise ValueError('Restored database failed integrity check')
        for path in target.rglob('*'):
            path.chmod(0o700 if path.is_dir() else 0o600)
        # File versions use absolute storage paths; update them when relocating.
        with sqlite3.connect(database) as connection:
            if connection.execute("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='file_versions'").fetchone()[0]:
                for file_id, org, document, old in connection.execute('SELECT id,org_id,document_id,storage_path FROM file_versions').fetchall():
                    if any(not part or part in ('.', '..') or '/' in part or '\\' in part for part in (org, document)):
                        raise ValueError('Invalid document storage identity')
                    new = destination / 'uploads' / org / document / Path(old).name
                    if not (target / 'uploads' / org / document / Path(old).name).is_file():
                        raise ValueError('Backup is missing a referenced document file')
                    connection.execute('UPDATE file_versions SET storage_path=? WHERE id=?', (str(new), file_id))
            if connection.execute("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='file_gc'").fetchone()[0]:
                for (old,) in connection.execute('SELECT path FROM file_gc').fetchall():
                    # Generated uploads have an organization, document and random filename.
                    parts = Path(old).parts
                    if len(parts) < 4 or parts[-4] != 'uploads' or any(part in ('.', '..') or '\\' in part for part in parts[-3:]):
                        raise ValueError('Invalid queued file cleanup path')
                    new = destination / 'uploads' / Path(*parts[-3:])
                    connection.execute('UPDATE file_gc SET path=? WHERE path=?', (str(new), old))
        os.rename(target, destination)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('archive')
    parser.add_argument('destination')
    args = parser.parse_args()
    restore(args.archive, args.destination)
    print(f'Restored to: {args.destination}. Preserve JWT_SECRET before starting the CRM.')
