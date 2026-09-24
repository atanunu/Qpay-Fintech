# Source ownership

main.tsx owns routes; auth.tsx owns isolated staff access; layout.tsx and modules.ts own permission-aware navigation. api.ts is the only real transport. review.ts is a compile-time separate, explicitly synthetic adapter. components/ui.tsx owns accessible form/table/dialog/error primitives. pages implement actual management journeys. Never add financial authority or provider secrets to browser code. Update the service/root README, tests and changed runtime captures with every source change.
