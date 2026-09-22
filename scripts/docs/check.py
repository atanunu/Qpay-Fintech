#!/usr/bin/env python3
"""Limited planning-document consistency checker. Not a financial readiness test."""
from __future__ import annotations
import argparse
import json
from pathlib import Path
import re
import sys
from urllib.parse import unquote

ROOT = Path(__file__).resolve().parents[2]
SERVICES = {'APIbackend': 'API', 'MobileApp': 'MOB', 'AdminDashboard': 'ADM', 'WebApp': 'WEB', 'Novu': 'NOT'}
STATUSES = {'Planned', 'In progress', 'Partial', 'Implemented', 'Blocked', 'Unverified', 'Deferred'}
HEADINGS = ('Purpose and boundaries', 'Architecture and visuals', 'Setup, configuration and commands',
            'Source and API map', 'Feature and task register', 'Done, pending and blocked',
            'Tests and evidence', 'Security and permissions', 'Operations, deployment and troubleshooting',
            'Roadmap, limitations and maintenance', 'Changelog')


def serialise(value: object) -> str:
    return json.dumps(value, ensure_ascii=False, indent=2) + '\n'


def collect() -> tuple[dict[str, list[dict]], list[str]]:
    errors: list[str] = []
    registers: dict[str, list[dict]] = {}
    seen: set[str] = set()
    for service, prefix in SERVICES.items():
        path = ROOT / service / 'README.md'
        if not path.is_file():
            errors.append(f'Missing {service}/README.md')
            continue
        text = path.read_text(encoding='utf-8')
        for heading in HEADINGS:
            if '\n## ' + heading + '\n' not in text:
                errors.append(f'{service}: missing heading {heading}')
        sections = re.findall(r'<!-- FEATURES:START -->(.*?)<!-- FEATURES:END -->', text, flags=re.S)
        if len(sections) != 1:
            errors.append(f'{service}: expected one register')
            continue
        rows = []
        for line in sections[0].splitlines():
            if not line.startswith('| ' + prefix + '-'):
                continue
            fields = [part.strip() for part in line.strip().strip('|').split('|')]
            if len(fields) != 8:
                errors.append(f'{service}: malformed row')
                continue
            ident, capability, milestone, priority, status, spec, acceptance, evidence = fields
            if not re.fullmatch(prefix + r'-\d{3}', ident) or ident in seen:
                errors.append(f'{service}: invalid or duplicate ID {ident}')
            seen.add(ident)
            if status not in STATUSES or priority not in {'P0', 'P1', 'P2'} or not re.fullmatch(r'M[0-6]', milestone):
                errors.append(f'{ident}: invalid status/priority/milestone')
            if not all((capability, spec, acceptance, evidence)):
                errors.append(f'{ident}: empty required field')
            if status == 'Implemented' and any(s in evidence.lower() for s in ('not implemented', 'not run', 'not created')):
                errors.append(f'{ident}: unsupported Implemented status')
            rows.append(dict(id=ident, capability=capability, milestone=milestone, priority=priority,
                             status=status, specification=spec, acceptance=acceptance, evidence=evidence))
        if not rows:
            errors.append(f'{service}: no parsed task rows')
        registers[service] = rows
    return registers, errors


def derive(registers: dict[str, list[dict]]) -> tuple[dict[Path, str], str, dict]:
    output = {}
    summary = {'basis': 'service task rows, not unique product features', 'services': {}}
    lines = ['| Service | Total tasks | Planned | In progress | Partial | Implemented | Blocked | Unverified | Deferred |',
             '|---|---:|---:|---:|---:|---:|---:|---:|---:|']
    for service, rows in registers.items():
        counts = {status: sum(row['status'] == status for row in rows) for status in sorted(STATUSES)}
        summary['services'][service] = dict(total=len(rows), counts=counts)
        output[ROOT / service / 'docs/FEATURES.json'] = serialise({'service': service,
            'canonical_source': '../README.md', 'rows': rows})
        order = ['Planned', 'In progress', 'Partial', 'Implemented', 'Blocked', 'Unverified', 'Deferred']
        lines.append('| ' + f'[{service}]({service}/README.md)' + ' | ' + str(len(rows)) + ' | ' +
                     ' | '.join(str(counts[s]) for s in order) + ' |')
    summary['total_task_rows'] = sum(len(rows) for rows in registers.values())
    output[ROOT / 'devdocs/project/PROGRESS.json'] = serialise(summary)
    return output, '\n'.join(lines), summary


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    modes = parser.add_mutually_exclusive_group(required=True)
    modes.add_argument('--write', action='store_true')
    modes.add_argument('--check', action='store_true')
    args = parser.parse_args()
    registers, errors = collect()
    if errors:
        print('\n'.join(errors), file=sys.stderr)
        return 1
    expected, table, summary = derive(registers)
    rootpath = ROOT / 'README.md'
    if not rootpath.is_file():
        print('Missing root README', file=sys.stderr)
        return 1
    roottext = rootpath.read_text(encoding='utf-8')
    pattern = r'<!-- PROGRESS:START -->.*?<!-- PROGRESS:END -->'
    if len(re.findall(pattern, roottext, flags=re.S)) != 1:
        print('Root progress markers missing/duplicated', file=sys.stderr)
        return 1
    expected[rootpath] = re.sub(pattern, '<!-- PROGRESS:START -->\n' + table + '\n<!-- PROGRESS:END -->', roottext, flags=re.S)
    for path, content in expected.items():
        if args.write:
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(content, encoding='utf-8')
        elif not path.exists() or path.read_text(encoding='utf-8') != content:
            errors.append(f'Stale/missing generated output: {path.relative_to(ROOT)}')
    destinations = 0
    for md in ROOT.rglob('*.md'):
        text = md.read_text(encoding='utf-8')
        for raw in re.findall(r'\]\(([^)]+)\)', text):
            url = raw.strip().split(' "', 1)[0]
            if re.match(r'^[a-zA-Z][a-zA-Z0-9+.-]*:', url) or url.startswith('#'):
                continue
            url = unquote(url.split('#', 1)[0])
            if not url:
                continue
            path = (md.parent / url).resolve()
            if not path.is_relative_to(ROOT.resolve()) or not path.exists():
                errors.append(f'{md.relative_to(ROOT)}: missing/outside local destination {url}')
            destinations += 1
    if errors:
        print('\n'.join(errors), file=sys.stderr)
        return 1
    print(serialise({'result': 'passed', 'mode': 'write' if args.write else 'check',
                     'services': len(registers), 'task_rows': summary['total_task_rows'],
                     'local_destination_references_checked': destinations,
                     'application_tests_run': 0, 'mermaid_rendering_checked': False,
                     'screenshots_validated': False, 'github_enforcement_installed': False}))
    return 0

if __name__ == '__main__':
    raise SystemExit(main())
