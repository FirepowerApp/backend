# TODOS

## Delete `staging-offseason` overlay

- **What:** Remove `k8s/overlays/staging-offseason/`.
- **Why:** Once runtime offseason detection (branch `NelsonBlakeN/offseason-detection`)
  neutralizes the `deploy.yml` `MMDD` date-gate (finding 1A), the overlay is only a
  manual break-glass. Long-term it's dead weight.
- **Context:** Runtime detection is the single source of truth: the scheduler probes
  the live NHL API and, in `emulator` mode, uses `EMULATOR_SCHEDULE_BASE_URL` for the
  schedule fetch while the handler resolves per-task `DataSource` via
  `season.ResolveBaseURL`. `TEAM_FILTER` applies unchanged in both live and emulator
  modes — there is no separate offseason filter (the user's intentional roster covers
  it; a narrower filter was tried and explicitly rejected as unnecessary). The
  `staging-offseason` overlay is intentionally kept during this change as a fallback.
- **Depends on / blocked by:** This branch shipped AND one verified staging offseason cycle
  (real, or forced via `SEASON_OVERRIDE=offseason`) confirming the runtime path routes the
  full enqueue → poll → notify chain to the emulator.
- **Where to start:** `git rm -r k8s/overlays/staging-offseason/`; confirm
  `validate-manifests.yml` no longer references it.
