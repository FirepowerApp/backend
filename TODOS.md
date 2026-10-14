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

## Teach the game data emulator the real play-by-play shape

- **What:** Make `gameDataEmulator`'s play-by-play response emit the top-level
  fields the real NHL API sends: `gameType`, `gameState`, `periodDescriptor`,
  `clock{timeRemaining,inIntermission}`, `homeTeam.score`, `awayTeam.score`.
- **Why:** The backend now derives state, clock and score from those fields. The
  emulator (`internal/models/play.go`) serves only `maxPeriods` and `plays`, so in
  emulator mode `gameState` is empty and every push is suppressed
  (`TestProcessGameUpdate_ResponseWithoutGameFieldsSendsNothing` pins this for ordinary plays).
  Exception: a `game-end` play still makes `IsFinal` true, so an emulator game would push "Final" with
  a 0-0 score, since the emulator also omits `homeTeam.score`/`awayTeam.score`.
- **Context:** Emulator mode is staging-only and only active in the offseason, so
  nothing is affected while the season is live. It breaks the first time staging
  routes to the emulator after this ships. The emulator is a separate repo
  (`FirepowerApp-gameDataEmulator`) published as `blnelson/firepowermockdataserver`.
- **Depends on / blocked by:** Nothing. Do it before the next offseason.
- **Where to start:** `gameDataEmulator/internal/models/play.go`
  `PlayByPlayResponse`, and the generator in `internal/services/testdata.go`.

## Derive the intermission wait from `clock.secondsRemaining`

- **What:** Replace `RescheduleInterval`'s `Number >= 3 && GameType == 2` rule with
  `secondsRemaining + a small buffer`, clamped to `[MessageInterval, PeriodEndInterval]`.
- **Why:** One wake-up per intermission, exactly when it ends, and no season flag.
  `PERIOD_END_INTERVAL_SECONDS` becomes a safety ceiling instead of a guess.
- **Context:** Deliberately not done yet. No game was in intermission while this
  was investigated, so it is unverified that `secondsRemaining` is the intermission
  countdown. A wrong guess polls every intermission at the short interval, ~18
  identical pushes each (nothing deduplicates an unchanged content-state).
- **Depends on / blocked by:** Observe one live period break:
  `curl -s https://api-web.nhle.com/v1/gamecenter/<id>/play-by-play | jq .clock`
  during an intermission. Also confirm `inIntermission` is `true` then.
- **Where to start:** `watchgameupdates/internal/services/gameprocessor.go`
  `RescheduleInterval`; add `SecondsRemaining` back to `models.Clock`.

## Send structured game state instead of a formatted `gameState` string

- **What:** Push `isFinal`, `periodNumber`, `periodType`, `timeRemaining` and
  `inIntermission` and let iOS compose the label.
- **Why:** `gameState` is the one place the backend formats for display, against
  the "client owns display formatting" rule in CLAUDE.md. It also cannot tell
  `"OT"` from `"2OT"` in a playoff double overtime.
- **Context:** A wire-contract change, so it needs a coordinated iOS release. The
  iOS client currently treats only the string `"final"` (case-insensitive) as
  meaningful and renders the rest verbatim.
- **Depends on / blocked by:** An iOS release that reads the new fields.
- **Where to start:** `liveactivity/formatter.go` `contentState`, and
  `FirepowerShared/.../FirepowerActivityAttributes.swift` `ContentState`.

## Persist the kubelet log-size setting in version control

- **What:** `container-log-max-size: 100Mi` for the k3s node is set by hand in
  `/etc/rancher/k3s/config.yaml`, outside this repo.
- **Why:** `kubectl logs` serves only the current log file, and the handler
  rotates its 10Mi file in under four hours on a game night, so the previous
  evening's logs are unreadable. A rebuild of the node would silently drop the fix.
- **Context:** Verify after any rebuild with
  `kubectl get --raw "/api/v1/nodes/<node>/proxy/configz" | jq '.kubeletconfig.containerLogMaxSize'`.
  Rotated files normally still exist under `/var/log/pods/<ns>_<pod>/<container>/`.
  Restarting k3s restarts every workload on the node, including `gm-engine`, which
  is not in this repo; never rebuild the control plane to apply it.
- **Depends on / blocked by:** A place to keep node config, if one is ever created.
- **Where to start:** Add `kubelet-arg: ["container-log-max-size=100Mi"]` to the
  node's k3s config and restart k3s in a quiet window.

## Send the final push when MoneyPuck serves a malformed CSV for a finished game

- **What:** In `ProcessGameUpdate`, when `IsFinal(resp)` and the CSV fails to parse (`ErrCSVParse`),
  send the final push with xG 0 instead of retrying every minute.
- **Why:** The retry exists from #55, where a zeroed push would have read "0-0 Pregame". Score and state
  now come from the NHL document, so that reason is gone for the final push. If MoneyPuck keeps a
  completed game's file malformed, the phone stays on the last live state and never shows "Final".
- **Context:** Low probability (a persistently broken file for a finished game), and bounded by
  `ExecutionEnd` (5 hours from the scheduler). Left as-is deliberately because a transient bad file on the
  final poll would otherwise permanently show xG 0.00. The branch also hardcodes `ShouldReschedule: true`,
  skipping the `ExecutionEnd` check (the handler's `ShouldSkipExecution` bounds it by one cycle).
- **Depends on / blocked by:** Nothing.
- **Where to start:** `watchgameupdates/internal/services/gameprocessor.go`, the `ErrCSVParse` branch.

## Harden handling of the NHL play-by-play document

- **What:** (1) Treat a response whose `homeTeam`/`awayTeam`/`periodDescriptor` are missing as a failed
  fetch, not as `0-0` / `0th period`. (2) Show `End of Nth` when the last play is `period-end` at
  `00:00` even if `clock.inIntermission` is still false for a poll.
- **Why:** Both fields decode to zero values silently. (1) Only matters if the NHL renames or drops the
  fields. (2) The old code showed `00:00 left, Nth period` at the same moment, so this is not a regression.
- **Context:** Raised by the pre-merge adversarial review. Fix (1) with `*int` / pointer fields or a
  validity check next to `FetchPlayByPlay`'s decode; unit-test with a response missing `homeTeam`.
- **Depends on / blocked by:** Nothing.
- **Where to start:** `models/playbyplayresponse.go` and `FormatGameState`.

## Make sports pluggable without changing the core processor

- **What:** Restructure so that adding a sport (e.g. baseball) means adding registered
  component(s), with no diff to the core game processor.
- **Why:** The processor asks sport-neutral questions but answers them with NHL logic:
  fetch state (`FetchPlayByPlay`), is this event worth a push (`recomputeTypes`), is the
  game over (`IsFinal`), what does the state label say (`FormatGameState`), and when to
  check next (`RescheduleInterval`: intermission, `gameType`, regulation periods).
  `models.Team` mirrors the NHL schedule shape (`CommonName["default"]`). `nonCSVKeys` is
  a symptom: notifier keys double as MoneyPuck column names, so the processor has to
  subtract the keys it fills itself.
- **Context:** Directions from design discussion, not settled:
  - Notifiers declare which source each need comes from (X from MoneyPuck, Y from NHL);
    an aggregator fetches per source and hands the notifier one map. This makes source
    conflicts explicit (this branch's bug was goals coming from the wrong source).
  - The scheduler payload is not a data source; its `Team` shape is NHL-coupled.
  - Data needs differ per sport, but the send mechanism (e.g. APNs) would be nearly
    identical, so a full notifier per sport would duplicate delivery code.
  - Open: what owns the sport rules above; where derived values like `gameState` live;
    who decides when to fetch a source; per-notifier policy when one source is down.
- **Depends on / blocked by:** A second sport or stats source actually being needed. A
  Live Activity for a new sport also needs an iOS widget (`content-state` is a wire
  contract) and its own channel map (`liveactivity/channels.go` is NHL teams).
- **Where to start:** `watchgameupdates/internal/services/gameprocessor.go`
  `ProcessGameUpdate`, `internal/notification/types.go` `GetRequiredDataKeys`,
  `internal/models/payload.go`.
