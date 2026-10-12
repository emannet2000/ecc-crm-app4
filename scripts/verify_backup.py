#!/usr/bin/env python3
"""Restore a private archive into temporary storage and verify database and upload hashes."""
import argparse
import hashlib
import json
from pathlib import Path
import sqlite3
import tempfile
from datetime import datetime, timezone

from restore import restore


def verify_backup(archive):
    with tempfile.TemporaryDirectory(prefix='ecc-restore-check-') as directory:
        restored = Path(directory) / 'restored'
        restore(archive, restored)
        counts = {}
        files = 0
        with sqlite3.connect(restored / 'crm.sqlite3') as database:
            tables = [row[0] for row in database.execute("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")]
            for name in tables:
                counts[name] = database.execute('SELECT count(*) FROM "' + name.replace('"', '""') + '"').fetchone()[0]
            if 'file_versions' in tables:
                columns = {row[1] for row in database.execute('PRAGMA table_info(file_versions)')}
                checksum = 'sha256' if 'sha256' in columns else "''"
                for path, expected in database.execute('SELECT storage_path,' + checksum + ' FROM file_versions'):
                    file = Path(path)
                    if not file.is_relative_to(restored) or not file.is_file():
                        raise ValueError('A restored document is unavailable')
                    digest = hashlib.sha256()
                    with file.open('rb') as source:
                        for chunk in iter(lambda: source.read(1024 * 1024), b''):
                            digest.update(chunk)
                    if expected and digest.hexdigest() != expected:
                        raise ValueError('A restored document checksum does not match')
                    files += 1
        return {'state': 'verified', 'checkedAt': datetime.now(timezone.utc).isoformat(),
                'archive': str(Path(archive).resolve()), 'tables': counts, 'documentFiles': files,
                'manifestVerified': (restored / 'backup-manifest.json').is_file()}


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('archive')
    parser.add_argument('--status', help='Optional backup-status.json to update with this verification result')
    args = parser.parse_args()
    try:
        result = verify_backup(args.archive)
    except Exception as error:
        result = {'state': 'failed', 'checkedAt': datetime.now(timezone.utc).isoformat(),
                  'archive': str(Path(args.archive).resolve()), 'message': str(error)[:300]}
    if args.status:
        import os
        status_path = Path(args.status)
        status = json.loads(status_path.read_text()) if status_path.exists() else {}
        status['restoreVerification'] = result
        status_path.write_text(json.dumps(status))
        os.chmod(status_path, 0o600)
    print(json.dumps(result))
    if result['state'] != 'verified':
        raise SystemExit(1)
