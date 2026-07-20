// Package api provides the public client interface for the z13ctl daemon.
// It contains the shared protocol types and socket client functions used by
// CLI commands, GUI frontends, and any other tool that communicates with the
// z13ctl daemon over its Unix socket.
package api

// State holds the last-applied settings for all controllable subsystems.
// It is returned by SendGetState and broadcast as part of daemon responses.
type State struct {
	Lighting           LightingState            `json:"lighting"`
	Devices            map[string]LightingState `json:"devices,omitempty"` // per-device overrides keyed by name
	Profile            string                   `json:"profile,omitempty"`
	Battery            int                      `json:"battery_limit,omitempty"`
	BootSound          int                      `json:"boot_sound,omitempty"`
	PanelOverdrive     int                      `json:"panel_overdrive,omitempty"`
	FanCurve           *FanCurveState           `json:"fan_curve,omitempty"`
	TDP                *TDPState                `json:"tdp,omitempty"`
	Undervolt          *UndervoltState          `json:"undervolt,omitempty"`
	UndervoltAvailable bool                     `json:"undervolt_available"` // true if ryzen_smu is loaded
	CPUPower           *CPUPowerState           `json:"cpu_power,omitempty"`
	RefreshRate        int                      `json:"refresh_rate,omitempty"` // eDP-1 refresh rate in Hz (0 = unknown)
	BatteryDetail      *BatteryState            `json:"battery_detail,omitempty"`
	Temperature        int                      `json:"temperature,omitempty"` // APU temp, degrees Celsius
	FanRPM             int                      `json:"fan_rpm,omitempty"`     // fan1 speed in RPM
}

// BatteryState captures live battery telemetry from power_supply sysfs.
// Health % is computed from energy_full / energy_full_design. Power is
// instantaneous (positive = discharging, 0 = idle or charging at threshold).
type BatteryState struct {
	CapacityPct  int    `json:"capacity_pct"`  // 0–100, current charge relative to threshold
	Status       string `json:"status"`        // "Charging" / "Discharging" / "Not charging" / "Full"
	HealthPct    int    `json:"health_pct"`    // energy_full / energy_full_design × 100
	PowerWatts   string `json:"power_watts"`   // instantaneous draw in watts ("12.3" or "0")
	VoltageVolts string `json:"voltage_volts"` // terminal voltage in volts
	ThresholdPct int    `json:"threshold_pct"` // charge_control_end_threshold (40–100)
	Charging     bool   `json:"charging"`      // true when Status == "Charging"
	WhCurrent    int    `json:"wh_current"`    // energy_now in Wh (last full or current)
	WhDesign     int    `json:"wh_design"`     // energy_full_design in Wh
}

// CPUPowerState captures the live AMD P-State controls exposed by cpufreq.
type CPUPowerState struct {
	MinFrequencyKHz int      `json:"min_frequency_khz"`
	MinLimitKHz     int      `json:"min_limit_khz"`
	MaxLimitKHz     int      `json:"max_limit_khz"`
	EPP             string   `json:"epp"`
	EPPChoices      []string `json:"epp_choices"`
	Boost           bool     `json:"boost"`
}

// LightingState captures all parameters needed to reproduce one lighting zone.
type LightingState struct {
	Enabled    bool   `json:"enabled"`
	Mode       string `json:"mode"`
	Color      string `json:"color"`  // "RRGGBB" hex
	Color2     string `json:"color2"` // "RRGGBB" hex
	Speed      string `json:"speed"`
	Brightness int    `json:"brightness"` // 0–3
}

// FanCurvePoint represents one point on an 8-point fan curve.
type FanCurvePoint struct {
	Temp int `json:"temp"` // degrees Celsius
	PWM  int `json:"pwm"`  // 0–255 duty cycle
}

// FanCurveState captures the fan curve and mode applied to both fans.
type FanCurveState struct {
	Mode   int             `json:"mode"`   // pwm_enable: 0=full-speed, 1=custom, 2=auto
	Points []FanCurvePoint `json:"points"` // 8 points
}

// UndervoltState captures the AMD Curve Optimizer offset applied to the CPU.
// Values are non-positive integers (0 = stock, negative = undervolt).
// Active indicates whether the offset is currently applied to hardware.
type UndervoltState struct {
	CPUCO  int  `json:"cpu_co"` // all-core CPU Curve Optimizer offset
	Active bool `json:"active"` // true when CO is applied to hardware
}

// TDPState captures all PPT (Package Power Tracking) values in watts.
type TDPState struct {
	PL1SPL       int `json:"pl1_spl"`       // Sustained Power Limit
	PL2SPPT      int `json:"pl2_sppt"`      // Short Boost
	FPPT         int `json:"fppt"`          // Fast Boost
	APUSPPT      int `json:"apu_sppt"`      // APU Short PPT
	PlatformSPPT int `json:"platform_sppt"` // Platform Short PPT
}
