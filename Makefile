.PHONY: docs-generate docs-check docs-test

docs-generate:
	python3 scripts/docs/render_diagrams.py --write
	python3 scripts/docs/check.py --write

docs-check:
	python3 scripts/docs/check.py --check
	python3 scripts/docs/governance.py

docs-test:
	python3 -m unittest discover -s scripts/docs -p 'test_*.py' -v
