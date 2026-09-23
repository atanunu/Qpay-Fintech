# Web source ownership
`api/` owns typed transport, validation, lossless money, original-operation recovery and integration-gap metadata. `review/` is an explicitly selected synthetic UI harness, never an API-error fallback. `state.tsx` owns session/resource lifecycle without persisting credentials. `components/` contains shared accessible controls and independent portal/auth shells. `pages/` implements complete customer journeys and labelled API-pending drafts. `styles/` owns responsive light/dark presentation; no external tracking/fonts are required.

The browser never calls QPay or Novu using master credentials. Review the [API mapping](../docs/API-MAPPING.md) before changing request contracts. Keep financial side effects behind explicit approval and never auto-replay writes.
