package daemon

// resume.go — sleep/resume watcher via systemd-logind DBus signals.
//
// Listens for org.freedesktop.login1.Manager.PrepareForSleep(false) on the
// system bus. When the system resumes from sleep, lighting and all volatile
// settings (undervolt, TDP, fan curves) are reapplied from daemon state.

import (
	"context"
	"log/slog"

	"github.com/godbus/dbus/v5"

	"github.com/dahui/z13ctl/internal/aura"
)

// watchResume connects to the system DBus and listens for resume events.
// When PrepareForSleep(false) is received, restoreVolatileState is called.
// Blocks until ctx is cancelled.
func (d *Daemon) watchResume(ctx context.Context) {
	conn, err := dbus.SystemBus()
	if err != nil {
		slog.Warn("cannot connect to system DBus for resume watcher", "err", err)
		return
	}

	if err := conn.AddMatchSignal(
		dbus.WithMatchInterface("org.freedesktop.login1.Manager"),
		dbus.WithMatchMember("PrepareForSleep"),
	); err != nil {
		slog.Warn("failed to add DBus match rule for PrepareForSleep", "err", err)
		return
	}

	ch := make(chan *dbus.Signal, 4)
	conn.Signal(ch)
	slog.Info("resume watcher started (listening for PrepareForSleep)")

	for {
		select {
		case <-ctx.Done():
			conn.RemoveSignal(ch)
			return
		case sig := <-ch:
			if sig == nil {
				continue
			}
			if sig.Name != "org.freedesktop.login1.Manager.PrepareForSleep" {
				continue
			}
			if len(sig.Body) < 1 {
				continue
			}
			sleeping, ok := sig.Body[0].(bool)
			if !ok {
				continue
			}
			if sleeping {
				slog.Info("system entering sleep")
				if d.dev != nil {
					if err := aura.TurnOff(d.dev); err != nil {
						slog.Warn("failed to turn off lighting before sleep", "err", err)
					}
				}
				continue
			}
			slog.Info("system resumed from sleep, restoring volatile state")
			d.restoreVolatileState()
		}
	}
}

// restoreVolatileState reapplies all settings that are lost on sleep/resume:
// lighting, fan curves, TDP, and Curve Optimizer offsets.
func (d *Daemon) restoreVolatileState() {
	d.mu.Lock()
	state := d.state
	d.mu.Unlock()
	defer d.requestDisplayReconcile()

	if state.ActivePreset == "" {
		d.applyMu.Lock()
		defer d.applyMu.Unlock()
	} else {
		d.applyMu.Lock()
	}

	// Restore lighting (lost on sleep regardless of profile).
	if d.dev != nil {
		if err := d.applyLightingState(); err != nil {
			slog.Warn("resume: failed to restore lighting", "err", err)
		} else {
			slog.Info("resume: lighting restored")
		}
	}

	if state.ActivePreset != "" {
		d.applyMu.Unlock()
		if err := d.applyPreset(state.ActivePreset); err != nil {
			slog.Warn("resume: failed to restore active preset", "preset", state.ActivePreset, "err", err)
		} else {
			slog.Info("resume: active preset restored", "preset", state.ActivePreset)
		}
		return
	}

	if err := applyTuning(state, false); err != nil {
		slog.Warn("resume: failed to restore tuning", "err", err)
	} else {
		slog.Info("resume: tuning restored", "profile", state.Profile)
	}
}
