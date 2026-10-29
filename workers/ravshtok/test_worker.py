import os
import tempfile
import unittest
from pathlib import Path
from unittest.mock import MagicMock, patch
from uuid import uuid4

os.environ.setdefault('DATABASE_URL', 'postgresql://unused')
import worker


class CleanupTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name)
        self.output = self.root / 'ravshtok'
        self.output.mkdir()
        self.paths = patch.multiple(worker, MEDIA_ROOT=self.root, OUTPUT_DIR=self.output)
        self.paths.start()

    def tearDown(self):
        self.paths.stop()
        self.temp.cleanup()

    def connection(self, rows):
        connection = MagicMock()
        cursor = connection.cursor.return_value.__enter__.return_value
        cursor.fetchall.side_effect = [rows, []]
        return connection, cursor

    def test_deleted_media_and_poster_removed(self):
        for name in ('test.mp4', 'test.webp'):
            (self.output / name).write_bytes(b'test')
        connection, cursor = self.connection([{
            'submission_id': 'test', 'media_path': 'ravshtok/test.mp4', 'poster_path': 'ravshtok/test.webp',
        }])
        self.assertEqual(worker.cleanup_expired(connection), 1)
        self.assertEqual(list(self.output.iterdir()), [])
        self.assertTrue(any("size_bytes=0" in call.args[0] for call in cursor.execute.call_args_list))

    def test_failed_unlink_keeps_paths_for_retry(self):
        connection, cursor = self.connection([{
            'submission_id': 'test', 'media_path': 'ravshtok/test.mp4', 'poster_path': '',
        }])
        with patch.object(worker, 'remove_media_file', side_effect=PermissionError('busy')):
            self.assertEqual(worker.cleanup_expired(connection), 0)
        self.assertFalse(any("UPDATE ravshtok_media" in call.args[0] for call in cursor.execute.call_args_list))

    def test_cleanup_rejects_paths_outside_media_directory(self):
        outside = self.root / 'keep.mp4'
        outside.write_bytes(b'keep')
        for relative in ('keep.mp4', 'ravshtok/../keep.mp4', str(outside)):
            with self.assertRaises(ValueError):
                worker.remove_media_file(relative)
        self.assertTrue(outside.exists())

    def test_deleted_during_download_cannot_be_published(self):
        connection, cursor = self.connection([])
        cursor.fetchone.return_value = ('approved', 'deleted')
        work = self.root / 'download'
        work.mkdir()
        video, poster = work / 'video.mp4', work / 'poster.webp'
        video.write_bytes(b'video')
        poster.write_bytes(b'poster')
        with patch.object(worker, 'place_media') as publish:
            self.assertFalse(worker.finish_job(connection, {'submission_id': 'test'}, video, poster, {}))
            publish.assert_not_called()
        self.assertFalse(work.exists())

    @unittest.skipUnless(os.getenv('TEST_DATABASE_URL'), 'isolated test database required')
    def test_cleanup_against_postgres(self):
        schema = 'worker_test_' + uuid4().hex
        with worker.psycopg.connect(os.environ['TEST_DATABASE_URL'], autocommit=True) as connection:
            connection.execute(f'CREATE SCHEMA {schema}')
            try:
                connection.execute(f'SET search_path TO {schema}')
                connection.execute('CREATE TABLE submissions(id text PRIMARY KEY, deleted_at timestamptz)')
                connection.execute('''CREATE TABLE ravshtok_media(submission_id text PRIMARY KEY,
                    status text,media_path text,poster_path text,expires_at timestamptz,
                    updated_at timestamptz,size_bytes bigint,lease_until timestamptz)''')
                connection.execute("INSERT INTO submissions VALUES('removed',now()),('retained',NULL)")
                connection.execute("""INSERT INTO ravshtok_media VALUES
                    ('removed','ready','ravshtok/removed.mp4','ravshtok/removed.webp',now()+interval '90 days',now(),10,NULL),
                    ('retained','ready','ravshtok/retained.mp4','',now()+interval '90 days',now(),10,NULL)""")
                for name in ('removed.mp4', 'removed.webp', 'retained.mp4'):
                    (self.output / name).write_bytes(b'test')
                self.assertEqual(worker.cleanup_expired(connection), 1)
                self.assertFalse((self.output / 'removed.mp4').exists())
                self.assertFalse((self.output / 'removed.webp').exists())
                self.assertTrue((self.output / 'retained.mp4').exists())
                self.assertEqual(connection.execute("SELECT status,media_path,size_bytes FROM ravshtok_media WHERE submission_id='removed'").fetchone(), ('expired', '', 0))
            finally:
                connection.execute(f'DROP SCHEMA {schema} CASCADE')


if __name__ == '__main__':
    unittest.main()
