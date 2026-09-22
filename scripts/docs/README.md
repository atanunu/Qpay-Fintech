# Documentation validation tools
Run from the repository root with Python 3.10 or newer and Make, or execute the underlying Python commands directly. No application dependencies, provider credentials or network access are needed.

```sh
make docs-generate
make docs-check
make docs-test
```

`render_diagrams.py --write` renders five target-design SVGs from the canonical JSON source. `check.py --write` generates service FEATURES.json files and the root/PROGRESS.json summary from canonical service README rows. Generated files must be committed with the source change. Check mode never edits files.

## Enforced locally and in the supplied workflow
`check.py --check`: four canonical registers, required service headings, unique stable IDs, permitted states/priority/milestones, unsupported completion claims, generated-file freshness and local Markdown destination existence.

`governance.py`: mandatory root sections and structure, explicit selected stacks, four runtime-capture manifests, capture metadata/hash/path policy when present, deterministic diagram source/output equality and SVG parsing. Unconfigured Mermaid blocks fail closed. Optional `--base FULL_SHA` compares git history and requires affected service and root README changes with service runtime-source/dependency changes. GitHub passes the event base via an environment variable, not shell interpolation.

`python3 -m unittest discover -s scripts/docs -p 'test_*.py' -v`: 23 regression tests at this baseline, including malformed/stale registers, missing headings/links, unsupported completion, source/docs coupling, visual metadata and deterministic rendering constraints.

## Deliberate limits
These checks do not prove source/test evidence is semantically sufficient, legal compliance, financial correctness, provider acceptance, production deployment, native compatibility or working customer journeys. The local link check validates destinations, not remote URLs or all Markdown anchor forms. Capture checking validates metadata and file integrity, not that pixels accurately represent a running app. No runtime screenshot exists at this baseline. Required branch protection, independent review and actual application CI are separate setup/work items. No OpenAPI schema or generated client exists yet to lint.

The workflow has read-only permissions and does not auto-publish generated files, deploy, make provider calls or run financial tests. Record exact local and GitHub outcomes separately in the validation report/handover.
