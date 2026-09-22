"""Regression tests for the limited planning checker, never application tests."""
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest
ROOT = Path(__file__).resolve().parents[2]

class DocumentationChecks(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.copy = Path(self.tmp.name) / 'repo'
        shutil.copytree(ROOT, self.copy, ignore=shutil.ignore_patterns('__pycache__'))
    def tearDown(self):
        self.tmp.cleanup()
    def run_check(self):
        return subprocess.run([sys.executable, str(self.copy/'scripts/docs/check.py'), '--check'],
                              capture_output=True, text=True, timeout=15)
    def change(self, relative, old, new):
        p=self.copy/relative
        p.write_text(p.read_text(encoding='utf-8').replace(old,new,1),encoding='utf-8')
    def test_valid_pack_passes(self):
        result=self.run_check()
        self.assertEqual(result.returncode,0,result.stderr)
    def test_missing_service_readme_fails(self):
        (self.copy/'MobileApp/README.md').unlink()
        self.assertNotEqual(self.run_check().returncode,0)
    def test_stale_generated_json_fails(self):
        (self.copy/'APIbackend/docs/FEATURES.json').write_text('{}\n')
        self.assertNotEqual(self.run_check().returncode,0)
    def test_duplicate_id_fails(self):
        self.change('APIbackend/README.md','| API-002 |','| API-001 |')
        self.assertIn('duplicate ID',self.run_check().stderr)
    def test_missing_local_link_fails(self):
        self.change('README.md','(SECURITY.md)','(MISSING.md)')
        self.assertIn('missing/outside local destination',self.run_check().stderr)
    def test_missing_required_heading_fails(self):
        self.change('WebApp/README.md','## Tests and evidence','## Tests removed')
        self.assertIn('missing heading',self.run_check().stderr)
    def test_unsubstantiated_implemented_fails(self):
        self.change('APIbackend/README.md','| Planned |','| Implemented |')
        self.assertIn('unsupported Implemented',self.run_check().stderr)
    def test_invalid_status_fails(self):
        self.change('AdminDashboard/README.md','| Planned |','| Magic |')
        self.assertIn('invalid status',self.run_check().stderr)

if __name__ == '__main__':
    unittest.main()
