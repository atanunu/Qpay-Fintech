# Notification test scope
Run `npm --prefix Novu test` from the repository root. The Node built-in test runner needs no external packages. Every built-in event renders against a synthetic fixture; additional tests cover data validation, escaping, safe amounts, origins, expiry, scope and recipient policy seams.

The factory tests deliberately inject a local workflow/step harness. They do not import @novu/framework, run the Novu service, authenticate a real bridge, send an email, test Go financial behaviour or prove production readiness. The qualificationOnly provider is test-only and cannot be deployed. Complete [the integration acceptance matrix](../../devdocs/Novu/07-TESTING-AND-ROADMAP.md) with the actual pinned dependencies and target environment.
