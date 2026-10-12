#!/usr/bin/env python3
"""SQLite online backup plus uploads and encryption secret in a private archive.
For a mutually consistent backup, stop the service while this command runs.
"""
import argparse
import json
import os
from pathlib import Path
import sqlite3
import tarfile
import tempfile
import hashlib
import io


def backup(database, destination, secret=None):
    database = Path(database).resolve()
    destination = Path(destination).resolve()
    if not database.is_file():
        raise ValueError('Database does not exist')
    secret = Path(secret).resolve() if secret else database.parent / 'auth-secret'
    destination.parent.mkdir(parents=True, exist_ok=True)
    if destination.exists():
        raise ValueError('Destination already exists; choose a new backup name')
    fd = os.open(destination, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    try:
        with tempfile.TemporaryDirectory() as directory:
            snapshot = Path(directory) / 'crm.sqlite3'
            with sqlite3.connect(f'file:{database}?mode=ro', uri=True) as source, sqlite3.connect(snapshot) as target:
                source.backup(target)
                if target.execute('PRAGMA integrity_check').fetchone()[0] != 'ok':
                    raise ValueError('Database integrity check failed')
            with os.fdopen(fd, 'wb') as file, tarfile.open(fileobj=file, mode='w:gz') as archive:
                fd = None
                checksums = {}
                def remember(source, name):
                    digest = hashlib.sha256()
                    with source.open('rb') as content:
                        for chunk in iter(lambda: content.read(1024 * 1024), b''):
                            digest.update(chunk)
                    checksums[name] = digest.hexdigest()
                remember(snapshot, 'crm.sqlite3')
                archive.add(snapshot, arcname='crm.sqlite3')
                uploads = database.parent / 'uploads'
                if uploads.exists():
                    for uploaded in uploads.rglob('*'):
                        if uploaded.is_file():
                            remember(uploaded, 'uploads/' + uploaded.relative_to(uploads).as_posix())
                    archive.add(uploads, arcname='uploads')
                if secret.exists():
                    remember(secret, 'auth-secret')
                    archive.add(secret, arcname='auth-secret')
                content = json.dumps({'version': 1, 'files': checksums}).encode()
                entry = tarfile.TarInfo('backup-manifest.json')
                entry.size = len(content)
                entry.mode = 0o600
                archive.addfile(entry, io.BytesIO(content))
    except BaseException:
        if fd is not None:
            os.close(fd)
        destination.unlink(missing_ok=True)
        raise


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('database')
    parser.add_argument('destination')
    parser.add_argument('--secret', help='Generated auth-secret path; preserve JWT_SECRET separately when configured')
    args = parser.parse_args()
    status_path = Path(args.database).resolve().parent / 'backup-status.json'
    try:
        backup(args.database, args.destination, args.secret)
        from verify_backup import verify_backup
        verification = verify_backup(args.destination)
    except Exception as error:
        status_path.write_text(json.dumps({'state': 'failed', 'checkedAt': __import__('datetime').datetime.now(__import__('datetime').timezone.utc).isoformat(), 'message': str(error)[:300]}))
        os.chmod(status_path, 0o600)
        raise
    status_path.write_text(json.dumps({'state': 'ok', 'checkedAt': __import__('datetime').datetime.now(__import__('datetime').timezone.utc).isoformat(), 'destination': str(Path(args.destination).resolve()), 'restoreVerification': verification}))
    os.chmod(status_path, 0o600)
    print(f'Backup created: {args.destination}')
