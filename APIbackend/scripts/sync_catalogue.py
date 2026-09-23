#!/usr/bin/env python3
"""Mirror the canonical Novu catalogue for Go embedding. Never edit the copy."""
from pathlib import Path
import sys
root = Path(__file__).resolve().parents[2]
source = root / 'Novu/catalogue/emails.psv'
target = root / 'APIbackend/internal/service/catalogue/emails.psv'
if '--check' in sys.argv:
    if source.read_bytes() != target.read_bytes():
        raise SystemExit('Embedded catalogue drift: run python3 APIbackend/scripts/sync_catalogue.py')
else:
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_bytes(source.read_bytes())
