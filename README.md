# z13ctl-plus

Power automation, composable tuning overrides, AMD P-State policy, coordinated
display state, and expanded telemetry for the 2025 ASUS ROG Flow Z13 on Linux.

[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

`z13ctl-plus` is a compatibility-preserving distribution of
[`dahui/z13ctl`](https://github.com/dahui/z13ctl). It deliberately retains the
`z13ctl` command, systemd units, socket and state paths, and canonical
`github.com/dahui/z13ctl/api` import path, preserving compatibility for
integrations that use those interfaces.

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

> [!TIP]
> **Prefer a touch interface? Install
> [Z13GUI+](https://github.com/aic0d3r/z13gui-plus).** It exposes power automation,
> CPU policy, expanded telemetry, refresh rate, and tuning controls through the
> daemon. Display posture/rotation and scripting remain CLI/API workflows.

## Install

```sh
# Arch Linux (AUR)
yay -S z13ctl-plus-bin

# Debian / Ubuntu
sudo apt install ./z13ctl-plus_*.deb

# Fedora / RHEL
sudo dnf install ./z13ctl-plus_*.rpm

# Manual (from release tarball)
tar xzf z13ctl-plus_*_linux_amd64.tar.gz
sudo install -Dm755 z13ctl /usr/local/bin/z13ctl
sudo z13ctl setup
```

See the [Installation guide](https://aic0d3r.github.io/z13ctl-plus/installation/) for
systemd service setup, source builds, and uninstall instructions.

## Quick Start

```sh
# Create/update the recommended AC and battery presets and apply the current one
z13ctl preset restore-recommended

# Inspect and enable automatic source switching
z13ctl preset list
z13ctl preset policy --enable --ac "Plugged In" --battery "On Battery"

# Apply a named configuration manually
z13ctl preset apply "On Battery"

# Coordinate tablet posture and rotation through the daemon
z13ctl display posture tablet
z13ctl display rotation right

# Inspect expanded live telemetry and active overrides
z13ctl status
```

## Documentation

Full documentation at **<https://aic0d3r.github.io/z13ctl-plus>**

- [Installation](https://aic0d3r.github.io/z13ctl-plus/installation/)
- [Quick Start](https://aic0d3r.github.io/z13ctl-plus/getting-started/)
- [Commands](https://aic0d3r.github.io/z13ctl-plus/commands/)
- [Daemon](https://aic0d3r.github.io/z13ctl-plus/daemon/)
- [API](https://aic0d3r.github.io/z13ctl-plus/api/)
- [Contributing](https://aic0d3r.github.io/z13ctl-plus/contributing/)

## License

[Apache 2.0](LICENSE)
