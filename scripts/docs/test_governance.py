from __future__ import annotations
import copy
import hashlib
import importlib.util
from pathlib import Path
import shutil
import sys
import tempfile
import unittest
sys.path.insert(0,str(Path(__file__).parent))
import governance
from render_diagrams import render

class GovernanceTests(unittest.TestCase):
    def setUp(self):
        self.tmp=tempfile.TemporaryDirectory();self.addCleanup(self.tmp.cleanup)
        self.root=Path(self.tmp.name)
        self.m=self.root/'WebApp/docs/SCREENSHOTS.json';self.m.parent.mkdir(parents=True)
        self.data={'schema_version':1,'service':'WebApp','status':'not_captured','reason':'Not implemented','captures':[]}
    def test_honest_empty_manifest(self):self.assertEqual(governance.capture_errors(self.root,self.m,self.data),[])
    def test_empty_captured_manifest_fails(self):
        self.data['status']='captured';self.assertTrue(governance.capture_errors(self.root,self.m,self.data))
    def test_uncaptured_needs_reason(self):
        self.data.pop('reason');self.assertTrue(governance.capture_errors(self.root,self.m,self.data))
    def capture(self):
        # Synthetic bytes test manifest integrity, not image-decoder correctness.
        p=self.m.parent/'screen.png';p.write_bytes(b'synthetic-image-fixture')
        self.data['status']='captured';self.data['captures']=[dict(path='screen.png',screen='/wallet',source_commit='a'*40,
            captured_at='2026-09-22T12:00:00Z',environment='documentation',fixture_id='synthetic-v1',state='empty',
            viewport='1280x800',sha256=hashlib.sha256(p.read_bytes()).hexdigest())]
        return self.data['captures'][0]
    def test_complete_provenance(self):
        self.capture();self.assertEqual(governance.capture_errors(self.root,self.m,self.data),[])
    def test_wrong_hash_fails(self):
        self.capture()['sha256']='0'*64;self.assertTrue(governance.capture_errors(self.root,self.m,self.data))
    def test_production_capture_fails(self):
        self.capture()['environment']='production';self.assertTrue(governance.capture_errors(self.root,self.m,self.data))
    def test_unsafe_path_fails(self):
        self.capture()['path']='../../../../outside.png';self.assertTrue(governance.capture_errors(self.root,self.m,self.data))
    def test_bad_timestamp_fails(self):
        self.capture()['captured_at']='2026-09-22';self.assertTrue(governance.capture_errors(self.root,self.m,self.data))
    def test_bad_commit_fails(self):
        self.capture()['source_commit']='main';self.assertTrue(governance.capture_errors(self.root,self.m,self.data))
    def test_runtime_source_requires_both_readmes(self):
        self.assertEqual(len(governance.coupled_changes(['WebApp/src/app.tsx'])),2)
    def test_source_with_readmes_passes(self):
        self.assertEqual(governance.coupled_changes(['WebApp/src/app.tsx','WebApp/README.md','README.md']),[])
    def test_docs_only_does_not_need_source_evidence(self):
        self.assertEqual(governance.coupled_changes(['WebApp/docs/README.md']),[])
    def test_renderer_escapes_text(self):
        spec={'title':'A & B','nodes':[{'title':'<safe>','detail':'one'},{'title':'two','detail':'three'}]}
        result=render(spec);self.assertIn('A &amp; B',result);self.assertIn('&lt;safe&gt;',result)
    def test_renderer_rejects_clipped_labels(self):
        with self.assertRaises(ValueError):render({'title':'ok','nodes':[{'title':'x'*80,'detail':'y'}]*2})
    def test_missing_root_headings_fail(self):
        self.assertTrue(any('Root README missing' in x for x in governance.validate(self.root)))

if __name__=='__main__':unittest.main()
