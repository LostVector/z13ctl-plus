package cli

// battery.go — read-only battery telemetry from power_supply sysfs.
// All values come from /sys/class/power_supply/BAT0/. Writes (charge limit)
// live in sysfs.go (FindBatteryThresholdPath). This file only reads.

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dahui/z13ctl/api"
)

const (
	powerSupplyDir = "/sys/class/power_supply"

	// µW → W conversion divisor. power_supply energy and power files use
	// microwatts (per kernel Documentation/ABI/testing/sysfs-class-power-supply).
	microwattsPerWatt = 1_000_000
)

// ReadBatteryState returns the live battery telemetry. Returns nil and an
// error only if BAT0 cannot be found (no battery present).
func ReadBatteryState() (*api.BatteryState, error) {
	batPath, err := findBatteryDir()
	if err != nil {
		return nil, err
	}
	cap, _ := readInt(batPath + "/capacity")
	status := readString(batPath + "/status")
	threshold := readIntSafe(batPath + "/charge_control_end_threshold")
	powerMicroW := readIntSafe(batPath + "/power_now")
	voltageMicroV := readIntSafe(batPath + "/voltage_now")
	energyNow := readIntSafe(batPath + "/energy_now")
	energyFull := readIntSafe(batPath + "/energy_full")
	energyDesign := readIntSafe(batPath + "/energy_full_design")

	// Health: energy_full / energy_full_design × 100. Falls back to 100 when
	// design capacity is missing or zero (firmware did not expose it).
	health := 100
	if energyDesign > 0 {
		health = energyFull * 100 / energyDesign
		if health > 100 {
			health = 100
		}
	}

	// Current "last full charge" Wh. Use energy_now when charging, energy_full otherwise.
	whCurrent := energyFull
	if status == "Discharging" && energyNow > 0 {
		whCurrent = energyNow
	}
	whCurrentW := whCurrent / microwattsPerWatt
	whDesignW := energyDesign / microwattsPerWatt

	return &api.BatteryState{
		CapacityPct:    cap,
		Status:         status,
		HealthPct:      health,
		PowerWatts:     formatWatts(powerMicroW),
		VoltageVolts:   formatVolts(voltageMicroV),
		ThresholdPct:   threshold,
		Charging:       status == "Charging",
		WhCurrent:      whCurrentW,
		WhDesign:       whDesignW,
		EnergyNowWh:    float64(energyNow) / microwattsPerWatt,
		EnergyFullWh:   float64(energyFull) / microwattsPerWatt,
		EnergyDesignWh: float64(energyDesign) / microwattsPerWatt,
	}, nil
}

// findBatteryDir locates the BAT* directory under /sys/class/power_supply.
func findBatteryDir() (string, error) {
	matches, err := filepath.Glob(powerSupplyDir + "/BAT*")
	if err != nil {
		return "", fmt.Errorf("scanning %s: %w", powerSupplyDir, err)
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("no battery found in %s", powerSupplyDir)
	}
	return matches[0], nil
}

func readInt(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

func readIntSafe(path string) int {
	v, _ := readInt(path)
	return v
}

func readString(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// formatWatts converts microwatts to "12.3" string. Returns "0" for 0/missing.
func formatWatts(microW int) string {
	if microW <= 0 {
		return "0"
	}
	watts := float64(microW) / microwattsPerWatt
	return strconv.FormatFloat(watts, 'f', 1, 64)
}

// formatVolts converts microvolts to "17.5" string.
func formatVolts(microV int) string {
	if microV <= 0 {
		return "0"
	}
	volts := float64(microV) / 1_000_000
	return strconv.FormatFloat(volts, 'f', 1, 64)
}
