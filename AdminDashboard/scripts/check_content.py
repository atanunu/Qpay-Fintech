#!/usr/bin/env python3
"""Require the read-only admin catalogue to match the canonical Novu source."""
from pathlib import Path
import json
root=Path(__file__).resolve().parents[2]
rows=[]
for line in (root/'Novu/catalogue/emails.psv').read_text().splitlines():
    if not line.strip() or line.startswith('#'): continue
    fields=line.split('|')
    if fields[0] in ('key','id'): continue
    rows.append(dict(zip(['key','audience','scope','class','theme','guard','subject'], fields[:7])))
assert rows==json.loads((root/'AdminDashboard/src/data/templates.json').read_text()),'Admin template metadata drift; regenerate through reviewed catalogue changes'
print(f'Validated {len(rows)} canonical notification metadata entries')
