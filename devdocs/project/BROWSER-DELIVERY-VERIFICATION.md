# Browser delivery verification

Source correction revision: `d59e888fc49e8e96861db05eba9ab9a1c7d76951`.

The previous normal-browser run exposed native fetch receiver binding failures, an incorrect customer capabilities field, and form/navigation timing issues. These were corrected with additive regression tests, not by removing or skipping browser journeys.

Fresh local checks after the fixes: 79 frontend unit/contract tests, TypeScript and 7 shared client-helper tests passed. The earlier GitHub backend run 35836388881 passed the full race/PostgreSQL suite, builds, helper tests and Docker Compose API smoke test. This revision reruns the complete GitHub checks; its final normal-browser/API results must be confirmed separately.

Manual in-person acceptance and actual QPay/provider qualification remain outstanding. No real-money or production operation is enabled.

[Delivery](GITHUB-DELIVERY.md) · [Web register](../../WebApp/README.md) · [API register](../../APIbackend/README.md)
