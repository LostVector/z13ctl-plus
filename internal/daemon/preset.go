package daemon

// preset.go - named performance presets and automatic AC/battery selection.

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/dahui/z13ctl/api"
	"github.com/dahui/z13ctl/internal/cli"
)

const powerPolicyPollInterval = 2 * time.Second

func (d *Daemon) seedDefaultPresets() error {
	d.mu.Lock()
	configured := d.state.PowerPolicy != nil || len(d.state.Presets) != 0
	d.mu.Unlock()
	if configured {
		return nil
	}

	ac := d.capturePreset()
	ac.RefreshRate = 180

	panelOff := 0
	battery := api.Preset{
		Profile:        "quiet",
		RefreshRate:    60,
		PanelOverdrive: &panelOff,
	}
	if cpu, err := cli.ReadCPUPower(); err == nil {
		epp := cpu.EPP
		for _, choice := range cpu.EPPChoices {
			if choice == "power" {
				epp = choice
				break
			}
		}
		battery.CPUPower = &api.CPUPowerPreset{
			MinFrequencyKHz: cpu.MinLimitKHz,
			EPP:             epp,
			Boost:           false,
		}
	}

	d.mu.Lock()
	d.state.Presets = map[string]api.Preset{"AC": ac, "Battery": battery}
	d.state.PowerPolicy = &api.PowerPolicy{
		Enabled:       true,
		ACPreset:      "AC",
		BatteryPreset: "Battery",
	}
	s := d.state
	d.mu.Unlock()
	return saveState(s)
}

func (d *Daemon) capturePreset() api.Preset {
	d.mu.Lock()
	p := api.Preset{
		Profile:   d.state.Profile,
		FanCurve:  cloneFanCurve(d.state.FanCurve),
		TDP:       cloneTDP(d.state.TDP),
		Undervolt: cloneUndervolt(d.state.Undervolt),
	}
	d.mu.Unlock()
	if p.Undervolt != nil {
		p.Undervolt.Active = false
	}

	if p.Profile == "" {
		p.Profile = readProfileFromSysfs()
	}
	if p.Profile == "custom" {
		p.BaseProfile = readProfileFromSysfs()
	}
	if cpu, err := cli.ReadCPUPower(); err == nil {
		p.CPUPower = &api.CPUPowerPreset{
			MinFrequencyKHz: cpu.MinFrequencyKHz,
			EPP:             cpu.EPP,
			Boost:           cpu.Boost,
		}
	}
	p.RefreshRate = d.desiredDisplayRefresh()
	if panel, err := readOptionalInt(cli.FindPanelOverdrivePath()); err == nil {
		p.PanelOverdrive = &panel
	}
	return p
}

func (d *Daemon) savePreset(name string) error {
	name, err := validatePresetName(name)
	if err != nil {
		return err
	}
	preset := d.capturePreset()
	if err := validatePreset(preset); err != nil {
		return err
	}
	d.mu.Lock()
	presets := clonePresetMap(d.state.Presets)
	if presets == nil {
		presets = make(map[string]api.Preset)
	}
	presets[name] = preset
	d.state.Presets = presets
	s := d.state
	s.Presets = clonePresetMap(d.state.Presets)
	s.PowerPolicy = clonePowerPolicy(d.state.PowerPolicy)
	d.mu.Unlock()
	return saveState(s)
}

func (d *Daemon) deletePreset(name string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, ok := d.state.Presets[name]; !ok {
		return fmt.Errorf("preset %q not found", name)
	}
	if policy := d.state.PowerPolicy; policy != nil && (policy.ACPreset == name || policy.BatteryPreset == name) {
		return fmt.Errorf("preset %q is assigned by the power policy", name)
	}
	presets := clonePresetMap(d.state.Presets)
	delete(presets, name)
	d.state.Presets = presets
	if d.state.ActivePreset == name {
		d.state.ActivePreset = ""
	}
	return saveState(d.state)
}

func (d *Daemon) applyPreset(name string) error {
	d.applyMu.Lock()
	defer d.applyMu.Unlock()

	d.mu.Lock()
	preset, ok := d.state.Presets[name]
	activePreset := d.state.ActivePreset
	d.mu.Unlock()
	if !ok {
		return fmt.Errorf("preset %q not found", name)
	}
	preset = clonePreset(preset)
	if err := validatePreset(preset); err != nil {
		return fmt.Errorf("preset %q: %w", name, err)
	}
	if err := validatePresetHardware(preset); err != nil {
		return fmt.Errorf("preset %q: %w", name, err)
	}
	if activePreset != name {
		d.clearActivePreset()
	}
	if err := d.applyPresetProfile(preset); err != nil {
		return err
	}
	if cpu := preset.CPUPower; cpu != nil {
		current, _ := cli.ReadCPUPower()
		if current.MinFrequencyKHz != cpu.MinFrequencyKHz {
			if err := cli.SetCPUMinFrequency(cpu.MinFrequencyKHz); err != nil {
				return fmt.Errorf("CPU minimum: %w", err)
			}
		}
		if current.EPP != cpu.EPP {
			if err := cli.SetCPUEPP(cpu.EPP); err != nil {
				return fmt.Errorf("CPU EPP: %w", err)
			}
		}
		if current.Boost != cpu.Boost {
			if err := cli.SetCPUBoost(cpu.Boost); err != nil {
				return fmt.Errorf("CPU boost: %w", err)
			}
		}
	}
	if preset.PanelOverdrive != nil {
		current, readErr := readOptionalInt(cli.FindPanelOverdrivePath())
		if readErr != nil || current != *preset.PanelOverdrive {
			if err := cli.SetPanelOverdrive(*preset.PanelOverdrive); err != nil {
				return fmt.Errorf("panel overdrive: %w", err)
			}
		}
	}
	if preset.RefreshRate != 0 {
		if err := d.setDisplayRefresh(preset.RefreshRate); err != nil {
			return fmt.Errorf("refresh rate: %w", err)
		}
	}

	d.mu.Lock()
	d.state.Profile = preset.Profile
	if preset.Profile == "custom" {
		d.state.FanCurve = cloneFanCurve(preset.FanCurve)
		d.state.TDP = cloneTDP(preset.TDP)
		d.state.Undervolt = cloneUndervolt(preset.Undervolt)
	} else if d.state.Undervolt != nil {
		d.state.Undervolt.Active = false
	}
	if preset.PanelOverdrive != nil {
		d.state.PanelOverdrive = *preset.PanelOverdrive
	}
	d.state.ActivePreset = name
	s := d.state
	s.Presets = clonePresetMap(d.state.Presets)
	s.PowerPolicy = clonePowerPolicy(d.state.PowerPolicy)
	d.mu.Unlock()
	if err := saveState(s); err != nil {
		return err
	}
	slog.Info("preset applied", "name", name)
	return nil
}

func (d *Daemon) applyPresetProfile(preset api.Preset) error {
	if preset.Profile == "custom" {
		// Reset the underlying firmware-managed state first so fields omitted
		// by this preset cannot leave stale PPT, fan, or CO values active.
		base := preset.BaseProfile
		if base == "" {
			base = "balanced"
		}
		if err := cli.ResetAllFanCurves(); err != nil {
			return fmt.Errorf("reset fan curve: %w", err)
		}
		if cli.SMUProbeUndervolt() {
			if err := cli.ResetCurveOptimizer(); err != nil {
				return fmt.Errorf("reset undervolt: %w", err)
			}
		}
		if err := cli.SetProfile(base); err != nil {
			return fmt.Errorf("reset firmware profile: %w", err)
		}
		if preset.TDP != nil && preset.TDP.PL1SPL > cli.TDPMaxSafe {
			curve := cli.HighTDPFanCurve()
			if preset.FanCurve != nil {
				curve = preset.FanCurve.Points
			}
			if err := cli.SetBothFanCurves(curve); err != nil {
				return fmt.Errorf("high-TDP fan curve: %w", err)
			}
		} else if preset.FanCurve != nil {
			if err := cli.SetBothFanCurves(preset.FanCurve.Points); err != nil {
				return fmt.Errorf("fan curve: %w", err)
			}
		}
		if preset.TDP != nil {
			if err := cli.SetTDP(0, preset.TDP.PL1SPL, preset.TDP.PL2SPPT, preset.TDP.FPPT); err != nil {
				return fmt.Errorf("TDP: %w", err)
			}
		}
		if preset.Undervolt != nil && cli.SMUProbeUndervolt() {
			if err := cli.SetCurveOptimizer(preset.Undervolt.CPUCO); err != nil {
				return fmt.Errorf("undervolt: %w", err)
			}
			preset.Undervolt.Active = true
		}
		return nil
	}

	d.mu.Lock()
	wasCustom := d.state.Profile == "custom"
	d.mu.Unlock()
	if wasCustom {
		if err := cli.ResetAllFanCurves(); err != nil {
			slog.Warn("failed to reset fan curves for preset", "err", err)
		}
		if cli.SMUProbeUndervolt() {
			if err := cli.ResetCurveOptimizer(); err != nil {
				slog.Warn("failed to reset undervolt for preset", "err", err)
			}
		}
	}
	// Write even when the physical profile already has this name: leaving the
	// virtual custom profile relies on this write to restore firmware PPT.
	if err := cli.SetProfile(preset.Profile); err != nil {
		return fmt.Errorf("profile: %w", err)
	}
	return nil
}

func readOptionalInt(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

func validatePresetName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 40 || strings.ContainsAny(name, "\r\n\t") {
		return "", fmt.Errorf("preset name must be 1-40 printable characters")
	}
	return name, nil
}

func validatePreset(p api.Preset) error {
	switch p.Profile {
	case "quiet", "balanced", "performance":
	case "custom":
		if p.FanCurve == nil && p.TDP == nil && p.Undervolt == nil {
			return fmt.Errorf("custom profile has no custom settings")
		}
		if p.BaseProfile != "" && p.BaseProfile != "quiet" && p.BaseProfile != "balanced" && p.BaseProfile != "performance" {
			return fmt.Errorf("invalid custom base profile %q", p.BaseProfile)
		}
	default:
		return fmt.Errorf("invalid profile %q", p.Profile)
	}
	if p.RefreshRate != 0 && p.RefreshRate != 60 && p.RefreshRate != 180 {
		return fmt.Errorf("refresh rate must be 60 or 180")
	}
	if p.PanelOverdrive != nil && *p.PanelOverdrive != 0 && *p.PanelOverdrive != 1 {
		return fmt.Errorf("panel overdrive must be 0 or 1")
	}
	if p.FanCurve != nil {
		if p.FanCurve.Mode != 1 || len(p.FanCurve.Points) != 8 {
			return fmt.Errorf("fan curve must contain 8 custom points")
		}
		for i, point := range p.FanCurve.Points {
			if point.Temp < 0 || point.Temp > 120 || point.PWM < 0 || point.PWM > 255 {
				return fmt.Errorf("invalid fan curve point %d:%d", point.Temp, point.PWM)
			}
			if i > 0 && (point.Temp <= p.FanCurve.Points[i-1].Temp || point.PWM < p.FanCurve.Points[i-1].PWM) {
				return fmt.Errorf("fan curve temperatures and PWM must be non-decreasing")
			}
		}
	}
	if p.TDP != nil {
		for _, watts := range []int{p.TDP.PL1SPL, p.TDP.PL2SPPT, p.TDP.FPPT} {
			if watts < cli.TDPMin || watts > cli.TDPMaxForced {
				return fmt.Errorf("TDP %dW out of range %d-%d", watts, cli.TDPMin, cli.TDPMaxForced)
			}
		}
		if p.TDP.PL1SPL > cli.TDPMaxSafe && p.FanCurve != nil {
			for _, point := range p.FanCurve.Points {
				if point.PWM < cli.HighTDPMinPWM {
					return fmt.Errorf("fan curve PWM must be at least %d above %dW", cli.HighTDPMinPWM, cli.TDPMaxSafe)
				}
			}
		}
	}
	if p.Undervolt != nil {
		if err := cli.ValidateCOValues(p.Undervolt.CPUCO); err != nil {
			return err
		}
	}
	return nil
}

func validatePresetHardware(p api.Preset) error {
	if p.CPUPower == nil {
		return nil
	}
	current, err := cli.ReadCPUPower()
	if err != nil {
		return fmt.Errorf("CPU power controls unavailable: %w", err)
	}
	if p.CPUPower.MinFrequencyKHz < current.MinLimitKHz || p.CPUPower.MinFrequencyKHz > current.MaxLimitKHz {
		return fmt.Errorf("CPU minimum %d kHz out of range %d-%d", p.CPUPower.MinFrequencyKHz, current.MinLimitKHz, current.MaxLimitKHz)
	}
	validEPP := false
	for _, choice := range current.EPPChoices {
		if choice == p.CPUPower.EPP {
			validEPP = true
			break
		}
	}
	if !validEPP {
		return fmt.Errorf("CPU EPP %q is unavailable", p.CPUPower.EPP)
	}
	return nil
}

func cloneFanCurve(in *api.FanCurveState) *api.FanCurveState {
	if in == nil {
		return nil
	}
	out := *in
	out.Points = append([]api.FanCurvePoint(nil), in.Points...)
	return &out
}

func cloneTDP(in *api.TDPState) *api.TDPState {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneUndervolt(in *api.UndervoltState) *api.UndervoltState {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func clonePreset(in api.Preset) api.Preset {
	in.FanCurve = cloneFanCurve(in.FanCurve)
	in.TDP = cloneTDP(in.TDP)
	in.Undervolt = cloneUndervolt(in.Undervolt)
	if in.CPUPower != nil {
		cpu := *in.CPUPower
		in.CPUPower = &cpu
	}
	if in.PanelOverdrive != nil {
		panel := *in.PanelOverdrive
		in.PanelOverdrive = &panel
	}
	return in
}

func clonePresetMap(in map[string]api.Preset) map[string]api.Preset {
	if in == nil {
		return nil
	}
	out := make(map[string]api.Preset, len(in))
	for name, preset := range in {
		out[name] = clonePreset(preset)
	}
	return out
}

func clonePowerPolicy(in *api.PowerPolicy) *api.PowerPolicy {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func (d *Daemon) setPowerPolicy(enabled bool, acPreset, batteryPreset string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	current := d.state.PowerPolicy
	if acPreset == "" {
		if current != nil {
			acPreset = current.ACPreset
		} else {
			acPreset = "AC"
		}
	}
	if batteryPreset == "" {
		if current != nil {
			batteryPreset = current.BatteryPreset
		} else {
			batteryPreset = "Battery"
		}
	}
	if _, ok := d.state.Presets[acPreset]; !ok {
		return fmt.Errorf("AC preset %q not found", acPreset)
	}
	if _, ok := d.state.Presets[batteryPreset]; !ok {
		return fmt.Errorf("battery preset %q not found", batteryPreset)
	}
	if current != nil && current.Enabled == enabled && current.ACPreset == acPreset && current.BatteryPreset == batteryPreset {
		return nil
	}
	d.state.PowerPolicy = &api.PowerPolicy{Enabled: enabled, ACPreset: acPreset, BatteryPreset: batteryPreset}
	if err := saveState(d.state); err != nil {
		return err
	}
	if enabled {
		d.requestPolicyApply()
	}
	return nil
}

func (d *Daemon) clearActivePreset() {
	d.mu.Lock()
	if d.state.ActivePreset == "" {
		d.mu.Unlock()
		return
	}
	d.state.ActivePreset = ""
	s := d.state
	d.mu.Unlock()
	if err := saveState(s); err != nil {
		slog.Warn("failed to clear active preset", "err", err)
	}
}

func (d *Daemon) requestPolicyApply() {
	select {
	case d.policyCh <- struct{}{}:
	default:
	}
}

func (d *Daemon) applyInitialPowerPolicy() bool {
	source, err := cli.ReadPowerSource()
	if err != nil {
		return false
	}
	d.mu.Lock()
	d.powerSource = source
	policy := clonePowerPolicy(d.state.PowerPolicy)
	d.mu.Unlock()
	if policy == nil || !policy.Enabled {
		return false
	}
	name := policy.BatteryPreset
	if source == cli.PowerSourceAC {
		name = policy.ACPreset
	}
	if err := d.applyPreset(name); err != nil {
		slog.Warn("failed to apply initial power preset; watcher will retry", "source", source, "preset", name, "err", err)
		d.requestPolicyApply()
	}
	return true
}

func (d *Daemon) watchPowerPolicy(ctx context.Context) {
	ticker := time.NewTicker(powerPolicyPollInterval)
	defer ticker.Stop()
	d.mu.Lock()
	lastAppliedSource := d.powerSource
	observedSource := d.powerSource
	d.mu.Unlock()
	force := false
	nextApply := time.Now().Add(3 * time.Second)

	for {
		select {
		case <-ctx.Done():
			return
		case <-d.policyCh:
			force = true
			nextApply = time.Now().Add(3 * time.Second)
		case <-ticker.C:
			source, err := cli.ReadPowerSource()
			if err != nil {
				continue
			}
			d.mu.Lock()
			d.powerSource = source
			policy := clonePowerPolicy(d.state.PowerPolicy)
			activePreset := d.state.ActivePreset
			d.mu.Unlock()
			if source != observedSource {
				observedSource = source
				assigned := ""
				if policy != nil {
					assigned = policy.BatteryPreset
					if source == cli.PowerSourceAC {
						assigned = policy.ACPreset
					}
				}
				if activePreset == assigned && assigned != "" {
					lastAppliedSource = source
					force = false
				} else {
					force = true
					nextApply = time.Now().Add(powerPolicyPollInterval)
				}
				continue
			}
			if policy == nil || !policy.Enabled || (!force && source == lastAppliedSource) {
				continue
			}
			if time.Now().Before(nextApply) {
				continue
			}
			name := policy.BatteryPreset
			if source == cli.PowerSourceAC {
				name = policy.ACPreset
			}
			if err := d.applyPreset(name); err != nil {
				slog.Warn("failed to apply power preset; will retry", "source", source, "preset", name, "err", err)
				continue
			}
			lastAppliedSource = source
			force = false
		}
	}
}
