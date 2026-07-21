package cli

// telemetry.go — live sensor readers for the AMD Strix Halo SoC.
//
// CPU side: k10temp (temp), cpufreq (clock), /proc/stat (utilisation as a
// jiffie delta between successive calls).
//
// GPU side: amdgpu driver sysfs. The drm card path is discovered once and
// cached; amdgpu exposes gpu_busy_percent, freq1_input (sclk),
// mem_info_vram_{used,total}, and mem_info_freq (mclk) on the device node.
// GPU edge temp comes from the amdgpu hwmon entry, which is looked up by
// hwmon name "amdgpu" via the existing FindFanHwmonPath helper.
//
// All readers return (0, nil) silently when the sensor file is absent or
// unreadable, so callers can populate a TelemetryState struct without
// per-field presence checks. A missing driver just yields zeros.

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/dahui/z13ctl/api"
)

// gpuDevicePath caches the discovered amdgpu drm card device directory.
// Discovered lazily on first telemetry read.
var (
	gpuDevicePathOnce sync.Once
	gpuDevicePath     string
)

// findGPUDevicePath returns the /sys/class/drm/card*/device path whose driver
// is amdgpu. Returns "" if no amdgpu card is found.
func findGPUDevicePath() string {
	gpuDevicePathOnce.Do(func() {
		const dir = "/sys/class/drm"
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range entries {
			name := e.Name()
			if !strings.HasPrefix(name, "card") || strings.Contains(name, "-") {
				continue // skip cardN-M (output ports, not the primary card)
			}
			devPath := dir + "/" + name + "/device"
			link, err := filepath.EvalSymlinks(devPath + "/driver")
			if err != nil {
				continue
			}
			if filepath.Base(link) == "amdgpu" {
				gpuDevicePath, _ = filepath.EvalSymlinks(devPath)
				return
			}
		}
	})
	return gpuDevicePath
}

// ReadGPUTemperature returns the iGPU edge temperature in degrees Celsius.
// Reads temp1_input from the amdgpu hwmon (millidegrees → degrees).
func ReadGPUTemperature() (int, error) {
	dir := FindFanHwmonPath("amdgpu")
	if dir == "" {
		return 0, fmt.Errorf("amdgpu hwmon not found")
	}
	milli, err := readIntFile(dir + "/temp1_input")
	if err != nil {
		return 0, err
	}
	return milli / 1000, nil
}

// ReadGPUUtilization returns the iGPU busy percentage (0–100).
// Uses amdgpu's gpu_busy_percent sysfs attribute.
func ReadGPUUtilization() (int, error) {
	dev := findGPUDevicePath()
	if dev == "" {
		return 0, fmt.Errorf("amdgpu device not found")
	}
	return readIntFile(dev + "/gpu_busy_percent")
}

// ReadGPUClockMHz returns the current iGPU system clock (sclk) in MHz.
// Tries amdgpu's freq1_input first (discrete GPUs); falls back to parsing
// pp_dpm_sclk on APUs (Strix Halo and similar) where freq1_input is absent.
// The pp_dpm_sclk file lists P-states with the current one marked by '*'.
func ReadGPUClockMHz() (int, error) {
	dev := findGPUDevicePath()
	if dev == "" {
		return 0, fmt.Errorf("amdgpu device not found")
	}
	if v, err := readIntFile(dev + "/freq1_input"); err == nil {
		return v, nil
	}
	return readCurrentDpmClock(dev + "/pp_dpm_sclk")
}

// ReadVRAM returns the iGPU VRAM usage in MB. On Strix Halo's unified memory
// architecture, "VRAM" is the carved-out portion of system RAM reserved for
// the iGPU. total is the carveout size; used is how much of it is in use.
func ReadVRAM() (used, total int, err error) {
	dev := findGPUDevicePath()
	if dev == "" {
		return 0, 0, fmt.Errorf("amdgpu device not found")
	}
	used, err = readIntFile(dev + "/mem_info_vram_used")
	if err != nil {
		return 0, 0, err
	}
	total, err = readIntFile(dev + "/mem_info_vram_total")
	if err != nil {
		return 0, 0, err
	}
	// amdgpu reports VRAM in bytes; convert to MiB.
	return used / (1024 * 1024), total / (1024 * 1024), nil
}

// ReadMemory returns actionable Linux memory usage in MiB. MemAvailable
// includes reclaimable cache, so total - available reflects memory pressure
// better than total - free.
func ReadMemory() (used, total int, err error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	var totalKB, availableKB int
	var foundTotal, foundAvailable bool
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		value, parseErr := strconv.Atoi(fields[1])
		if parseErr != nil {
			continue
		}
		switch fields[0] {
		case "MemTotal:":
			totalKB = value
			foundTotal = true
		case "MemAvailable:":
			availableKB = value
			foundAvailable = true
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, 0, err
	}
	if !foundTotal || !foundAvailable || totalKB <= 0 || availableKB < 0 || availableKB > totalKB {
		return 0, 0, fmt.Errorf("invalid memory totals")
	}
	return (totalKB - availableKB) / 1024, totalKB / 1024, nil
}

// ReadMemClockMTs returns the iGPU memory clock in MT/s (mega-transfers/sec).
// Tries amdgpu's mem_info_freq first (discrete GPUs, already in MT/s); falls
// back to parsing pp_dpm_mclk on APUs where mem_info_freq is absent. The
// pp_dpm_mclk value is in MHz; we convert to MT/s by doubling (double data
// rate). Returns 0 if neither source is readable.
func ReadMemClockMTs() (int, error) {
	dev := findGPUDevicePath()
	if dev == "" {
		return 0, fmt.Errorf("amdgpu device not found")
	}
	if v, err := readIntFile(dev + "/mem_info_freq"); err == nil {
		return v, nil
	}
	mhz, err := readCurrentDpmClock(dev + "/pp_dpm_mclk")
	if err != nil {
		return 0, err
	}
	return mhz * 2, nil // MHz → MT/s (double data rate)
}

// readCurrentDpmClock parses an amdgpu pp_dpm_* file and returns the current
// clock in MHz. These files list one P-state per line, e.g.:
//
//	0: 600Mhz
//	1: 638Mhz *
//	2: 2900Mhz
//
// The line ending with '*' is the active P-state. Returns an error if the
// file is missing or no active P-state is found.
func readCurrentDpmClock(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.Contains(line, "*") {
			continue
		}
		// "1: 638Mhz *" → extract "638" between space and "Mhz".
		mhz := parseIntBetween(line, " ", "Mhz")
		if mhz > 0 {
			return mhz, nil
		}
	}
	return 0, fmt.Errorf("no active P-state in %s", path)
}

// parseIntBetween extracts the first integer between the given prefix and
// suffix substrings. Returns 0 if not found.
func parseIntBetween(s, prefix, suffix string) int {
	i := strings.Index(s, prefix)
	if i < 0 {
		return 0
	}
	rest := s[i+len(prefix):]
	j := strings.Index(rest, suffix)
	if j < 0 {
		return 0
	}
	v, _ := strconv.Atoi(strings.TrimSpace(rest[:j]))
	return v
}

// cpuUtilState tracks /proc/stat jiffies between successive calls to compute
// CPU utilisation as a delta. Guarded by cpuUtilMu.
var (
	cpuUtilMu       sync.Mutex
	cpuUtilPrev     uint64 // sum of all jiffies (user+nice+system+...+idle+iowait)
	cpuUtilPrevBusy uint64 // sum of non-idle jiffies
	cpuUtilHasPrev  bool
)

// ReadCPUUtilization returns aggregate CPU utilisation (0–100) across all
// cores, computed as the busy-jiffie delta over the total-jiffie delta since
// the previous call. The first call after daemon start returns 0.
func ReadCPUUtilization() (int, error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		return 0, fmt.Errorf("empty /proc/stat")
	}
	// Aggregate line: "cpu  user nice system idle iowait irq softirq steal guest guest_nice"
	fields := strings.Fields(scanner.Text())
	if len(fields) < 5 || fields[0] != "cpu" {
		return 0, fmt.Errorf("malformed /proc/stat aggregate line")
	}
	var jiffies []uint64
	for _, s := range fields[1:] {
		v, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parsing jiffie %q: %w", s, err)
		}
		jiffies = append(jiffies, v)
	}
	// idle = idle + iowait; busy = everything else
	idle := jiffies[3]
	if len(jiffies) >= 5 {
		idle += jiffies[4]
	}
	var total uint64
	for _, v := range jiffies {
		total += v
	}
	busy := total - idle

	cpuUtilMu.Lock()
	defer cpuUtilMu.Unlock()
	if !cpuUtilHasPrev {
		cpuUtilPrev = total
		cpuUtilPrevBusy = busy
		cpuUtilHasPrev = true
		return 0, nil
	}
	dt := total - cpuUtilPrev
	db := busy - cpuUtilPrevBusy
	cpuUtilPrev = total
	cpuUtilPrevBusy = busy
	if dt == 0 {
		return 0, nil
	}
	pct := int((db * 100) / dt)
	if pct < 0 {
		pct = 0
	} else if pct > 100 {
		pct = 100
	}
	return pct, nil
}

// ReadCPUClockMHz returns the average scaling_cur_freq across all CPU cores,
// in MHz. cpufreq reports kHz; we convert. Cores with missing cpufreq nodes
// are skipped; if none are readable, returns an error.
func ReadCPUClockMHz() (int, error) {
	const dir = "/sys/devices/system/cpu"
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	var sum, count uint64
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "cpu") {
			continue
		}
		// Filter out cpuN aliases like cpuidle, cpufreq, etc.
		if _, err := strconv.Atoi(strings.TrimPrefix(name, "cpu")); err != nil {
			continue
		}
		path := dir + "/" + name + "/cpufreq/scaling_cur_freq"
		khz, err := readIntFile(path)
		if err != nil {
			continue
		}
		sum += uint64(khz) / 1000
		count++
	}
	if count == 0 {
		return 0, fmt.Errorf("no readable cpu cpufreq nodes")
	}
	return int(sum / count), nil
}

// ReadTelemetry returns a fully-populated TelemetryState. Missing sensors
// silently contribute zero values, so callers can use the struct directly
// without nil-checking individual fields.
func ReadTelemetry() *api.TelemetryState {
	t := &api.TelemetryState{}
	if v, err := ReadAPUTemperature(); err == nil {
		t.CPUTemp = v
	}
	if v, err := ReadGPUTemperature(); err == nil {
		t.GPUTemp = v
	}
	if rpms, err := ReadBothFanRPM(); err == nil {
		t.CPUFanRPM = rpms[0]
		t.GPUFanRPM = rpms[1]
	}
	if v, err := ReadCPUUtilization(); err == nil {
		t.CPUUtil = v
	}
	if v, err := ReadGPUUtilization(); err == nil {
		t.GPUUtil = v
	}
	if power, util, err := ReadNPUSensors(); err == nil {
		t.NPUPowerW = power
		t.NPUUtil = util
	}
	if v, err := ReadNPUClockMHz(); err == nil {
		t.NPUClockMHz = v
	}
	if v, err := ReadCPUClockMHz(); err == nil {
		t.CPUClockMHz = v
	}
	if v, err := ReadGPUClockMHz(); err == nil {
		t.GPUClockMHz = v
	}
	if used, total, err := ReadVRAM(); err == nil {
		t.VRAMUsedMB = used
		t.VRAMTotalMB = total
	}
	if used, total, err := ReadMemory(); err == nil {
		t.MemoryUsedMB = used
		t.MemoryTotalMB = total
	}
	if v, err := ReadMemClockMTs(); err == nil {
		t.MemClockMTs = v
	}
	return t
}
