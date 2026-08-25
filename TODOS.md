# TODOS

## Delete `staging-offseason` overlay + re-home its TEAM_FILTER

- **What:** Remove `k8s/overlays/staging-offseason/` and relocate its offseason
  `TEAM_FILTER` widening.
- **Why:** Once runtime offseason detection (branch `NelsonBlakeN/offseason-detection`)
  neutralizes the `deploy.yml` `MMDD` date-gate (finding 1A), the overlay is only a
  manual break-glass. Long-term it's dead weight and a second offseason config to keep
  in sync with `OFFSEASON_TEAM_FILTER`.
- **Context:** Runtime detection becomes the single source of truth: the scheduler probes
  the live NHL API, and in `emulator` mode uses `EMULATOR_SCHEDULE_BASE_URL` +
  `OFFSEASON_TEAM_FILTER`; the handler resolves per-task `DataSource` via
  `season.ResolveBaseURL`. The `staging-offseason` overlay is intentionally kept during
  this change as a fallback. Its CHI-inclusive `TEAM_FILTER` was the seed for
  `OFFSEASON_TEAM_FILTER`.
- **Depends on / blocked by:** This branch shipped AND one verified staging offseason cycle
  (real, or forced via `SEASON_OVERRIDE=offseason`) confirming the runtime path routes the
  full enqueue → poll → notify chain to the emulator.
- **Where to start:** `git rm -r k8s/overlays/staging-offseason/`; confirm
  `validate-manifests.yml` no longer references it; confirm `OFFSEASON_TEAM_FILTER` in the
  staging ConfigMap covers the emulator's first-week slate.
