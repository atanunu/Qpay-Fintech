# Target design diagrams
These images describe the approved design, not running services or implemented features. `diagrams.json` is canonical; `scripts/docs/render_diagrams.py` renders deterministic self-contained SVGs. Run `python3 scripts/docs/render_diagrams.py --write` after editing the source and run the documentation checks. No external images, fonts, links or customer data are embedded.

[System](system.svg) · [APIbackend](apibackend.svg) · [MobileApp](mobileapp.svg) · [AdminDashboard](admindashboard.svg) · [WebApp](webapp.svg)

Real product screenshots remain absent until applications exist. The separate service capture manifests must record actual source revision, environment, route, fixture, device and hash. These design SVGs must never be entered as runtime captures.
