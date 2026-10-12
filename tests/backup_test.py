import importlib.util
import io
from pathlib import Path
import sqlite3
import tarfile
import tempfile
import unittest
import subprocess
import sys
import hashlib
import json

ROOT = Path(__file__).resolve().parents[1]


def module(name):
    spec = importlib.util.spec_from_file_location(name, ROOT / 'scripts' / (name + '.py'))
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


class BackupTest(unittest.TestCase):
    def test_restore_verification_checks_document_bytes_and_reports_corruption(self):
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            data = directory / 'original'
            data.mkdir()
            file = data / 'uploads' / 'org' / 'doc' / 'original.txt'
            file.parent.mkdir(parents=True)
            file.write_bytes(b'Original document')
            database = data / 'crm.sqlite3'
            with sqlite3.connect(database) as connection:
                connection.execute('CREATE TABLE file_versions(id TEXT,org_id TEXT,document_id TEXT,storage_path TEXT,sha256 TEXT)')
                connection.execute('INSERT INTO file_versions VALUES(?,?,?,?,?)', ('version', 'org', 'doc', str(file), hashlib.sha256(file.read_bytes()).hexdigest()))
            archive = directory / 'verified.tar.gz'
            # Exercise the actual backup CLI, which verifies before declaring success.
            result = subprocess.run([sys.executable, str(ROOT / 'scripts/backup.py'), str(database), str(archive)], capture_output=True, text=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            status = json.loads((data / 'backup-status.json').read_text())
            self.assertEqual(status['restoreVerification']['state'], 'verified')
            self.assertEqual(status['restoreVerification']['documentFiles'], 1)
            self.assertEqual(status['restoreVerification']['tables']['file_versions'], 1)
            file.write_bytes(b'Corrupted document')
            broken = directory / 'broken.tar.gz'
            module('backup').backup(database, broken)
            result = subprocess.run([sys.executable, str(ROOT / 'scripts/verify_backup.py'), str(broken)], capture_output=True, text=True)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn('checksum', result.stdout)
            self.assertEqual(file.read_bytes(), b'Corrupted document')

    def test_roundtrip_preserves_wal_uploads_secret_and_relocates_files(self):
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            data = directory / 'original'
            data.mkdir()
            database = data / 'crm.sqlite3'
            file = data / 'uploads' / 'org' / 'document' / 'private-file'
            file.parent.mkdir(parents=True)
            file.write_text('Confidential document')
            (data / 'auth-secret').write_text('private encryption secret')
            connection = sqlite3.connect(database)
            connection.execute('PRAGMA journal_mode=WAL')
            connection.execute('CREATE TABLE file_versions(id TEXT,org_id TEXT,document_id TEXT,storage_path TEXT)')
            connection.execute('INSERT INTO file_versions VALUES(?,?,?,?)', ('version', 'org', 'document', str(file)))
            queued = file.parent / 'queued-file'
            queued.write_text('Pending cleanup')
            connection.execute('CREATE TABLE file_gc(path TEXT PRIMARY KEY)')
            connection.execute('INSERT INTO file_gc VALUES(?)', (str(queued),))
            connection.commit()
            archive = directory / 'backup.tar.gz'
            module('backup').backup(database, archive)
            connection.close()
            self.assertEqual(archive.stat().st_mode & 0o777, 0o600)
            restored = directory / 'restored'
            module('restore').restore(archive, restored)
            self.assertEqual((restored / 'auth-secret').read_text(), 'private encryption secret')
            with sqlite3.connect(restored / 'crm.sqlite3') as db:
                new = Path(db.execute('SELECT storage_path FROM file_versions').fetchone()[0])
            self.assertEqual(new.read_text(), 'Confidential document')
            with sqlite3.connect(restored / 'crm.sqlite3') as db:
                cleanup = Path(db.execute('SELECT path FROM file_gc').fetchone()[0])
            self.assertEqual(cleanup.read_text(), 'Pending cleanup')
            self.assertTrue(cleanup.is_relative_to(restored))
            self.assertTrue(new.is_relative_to(restored))
            self.assertEqual(new.stat().st_mode & 0o777, 0o600)
            with self.assertRaises(ValueError):
                module('restore').restore(archive, restored)

    def test_archive_traversal_and_symlinks_are_rejected(self):
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            for name, kind in [('../escape', tarfile.REGTYPE), ('uploads/link', tarfile.SYMTYPE)]:
                archive = directory / 'unsafe.tar.gz'
                with tarfile.open(archive, 'w:gz') as tar:
                    info = tarfile.TarInfo(name)
                    info.type = kind
                    info.linkname = '../../escape'
                    tar.addfile(info, io.BytesIO())
                with self.assertRaises(ValueError):
                    module('restore').restore(archive, directory / 'restored')
                self.assertFalse((directory / 'restored').exists())
                self.assertFalse((directory / 'escape').exists())

    def test_backup_refuses_overwrite_and_missing_database(self):
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            with self.assertRaises(ValueError):
                module('backup').backup(directory / 'missing', directory / 'backup')
            database = directory / 'crm.sqlite3'
            with sqlite3.connect(database) as db:
                db.execute('CREATE TABLE test(id INTEGER)')
            archive = directory / 'backup.tar.gz'
            module('backup').backup(database, archive)
            original = archive.read_bytes()
            with self.assertRaises(ValueError):
                module('backup').backup(database, archive)
            self.assertEqual(archive.read_bytes(), original)


if __name__ == '__main__':
    unittest.main()
