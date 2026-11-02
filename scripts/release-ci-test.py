import importlib.util
import json
import tempfile
import unittest
from pathlib import Path

spec = importlib.util.spec_from_file_location('release_ci', Path(__file__).with_name('release-ci.py'))
ci = importlib.util.module_from_spec(spec)
spec.loader.exec_module(ci)


class ReleaseTests(unittest.TestCase):
    def test_next_patch_ignores_other_series_and_prereleases(self):
        self.assertEqual(ci.next_tag(['v0.0.9', 'v0.0.12', 'v0.0.13-rc.1'], 0, 0), 'v0.0.13')

    def test_same_commit_reuses_existing_tag_and_rejects_ambiguity(self):
        self.assertEqual(ci.select_tag(['v0.0.12'], ['v0.0.12'], 0, 0), 'v0.0.12')
        with self.assertRaises(ValueError):
            ci.select_tag(['v0.0.12', 'v0.0.13'], ['v0.0.12', 'v0.0.13'], 0, 0)

    def test_registry_seeds_patch_and_rejects_wrong_series_retry(self):
        self.assertEqual(ci.select_tag([], [], 0, 0, '0.0.12'), 'v0.0.13')
        self.assertEqual(ci.select_tag(['v0.0.9'], [], 0, 0, '0.0.12'), 'v0.0.13')
        with self.assertRaisesRegex(ValueError, 'series'):
            ci.select_tag(['v0.1.0'], ['v0.1.0'], 0, 0, '0.0.12')
        with self.assertRaisesRegex(ValueError, 'series'):
            ci.select_tag(['v0.1.0'], [], 0, 0, '0.0.12')

    def test_newest_tag_must_be_ancestor_of_source(self):
        calls = []
        def ancestor(tag, source):
            calls.append((tag, source))
            return tag == 'v0.0.12' and source == 'new'
        ci.assert_current_source(['v0.0.9', 'v0.0.12'], 'new', ancestor)
        self.assertEqual(calls, [('v0.0.12', 'new')])
        for source in ('old', 'diverged'):
            with self.assertRaisesRegex(ValueError, 'ancestor'):
                ci.assert_current_source(['v0.0.9', 'v0.0.12'], source, ancestor)
        ci.assert_current_source([], 'old', ancestor)

    def test_published_checksum_must_match_local_build(self):
        with tempfile.TemporaryDirectory() as tmp:
            local = Path(tmp) / 'local'
            remote = Path(tmp) / 'remote'
            local.write_text('original checksum')
            remote.write_text('different checksum')
            with self.assertRaisesRegex(ValueError, 'checksum'):
                ci.verify_checksums(local, remote)
            remote.write_text('original checksum')
            ci.verify_checksums(local, remote)

    def test_registry_never_moves_backwards_and_preserves_other_data(self):
        data = {'plugins': [{'id': 'quota-reset', 'version': '0.0.14', 'name': 'keep'}]}
        self.assertFalse(ci.update_registry(data, '0.0.13'))
        self.assertTrue(ci.update_registry(data, '0.1.0'))
        self.assertEqual(data['plugins'][0], {'id': 'quota-reset', 'version': '0.1.0', 'name': 'keep'})

    def test_stamp_requires_single_matching_fields(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / 'internal/runtime').mkdir(parents=True)
            (root / 'internal/management').mkdir(parents=True)
            runtime = root / 'internal/runtime/runtime.go'
            page = root / 'internal/management/page.go'
            runtime.write_text('Version:          "0.0.12",')
            page.write_text('<span class="version-badge">v0.0.12</span>')
            ci.stamp(root, '0.0.13')
            self.assertIn('"0.0.13"', runtime.read_text())
            self.assertIn('>v0.0.13</span>', page.read_text())
            page.write_text('missing badge')
            with self.assertRaises(ValueError):
                ci.stamp(root, '0.0.14')


if __name__ == '__main__':
    unittest.main()
