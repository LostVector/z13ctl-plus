# z13ctl-plus — audit + verification (2026-10-10)

Provenance: audit pass by qwen-flash-next; adversarial verification by two glm-5.3
reviewer subagents (read-only, isolated context). Result: 14/15 confirmed, 1 partly
right (corrected below), 0 refuted, +5 additional findings from the verification
pass. Line numbers were re-resolved by the verifier and are accurate as of commit
`65fc107`.

Fix order (updated after the opus 5.5 pass): 23, 21, 22, 1, 2, 16, 17 first, then the
rest. **Read "Post-verification corrections" at the bottom before fixing — five of
the fix sketches above are wrong (#1, #3, #4, #20) and #19 is refuted.** Commit per
finding; run `make test && make lint` each time.

---

## Original audit findings

### 1. `brightness` (all devices) leaves stale per-device state — reverts on restart [MUST FIX]
- Verified: CONFIRMED. `internal/daemon/server.go:526-533` — the `req.Device == ""`
  branch sets `d.state.Lighting.Brightness/Enabled` and saves, but does not clear
  `d.state.Devices`. `handleApply` does clear it (`server.go:441-442`), as does
  `handleOff` (`server.go:487-488`). `applyLightingState` (`daemon.go:290`) prefers
  `Devices[name]` over `Lighting` whenever `Devices` is non-empty.
- Repro: `apply --device lightbar --color red` then `brightness high` (no --device)
  → hardware updated, command OK, but on daemon restart the lightbar reverts to the
  old brightness/mode.
- Fix: add `d.state.Devices = nil` in the `req.Device == ""` branch (one line,
  mirroring `handleApply`).

### 2. Data race on `d.dev` between resume/lid handlers and keyboard hotplug [MUST FIX]
- Verified: CONFIRMED. Write under `d.mu`: `daemon.go:274-277` (`reopenAndRestore`).
  Unlocked reads: `resume.go:89-90` (`handlePrepareForSleep` holds no lock),
  `resume.go:134` (`restoreVolatileState` holds only `applyMu`), `resume.go:166`
  (`restoreLighting`), and `daemon.go:296,311` when `applyLightingState` is called
  from those paths. Socket handlers read `d.dev` under `d.mu` — safe.
- Effect: race-detector-flaggable; resume/lid restore can write to a just-closed
  device (EBADF, lighting stays off until the next event).
- Fix + TRAP: snapshot `d.mu.Lock(); dev := d.dev; d.mu.Unlock()` at the top of the
  three resume paths and pass `dev` down as a parameter. Do NOT take `d.mu` inside
  `applyLightingState` — it is already called WITH `d.mu` held from
  `reopenAndRestore` (`daemon.go:268→279`); that self-deadlocks.

### 3. No CI runs tests/lint; all hardware handlers at 0% coverage [MUST FIX]
- Verified: CONFIRMED. `.github/workflows/` contains only `release.yml` (tag push +
  workflow_dispatch). Measured coverage: `handleApply`, `handleOff`,
  `handleFanCurve`, `handleTDP`, `handleTDPReset`, `savePreset`, `applyPreset`,
  `setPowerPolicy`, `watchPowerPolicy`, `Run` all 0.0%; `dispatch` 8.6%; total 26.0%.
- Fix: ~15-line `ci.yml` (`on: [push, pull_request]`, setup-go from `go.mod`,
  `go build ./... && go test -race ./... && make lint`), then table tests over
  `dispatch(request{...})` — it is a plain function; no hardware needed
  (see `internal/daemon/server_test.go:5` for the pattern).

### 4. No socket deadlines — one blocked command hangs every client [SHOULD FIX]
- Verified: CONFIRMED. Zero `Deadline` calls in `api/client.go` or
  `internal/daemon/`: dial-only timeout (`client.go:78`, `:376`), server
  `handleConn` (`server.go:66`) sets none. `dispatch` serializes hardware commands
  on `applyMu` (`server.go:94-97`, `262-275`); even `get-state` stalls because
  `handleApply` holds `d.mu` during HID writes.
- Case: a sysfs write into an unresponsive EC stalls one handler → every CLI call
  blocks forever, silently.
- Fix: `conn.SetDeadline(time.Now().Add(10*time.Second))` after dial in
  `sendCommand` and `Subscribe`; read deadline on accepted conns in `handleConn`
  (non-subscribe path).

### 5. High-TDP fan safety replaces a *stronger* user curve [SHOULD FIX]
- Verified: CONFIRMED. `tuning.go:35` (`FanSafety = PL1SPL > TDPMaxSafe`), `:37-38`
  unconditionally sets `plan.Fan = HighTDPFanCurve()` — no clamp exists anywhere.
  `server.go:698-702`: under FanSafety the user curve is stored, persisted, and
  `OK:true` returned, but never written. Comments `fan.go:28-29` and `:176-178`
  promise a floor ("higher but not lower"), the code does full replacement.
- Case: `tdp --set 80 --force` then a 100%-everywhere curve → fans stay 204-255
  instead of 255, CLI prints success.
- Fix: in `planTuning`, when FanSafety and a user curve exists, take the per-point
  max with `HighTDPFanCurve()` (or skip the override when every user PWM ≥
  `HighTDPMinPWM` = 204).

### 6. `--dry-run tdp` prints actions the code never performs [SHOULD FIX]
- Verified: CONFIRMED. `dryrun.go:162-163` prints "all fans to full speed
  (pwm_enable=0)" when `force && (pl1||pl2||pl3 > 75)` — real trigger is pl1 only
  (`tuning.go:35`, `cmd/tdp.go:153`) and the real action is the custom-mode 80%+
  curve (`pwm_enable=1`). `dryrun.go:177` prints "switch profile to balanced" but
  reset re-applies the CURRENT profile (`server.go:857-868`, `cmd/tdp.go:180-184`).
- Fix: mirror the real trigger/action in `DryRunTdp`; drop the wrong line in
  `DryRunTdpReset`.

### 7. Wire protocol structs duplicated daemon↔api with no drift guard [SHOULD FIX]
- Verified: CONFIRMED. `server.go:34-65` vs `api/client.go:43-75` are currently
  identical field-for-field; `server.go:72` uses plain `json.Unmarshal` (no
  `DisallowUnknownFields` anywhere in the repo); no test compares the two.
- Failure mode: a field added to one side only (or a typo'd tag) is silently
  dropped — command returns `ok:true` and does nothing.
- Fix: ~15-line daemon test unmarshalling a JSON doc with every documented key
  into `request` via `json.DisallowUnknownFields()`; or export the structs from
  `api` and delete the daemon copies (~-45 lines, needs an `api/v1.6.0` tag).

### 8. `setup` discards every chgrp/chmod error, still prints success [SHOULD FIX]
- Verified: CONFIRMED. `setup.go:95-96` (`_ = exec.Command(...)`), `:234-235`
  unconditional "Applied sysfs permissions.". Note: when chgrp fails, the
  subsequent `chmod g+w` widens the *root* group's bits.
- Case: `sudo z13ctl-plus setup --group=nonexistent` → rules + service written with
  the bad group, zero permissions granted, "Done." printed.
- Fix: count and print failures; only print success at count 0. Prefer
  `os.Chmod`/`os.Chown` over two subprocesses per file.

### 9. `SetProfile` can return nil without writing anything [SHOULD FIX]
- Verified: CONFIRMED. `sysfs.go:47-68`: entries without a readable `profile` file
  are skipped (`:56-58`); `primaryErr` is set only when `p == primaryPath` (`:60-63`).
  If `/sys/class/platform-profile` exists but no entry has a `profile` file: zero
  writes, `primaryErr` nil, `setPPD` still runs, returns nil. Caller
  (`daemon.go:95-101`) logs "profile tuning restored".
- Fix: track whether any write was attempted; error (naming the path) if none.

### 10. `--group=users` default grants SMU/PPT/cpufreq writes to all human users [SHOULD FIX]
- Verified: CONFIRMED (code side). Grant list from `setup.go:36-72`, `:104-137`:
  hidraw 0b05:18c6/1a30, platform-profile, battery threshold, input event node,
  firmware-attributes, hwmon pwm*, `ppt_*`, cpufreq boost/min-freq/EPP, and
  `/sys/kernel/ryzen_smu_drv/{smu_args,mp1_smu_cmd,rsmu_cmd}` (undervolt to -40,
  TDP to 93W — reachable over plain sysfs, works via SSH). Default at
  `setup.go:265`.
- Caveat: severity is install-dependent — on a single-user machine `users` holds
  one account; on multi-user Arch it is every human user.
- Fix: dedicated group default (or the invoking user's group) + a warning whenever
  the group is `users`/`wheel`.

### 11. TDP safety rule implemented twice [NICE TO HAVE]
- Verified: CONFIRMED equivalent today: `cmd/tdp.go:102-127` vs
  `server.go:782-800` (~20×2 lines); `cli.SetTDP` does no validation.
- Fix: one `cli.ValidateTDP(pl1, pl2, pl3 int, force bool) error`, called from both
  (~-30 lines).

### 12. `get-state` spawns kscreen-doctor + a D-Bus call every time [NICE TO HAVE]
- Verified: CONFIRMED. Chain `server.go:222` → `refresh.go:61-78` →
  `refresh.go:152-172` (fresh exec + `dbus.SessionBus()` + Properties.Get per
  call). No cache in refresh.go. Measured ~33 ms/call.
- Fix: 1-second TTL cache inside `ReadRefreshRate` only (keep
  `ReadInternalDisplay` uncached — the display coordinator needs fresh reads).

### 13. `readTrimmed` defined twice, identical bodies [NICE TO HAVE]
- Verified: CONFIRMED. `server.go:646-653` and `cpu_power.go:130-137`.
- Fix: export `cli.ReadTrimmedFile`, delete the daemon copy (-5 lines).

### 14. CPU utilisation delta noisy with two concurrent pollers [NICE TO HAVE, CORRECTED]
- Verified: PARTLY RIGHT. Mechanism confirmed (`telemetry.go:322-352`: shared
  snapshot under `cpuUtilMu`, `dt==0` → 0, integer division → 0/100 on tiny dt).
  CORRECTION: `z13ctl-plus status` reads sysfs directly in the CLI process
  (`cmd/status.go:86`), NOT through the daemon — the interference case is two
  concurrent `get-state` clients (GUI + Decky) in the daemon.
- Fix: return the previous value when the interval is under ~200 ms (3 lines).

### 15. `restoreVolatileState` splits lock/unlock across branches [NICE TO HAVE]
- Verified: CONFIRMED. `resume.go:126-131`: `defer d.applyMu.Unlock()` exists only
  in the `ActivePreset == ""` branch; the other branch manually unlocks at `:143`
  before `d.applyPreset` (which locks at `preset.go:193-194`). All current paths
  unlock correctly; any new early return in the else-branch leaks `applyMu` and
  wedges every hardware command. Unique pattern — every other site is Lock+defer.
- Fix: restructure so the unlock sits next to the lock (e.g. helper that takes
  applyMu, or hoist the preset branch to the caller).

---

## Additional findings from the glm-5.3 verification pass

### 16. `z13ctl-plus status` always prints 0% CPU utilisation [BUG — fix early]
- `cmd/status.go:86` calls `cli.ReadTelemetry()` once in a fresh process; the first
  `ReadCPUUtilization` returns 0 (`telemetry.go:339` `if !cpuUtilHasPrev`).
  Repro: any load → `CPU: XX°C, 0%, ...` every time.
- Fix: prime the snapshot (read once, brief sleep, read again) or derive from
  /proc/loadavg for the one-shot CLI path.

### 17. One stuck subscriber freezes event delivery for all clients [SHOULD FIX]
- `daemon.go:219`: `c.Write(data)` under `subMu` with no write deadline; a
  subscriber that stops reading (full socket buffer) blocks `broadcastLoop` and the
  ctx-done cleanup → `gui-toggle` stops for every GUI.
- Fix: `SetWriteDeadline` before the write, drop the subscriber on timeout/error.

### 18. CLI direct fallback never lifts the high-TDP fan curve [SHOULD FIX]
- `cmd/tdp.go:153-165`: daemon-less `tdp --set 80 --force` locks fans to the 204+
  curve; a later `tdp --set 60` writes PPT but leaves `pwm_enable=1` + the safety
  curve active indefinitely. The daemon handles this transition
  (`server.go:828-836`); the CLI fallback has no counterpart, and
  `runTdpReset`'s `SetProfile` does not touch fan modes.
- Fix: mirror the daemon's oldSafety/plan transition in the direct-fallback path.

### 19. `kscreenCommand` env merge order makes it ineffective [SHOULD FIX]
- `refresh.go:166-171`: `cmd.Env = append(cmd.Env, values...)` puts fresh
  systemd-manager vars AFTER `os.Environ()`; getenv is first-match, so a stale
  socket-activation value (e.g. old `DISPLAY` after re-login) shadows the fresh one
  — the exact scenario the merge exists for.
- Fix: prepend the manager values, or strip existing keys with the same name first.

### 20. PPT apply is 5 sequential writes with no rollback [NICE TO HAVE]
- `tdp.go:72+`, `server.go:820-827`: a mid-sequence failure (e.g. `ppt_pl2_sppt`
  after `ppt_pl1_spl` succeeded) restores fans only; hardware keeps a partially
  applied limit set while the client sees a clean error.
- Fix: read current values before writing; restore written ones on failure.

---

## Minor
- `AGENTS.md` lists `internal/daemon/client.go` ("Redirect comment only") — the
  file does not exist. Doc drift.

## Not checked by either pass
Hardware writes (fans/TDP/SMU/HID were never exercised), `z13gui-plus` and
`z13-tablet-kit` callers, goreleaser/AUR packaging, `PROTOCOL.md` accuracy,
mkdocs `site/` output.

## Lean summary
-85 lines possible, -0 dependencies: TDP validation -30 (#11), wire structs -45
(#7, if exported from `api`), `readTrimmed` -5 (#13).

---

## Post-verification corrections (opus 5.5 pass — supersedes fix sketches above)

Provenance: opus 5.5 verified all 20 findings read-only in a fresh session against
`AUDIT-claims.md` (bias-stripped). Results: 15 confirmed, 3 partly right (#4, #10,
#12), 1 refuted (#19), 1 partly checkable (#18). Its two contested claims (#19
refutation, #23) and its new findings #21/#23 were re-adjudicated locally
(env-dedup test in /tmp, `sh` exit-status test, grep of the clone block) — all
held. #22/#24/#25 verified by code reading.

### Corrected verdicts
- **#19 REFUTED — do not fix.** Go's `os/exec` dedups `cmd.Env` keeping the LAST
  duplicate (verified: child saw `DISPLAY=:fresh` with stale-first order). The
  append order in `refresh.go:166-171` is correct; the proposed prepend would
  introduce the bug it described.
- **#4 wording:** only commands that wait on `applyMu`/`d.mu` hang; `--get`
  commands read sysfs directly and are unaffected.
- **#10 wording:** "every human user" overstated (the `users` group is not
  auto-populated); understated: raw sysfs bypasses the CLI's 93 W / -40 caps and
  the SMU mailbox accepts any command.
- **#12:** `dbus.SessionBus()` is a shared cached connection; only the
  Properties.Get is per request.
- **#18 sub-claim unchecked:** whether `SetProfile` leaves fan modes alone needs
  kernel source; the daemon comment (`daemon.go:91-92`) suggests it affects fans.

### Corrected fixes (the sketches earlier in this file are WRONG for these)
- **#1:** do NOT `Devices = nil` — the lightbar would lose its own mode/colour.
  Instead set `Brightness`/`Enabled` on each existing `Devices` entry.
- **#2:** do NOT snapshot-and-pass (hotplug can close the snapshot before the
  write). Hold `d.mu` around device use in `handlePrepareForSleep`,
  `restoreVolatileState`, `restoreLighting`. Lock order `applyMu` → `d.mu` is
  already global; `reopenAndRestore` takes only `d.mu`, so no deadlock. This also
  covers the state reads inside `applyLightingState`.
- **#3:** a `dispatch` test would write REAL sysfs (`ppt_*` is root:users rw
  here) — test validation errors only, or inject paths first. CI must also run
  `cd api && go test ./...` (separate module) and install golangci-lint.
- **#4:** never put a blanket deadline on `Subscribe` (kills every subscription
  after 10 s). Deadline for the handshake only, then clear it with
  `SetDeadline(time.Time{})`.
- **#5:** use the "skip the override when every user PWM ≥ 204" option; an
  index-wise max pairs points at different temperatures and can be weaker at the
  high end.
- **#6:** `dryrun_test.go:214-218` and `:242` assert the current wrong text —
  update them with the fix.
- **#7:** `go.mod` already has `replace => ./api`, so exporting the structs needs
  no tag for this repo. Stronger guard: a daemon test that calls each `api.Send*`
  against a fake socket and decodes with `DisallowUnknownFields`.
- **#8:** also `user.LookupGroup` before writing anything.
- **#18:** minimal fix: if PL1 ≤ 75 and the current curve equals
  `HighTDPFanCurve()` in custom mode, call `ResetAllFanCurves()`.
- **#20:** roll back by reapplying previous known state (old TDP if active, else
  `SetProfile(current)`); do NOT read sysfs back (stale 5 W cache, `tdp.go:22-25`).

### Additional findings (opus 5.5)

#### 21. `get-state` can crash the daemon: concurrent map read/write on `Devices` [MUST FIX — crash]
- `server.go:174-181` clones every shared field except `Devices`; the response is
  JSON-encoded after `d.mu` is released while per-device apply/off/brightness
  write the same map (`:448`, `:480`, `:543`). Go aborts the process with
  "concurrent map iteration and map write". Case: GUI polls get-state while the
  user changes lightbar brightness. (Verified locally: the clone block has no
  `Devices` clone.)
- Fix: `s.Devices = maps.Clone(d.state.Devices)` in the clone block.

#### 22. per-device off then brightness saves unrestorable state + 2 s retry loop forever [MUST FIX]
- `server.go:537-543`: `off --device lightbar` saves `{Enabled:false}`; later
  `brightness 2 --device lightbar` reuses that entry → `{Enabled:true, Mode:""}`;
  `applyZone` then fails on `ModeFromString("")`. Restore fails on restart,
  resume and lid-open, and `reopenAndRestore` returns false so `hotplugTick`
  (`hotplug.go:142-147`) reopens the HID device and resends every 2 s forever.
- Fix: when the base per-device entry has `Mode == ""`, build from
  `d.state.Lighting`.

#### 23. Boot permissions service stops at the first missing file [MUST FIX — breaks TDP/undervolt after reboot on kernels < 6.19]
- `setup.go:75-78` and `contrib/systemd/system/z13ctl-plus-perms.service:13-16`:
  `for …; do [ -e "$f" ] && …; done` exits 1 when the last file is missing
  (verified: `sh` exit=1). A failed `ExecStart=` stops the oneshot, so the
  remaining lines never run. Case: a kernel without asus-armoury has no
  `panel_overdrive`, so the firmware-attributes line fails and PPT, cpufreq and
  ryzen_smu permissions are never applied at boot → TDP/undervolt fail for
  non-root users after reboot.
- Fix: append `|| true` to each `ExecStart` line (or restructure the loop to
  exit 0 on missing files) in BOTH `buildServiceContent` and the contrib unit.

#### 24. Refresh-rate changes are silently dropped without z13-tablet-kit [SHOULD FIX]
- `display.go:121` returns early when `posture == ""`; only the kit sets posture.
  `refresh-rate 60` returns `ok:true` and never calls kscreen-doctor; preset
  refresh rates are lost the same way although the kit is documented as optional.
- Fix: when posture is unset, apply refresh intents directly (skip only
  scale/rotation).

#### 25. State saves can land out of order (lost update) [SHOULD FIX — narrow]
- Handlers copy state under `d.mu`, unlock, then `saveState(s)` (e.g.
  `server.go:589`). `tablet-settings-set` and `power-policy-set` do not take
  `applyMu`, so one of them can save inside that gap; the older copy then
  overwrites it on disk and the change reverts at the next daemon restart.
- Fix: hold `d.mu` through `saveState` (as `deletePreset` already does) or
  serialize all saves under one mutex.
