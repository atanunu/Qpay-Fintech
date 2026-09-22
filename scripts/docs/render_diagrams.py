#!/usr/bin/env python3
"""Render deterministic target-design SVGs, never application screenshots."""
from __future__ import annotations
import argparse
from html import escape
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]

def render(spec: dict) -> str:
    title = spec['title']
    nodes = spec['nodes']
    if not isinstance(title, str) or not title or not isinstance(nodes, list) or not 2 <= len(nodes) <= 12:
        raise ValueError('Invalid diagram title/nodes')
    for node in nodes:
        if set(node) != {'title', 'detail'} or not all(isinstance(v, str) and v for v in node.values()):
            raise ValueError('Invalid diagram node')
        if len(node['title']) > 65 or len(node['detail']) > 100:
            raise ValueError('Diagram label exceeds layout bounds')
    height = 126 + len(nodes) * 104
    parts = [f'<svg xmlns="http://www.w3.org/2000/svg" width="1040" height="{height}" viewBox="0 0 1040 {height}" role="img" aria-labelledby="title description">',
             f'<title id="title">{escape(title)}</title>',
             '<desc id="description">Approved target design, not deployment or runtime screenshot evidence.</desc>',
             '<defs><marker id="arrow" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M0,0 L8,4 L0,8 Z" fill="#247888"/></marker></defs>',
             f'<rect width="1040" height="{height}" rx="18" fill="#f3f6fa"/>',
             '<style>text{font-family:Arial,Helvetica,sans-serif} .label{font-size:21px;font-weight:bold;fill:#152740} .detail{font-size:16px;fill:#354b63}</style>',
             f'<text x="40" y="43" class="label">{escape(title)}</text>',
             '<text x="40" y="72" class="detail">Approved target design | Applications not implemented | 22 September 2026</text>']
    for i, node in enumerate(nodes):
        y = 100 + i * 104
        parts.extend([f'<rect x="40" y="{y}" width="960" height="80" rx="12" fill="white" stroke="#b8c9d9"/>',
                      f'<text x="62" y="{y+29}" class="label">{i+1:02d}  {escape(node["title"])}</text>',
                      f'<text x="62" y="{y+56}" class="detail">{escape(node["detail"])}</text>'])
        if i < len(nodes)-1:
            parts.append(f'<path d="M520,{y+82} L520,{y+101}" stroke="#247888" stroke-width="2" marker-end="url(#arrow)"/>')
    parts.append('</svg>')
    return '\n'.join(parts)+'\n'

def outputs(root: Path) -> dict[Path, str]:
    specs = json.loads((root/'docs/diagrams/diagrams.json').read_text())
    if set(specs) != {'system', 'apibackend', 'mobileapp', 'admindashboard', 'webapp'}:
        raise ValueError('Expected exactly five named target diagrams')
    return {root/'docs/diagrams'/f'{key}.svg': render(spec) for key, spec in specs.items()}

def main() -> int:
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--write', action='store_true')
    args=parser.parse_args()
    errors=[]
    for path, text in outputs(ROOT).items():
        if args.write:
            path.write_text(text)
        elif not path.is_file() or path.read_text()!=text:
            errors.append(f'Stale/missing diagram {path.relative_to(ROOT)}')
    if errors:
        print('\n'.join(errors));return 1
    print('Five target-design diagrams rendered/verified; runtime screenshots: none.')
    return 0
if __name__ == '__main__':
    raise SystemExit(main())
