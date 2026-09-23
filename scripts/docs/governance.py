#!/usr/bin/env python3
"""Repository governance checks, not financial or legal acceptance."""
from __future__ import annotations
import argparse
from datetime import datetime
import hashlib
import json
from pathlib import Path
import re
import subprocess
import sys
import xml.etree.ElementTree as ET
from render_diagrams import outputs

ROOT=Path(__file__).resolve().parents[2]
SERVICES=('APIbackend','MobileApp','AdminDashboard','WebApp','Novu')
HEADINGS=('Product and operating scope','Applications and approved stacks','Architecture and integration',
          'Images and visual evidence','Cross-service progress','Documentation and open decisions',
          'Roadmap and task tracking','Setup and configuration','Tests, CI and release readiness',
          'Security and contributions','Limitations and launch blockers','Changelog')
REQUIRED=('AGENTS.md','CONTRIBUTING.md','SECURITY.md','.github/CODEOWNERS',
          '.github/workflows/documentation.yml','.github/pull_request_template.md',
          'devdocs/adrs/0001-APPROVED-ARCHITECTURE.md','devdocs/adrs/0002-DOCUMENTATION-AND-EVIDENCE.md',
          'contracts/openapi/README.md','contracts/events/README.md','contracts/fixtures/README.md',
          'infra/docker/README.md','infra/observability/README.md','infra/runbooks/README.md',
          'tests/acceptance/README.md','APIbackend/cmd/api/README.md',
          'APIbackend/cmd/worker/README.md','APIbackend/cmd/scheduler/README.md',
          'APIbackend/internal/README.md','APIbackend/migrations/README.md')

def capture_errors(root: Path, manifest: Path, data: dict) -> list[str]:
    errors=[]
    service=manifest.parent.parent.name
    if data.get('schema_version') != 1 or data.get('service') != service:
        errors.append(f'{service}: invalid screenshot manifest identity')
    captures=data.get('captures')
    status=data.get('status')
    if not isinstance(captures,list) or status not in ('not_captured','captured'):
        return errors+[f'{service}: invalid screenshot status/captures']
    if status=='not_captured' and (captures or not data.get('reason')):
        errors.append(f'{service}: uncaptured manifest needs reason and empty captures')
    if status=='captured' and not captures:
        errors.append(f'{service}: captured manifest is empty')
    seen=set()
    for item in captures:
        if not isinstance(item,dict):
            errors.append(f'{service}: capture must be object');continue
        required=('path','screen','source_commit','captured_at','environment','fixture_id','state','viewport','sha256')
        if any(not item.get(k) for k in required):
            errors.append(f'{service}: incomplete capture provenance');continue
        path=(manifest.parent/str(item['path'])).resolve()
        if not path.is_relative_to(root.resolve()) or not path.is_file() or path.suffix.lower() not in ('.png','.jpg','.jpeg','.webp'):
            errors.append(f'{service}: invalid/missing runtime image');continue
        if str(path) in seen:
            errors.append(f'{service}: duplicate capture path')
        seen.add(str(path))
        if not re.fullmatch(r'[0-9a-f]{40}',str(item['source_commit'])) or set(str(item['source_commit']))=={'0'}:
            errors.append(f'{service}: invalid source commit')
        try:
            dt=datetime.fromisoformat(str(item['captured_at']).replace('Z','+00:00'))
            if dt.utcoffset() is None or dt.utcoffset().total_seconds()!=0:
                raise ValueError('UTC required')
        except (ValueError,TypeError):
            errors.append(f'{service}: invalid UTC capture timestamp')
        if str(item['environment']).lower() not in ('local','test','staging','documentation'):
            errors.append(f'{service}: use a non-production synthetic capture environment')
        if hashlib.sha256(path.read_bytes()).hexdigest()!=item['sha256']:
            errors.append(f'{service}: capture hash mismatch')
    return errors

def coupled_changes(paths: list[str]) -> list[str]:
    errors=[]
    for service in SERVICES:
        changed=[p for p in paths if p.startswith(service+'/')]
        source=[p for p in changed if not '/docs/' in p and
                (p.endswith(('.go','.ts','.tsx','.js','.jsx','.mjs','.psv','.css','.sql')) or
                 Path(p).name in ('go.mod','go.sum','package.json','package-lock.json','pnpm-lock.yaml'))]
        if source and f'{service}/README.md' not in paths:
            errors.append(f'{service}: source change requires canonical README update')
        if source and 'README.md' not in paths:
            errors.append(f'{service}: source change requires root README update')
    return errors

def validate(root: Path) -> list[str]:
    errors=[]
    for rel in REQUIRED:
        if not (root/rel).is_file(): errors.append(f'Missing required {rel}')
    rootread=root/'README.md'
    text=rootread.read_text() if rootread.is_file() else ''
    for h in HEADINGS:
        if '\n## '+h+'\n' not in text: errors.append(f'Root README missing heading: {h}')
    for service in SERVICES:
        for rel in ('README.md','docs/README.md','docs/SCREENSHOTS.json','tests/README.md'):
            if not (root/service/rel).is_file(): errors.append(f'Missing {service}/{rel}')
        m=root/service/'docs/SCREENSHOTS.json'
        if m.is_file():
            try: errors+=capture_errors(root,m,json.loads(m.read_text()))
            except (ValueError,TypeError): errors.append(f'{service}: unreadable capture manifest')
        readme=root/service/'README.md'
        if readme.is_file() and '**Approved stack:**' not in readme.read_text():
            errors.append(f'{service}: selected stack not explicit')
    for md in root.rglob('*.md'):
        if any(part in {'.git', 'node_modules', 'dist', 'test-results', 'playwright-report'} for part in md.parts): continue
        if '.git' in md.parts:continue
        if re.search(r'^```mermaid\s*$',md.read_text(),re.M):
            errors.append(f'{md.relative_to(root)}: Mermaid renderer not configured; use validated diagrams')
    try:
        for path,expected in outputs(root).items():
            if not path.is_file() or path.read_text()!=expected:
                errors.append(f'Stale/missing rendered diagram: {path.relative_to(root)}');continue
            ET.fromstring(path.read_text())
    except (ValueError,KeyError,ET.ParseError,FileNotFoundError,TypeError) as exc:
        errors.append(f'Diagram validation failed: {exc}')
    return errors

def main() -> int:
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--base', help='Optional trusted git base SHA for source/doc coupling')
    args=parser.parse_args()
    errors=validate(ROOT)
    if args.base and set(args.base)!={'0'}:
        if not re.fullmatch(r'[0-9a-f]{40}',args.base):errors.append('Invalid base revision')
        else:
            try:
                paths=subprocess.check_output(['git','diff','--name-only',args.base,'HEAD','--'],cwd=ROOT,text=True).splitlines()
                errors+=coupled_changes(paths)
            except subprocess.CalledProcessError:errors.append('Cannot compare requested base revision')
    if errors:
        print('\n'.join(errors),file=sys.stderr);return 1
    print(json.dumps({'result':'passed','root_and_service_governance':'checked','target_diagrams':5,
                      'runtime_screenshot_manifests':len(SERVICES),'source_doc_coupling':bool(args.base),
                      'application_tests_run':0,'branch_protection_verified':False},indent=2))
    return 0
if __name__=='__main__':raise SystemExit(main())
