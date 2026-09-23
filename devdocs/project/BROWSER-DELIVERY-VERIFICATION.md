# Browser delivery verification

Source correction revisions: `d59e888fc49e8e96861db05eba9ab9a1c7d76951` and `4458128d1db3c4bb6f486116bf541b21af31e289`.

Normal-browser tests exposed native fetch receiver binding, an incorrect customer capabilities field, and asynchronous form-readiness races. Both clients now bind native fetch correctly; capability display uses the backend payments field and fails closed for missing/unknown values; bank and bill quote actions wait for their selected records. The recovery test waits for the actual destination screen rather than only its changed URL. No assertions were removed, and automatic browser retries remain disabled.

## Evidence before the final rerun

Local: 79 frontend unit/contract tests, TypeScript and seven shared client tests passed. GitHub backend run 35836388881 passed the complete race/PostgreSQL suite, builds and Docker Compose API smoke test.

GitHub web run 35837962795 on source head `939bccb5e8ca2c3ffd36561c08d1ed66f9bf14cf` passed all 11 actual Go/PostgreSQL browser tests, including private uploads, request/schedule authorisation, real cryptographic WebAuthn and outage/production-submission guards. Its screenshot sweep captured all 38 screens with accessibility and overflow assertions passing. Overall synthetic-review result was 18 passed, two failed: the final form-readiness corrections above address those failures. The complete suite must be green on a subsequent revision before final delivery acceptance.

Manual in-person acceptance and actual QPay/provider qualification remain outstanding. No real-money or production operation is enabled.

[Delivery](GITHUB-DELIVERY.md) · [Web register](../../WebApp/README.md) · [API register](../../APIbackend/README.md)
