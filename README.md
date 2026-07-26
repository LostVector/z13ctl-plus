# z13ctl-plus

RGB lighting, performance profiles, battery limit, fan curves, TDP control,
CPU undervolting, and display settings for the 2025 ASUS ROG Flow Z13 on
Linux.

[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

`z13ctl-plus` is a compatibility-preserving distribution of
[`dahui/z13ctl`](https://github.com/dahui/z13ctl). It deliberately retains the
`z13ctl` command, systemd units, socket and state paths, and canonical
`github.com/dahui/z13ctl/api` Go API so existing integrations keep working.

`z13ctl-plus` implements the Aura HID protocol directly against the Linux `hidraw`
interface — no kernel patches, no external daemons. System settings (profiles,
battery limit, boot sound, panel overdrive, fan curves, TDP) use the standard
asus-wmi and asus-armoury sysfs interfaces. CPU undervolting uses the
`ryzen_smu` kernel module for AMD Curve Optimizer control. A background daemon
persists state across reboots, restores volatile settings after sleep/resume,
re-applies keyboard lighting when the detachable keyboard is reattached, and
watches the Armoury Crate button.

## What the plus fork adds

Compared with the upstream project, z13ctl-plus adds:

### Features

- CPU minimum-frequency, energy-performance-preference, and boost controls
- Named performance presets with automatic AC/battery switching
- Coordinated display posture, rotation, scale, and refresh-rate handling
- Independent fan-curve, TDP, and undervolt overrides
- Live CPU, GPU, VRAM, NPU, fan, thermal, and detailed battery telemetry

### Bug fixes

- Restores lighting after keyboard reattach, sleep/resume, and lid-open unlock
- Finds the graphical session reliably for KScreen on desktop and Steam Gaming Mode
- Applies sysfs permissions reliably across late-created battery, firmware,
  power, fan, CPU, and `ryzen_smu` controls

### Code cleanup

- Removed dead fan-preset, HID feature-report, and unused hardware-helper paths
- Consolidated repeated API, sysfs, daemon response, clone, and test logic
- Made the protocol reference single-source and removed generated API-doc tooling
- Removed more than 1,100 net lines while retaining compatibility and zero lint issues

> [!TIP]
> **New to Linux? Install [z13gui](https://github.com/dahui/z13gui).** Most
> users (especially newcomers!) should use **z13gui**, a touch-friendly
> graphical overlay that exposes every z13ctl-plus feature (lighting, fan curves,
> TDP, undervolt, profiles, battery limit) with no command line required. The
> raw `z13ctl` CLI shines for scripting and advanced tuning, but if you're not a
> Linux veteran, install z13gui alongside z13ctl-plus for a far smoother experience.
> The Armoury Crate button opens the overlay.

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
# Solid cyan at full brightness
z13ctl apply --color cyan --brightness high

# Breathing red
z13ctl apply --mode breathe --color red --speed slow

# Rainbow wave
z13ctl apply --mode rainbow --speed normal

# Turn off lighting
z13ctl off

# Set performance profile
z13ctl profile --set balanced

# Cap battery charge at 80%
z13ctl batterylimit --set 80

# Custom fan curve (8-point, temp:pwm pairs — both fans)
z13ctl fancurve --set "48:2,53:22,57:30,60:43,63:56,65:68,70:89,76:102"

# Set TDP to 50W
z13ctl tdp --set 50

# Undervolt CPU by -20 (Curve Optimizer, requires ryzen_smu)
z13ctl undervolt --set -20
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
