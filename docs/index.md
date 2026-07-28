# z13ctl-plus

Power automation, composable tuning overrides, AMD P-State policy, coordinated
display state, and expanded telemetry for the **2025 ASUS ROG Flow Z13** on Linux.

[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](https://github.com/aic0d3r/z13ctl-plus/blob/main/LICENSE)

`z13ctl-plus` is a compatibility-preserving distribution of
[`dahui/z13ctl`](https://github.com/dahui/z13ctl). It installs the `z13ctl`
command and retains the canonical `github.com/dahui/z13ctl/api` Go API.

---

## What Plus Adds

- **Whole-system power automation** — capture named presets and assign separate
  configurations to plugged-in and battery operation
- **Composable tuning** — keep the physical profile independent from fan-curve,
  TDP, and undervolt overrides; reset or restore each without disturbing the others
- **AMD P-State policy** — manage minimum CPU frequency, EPP, and boost through
  presets and the API
- **Coordinated display state** — serialize KDE posture, rotation, scale, and
  refresh changes for the internal panel without touching external displays
- **Expanded telemetry** — expose CPU, GPU, NPU, memory, VRAM, dual-fan, thermal,
  power, clock, and detailed battery data
- **Better hardware recovery** — restore lighting after lid-open events that do
  not suspend and preserve firmware zero-RPM behavior in the Quiet profile

!!! tip "New to Linux? Use the GUI"
    **[Z13GUI+](https://github.com/aic0d3r/z13gui-plus)** provides touch-friendly
    controls for power automation, CPU policy, expanded telemetry, refresh rate,
    and tuning through the daemon. Display posture/rotation and scripting remain
    CLI/API workflows.

---

## Requirements

- Linux kernel with `hidraw` support
- 2025 ASUS ROG Flow Z13 (USB IDs `0b05:18c6` and `0b05:1a30`)
- Read/write access to the relevant `hidraw` and sysfs files — either run as root,
  or use [`z13ctl setup`](commands.md#setup) to install udev rules granting access
  to a group

---

## Next steps

Head to the [Installation](installation.md) guide to get z13ctl-plus on your system,
then follow the [Quick Start](getting-started.md) to configure power automation
and display behavior. For the full CLI reference, see [Commands](commands.md).
