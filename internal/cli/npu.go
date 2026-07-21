package cli

// npu.go — AMD Ryzen AI NPU (XDNA) live telemetry via the amdxdna DRM accel
// driver (mainline since Linux 6.19; amdxdna-dkms 7.0+ on older kernels).
//
// Reads power (W) and average per-column utilization (%) via the
// DRM_AMDXDNA_QUERY_SENSORS ioctl, plus the MP-NPU clock via
// DRM_AMDXDNA_QUERY_CLOCK_METADATA. Both are hardware-backed sensors
// exported by the driver — no derived submission-counter deltas, no
// xrt-smi / xdna-top dependency.
//
// Device discovery: /sys/class/accel/accel*/device with vendor 0x1022 (AMD).
// The accel char device (/dev/accel/accelN) is opened per call to match the
// amdgpu sysfs reader pattern; the cost is one extra open per telemetry pull.
// Requires render-group access (same perms as the amdgpu DRM node).

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

const (
	// _IOWR('d', 0x40+7, struct amdxdna_drm_get_info) on x86_64 Linux.
	// dir=3 (RW), magic='d'=0x64, nr=0x47, size=16 -> 0xC0106447.
	amdxdnaIoctlGetInfo = 0xC0106447
	// _IOWR('d', 0x40+8, struct amdxdna_drm_set_state), same size -> 0xC0106448.
	amdxdnaIoctlSetState = 0xC0106448

	drmAmdxdnaQueryClockMetadata = 3 // enum amdxdna_drm_get_param
	drmAmdxdnaQuerySensors       = 4
	drmAmdxdnaGetPowerMode       = 9

	drmAmdxdnaSetPowerMode = 0 // enum amdxdna_drm_set_param

	amdxdnaSensorPower             = 0 // enum amdxdna_sensor_type
	amdxdnaSensorColumnUtilization = 1

	// NPU power mode values (enum amdxdna_power_mode_type).
	NPUPowerModeDefault = 0 // fallback to calculated DPM
	NPUPowerModeLow     = 1 // lowest DPM
	NPUPowerModeMedium  = 2 // medium DPM
	NPUPowerModeHigh    = 3 // highest DPM
	NPUPowerModeTurbo   = 4 // maximum power

	amdxdnaSensorSize  = 168 // unsafe.Sizeof(amdxdnaQuerySensor{})
	amdxdnaClockMDSize = 48  // unsafe.Sizeof(amdxdnaQueryClockMetadata{})
	amdxdnaPowerModeSz = 8   // unsafe.Sizeof(amdxdnaDrmPowerMode{}
)

// amdxdnaDrmGetInfo mirrors struct amdxdna_drm_get_info (16 bytes).
// Also used for SET_STATE — struct amdxdna_drm_set_state has the same layout.
// buffer is unsafe.Pointer (not uint64) so the GC keeps the backing slice
// alive across the syscall; the byte layout matches the kernel's __u64.
type amdxdnaDrmGetInfo struct {
	param      uint32
	bufferSize uint32
	buffer     unsafe.Pointer
}

// amdxdnaDrmPowerMode mirrors struct amdxdna_drm_(get|set)_power_mode (8 bytes).
type amdxdnaDrmPowerMode struct {
	mode uint8
	pad  [7]byte
}

// amdxdnaQuerySensor mirrors struct amdxdna_drm_query_sensor (168 bytes).
type amdxdnaQuerySensor struct {
	label   [64]byte
	input   uint32
	max     uint32
	average uint32
	highest uint32
	status  [64]byte
	units   [16]byte
	unitm   int8
	typ     uint8
	pad     [6]byte
}

// amdxdnaQueryClock mirrors struct amdxdna_drm_query_clock (24 bytes).
type amdxdnaQueryClock struct {
	name    [16]byte
	freqMHz uint32
	pad     uint32
}

// amdxdnaQueryClockMetadata mirrors struct amdxdna_drm_query_clock_metadata
// (48 bytes): two amdxdna_drm_query_clock structs.
type amdxdnaQueryClockMetadata struct {
	mpNpuClock amdxdnaQueryClock
	hClock     amdxdnaQueryClock
}

// npuDeviceOnce + npuDevicePath cache the discovered accel device path.
var (
	npuDeviceOnce sync.Once
	npuDevicePath string
)

// findNPUDevice returns the /dev/accel/accelN path for the AMD XDNA NPU,
// or "" if no AMD accel device is bound. Cached on first call.
func findNPUDevice() string {
	npuDeviceOnce.Do(func() {
		const sysClass = "/sys/class/accel"
		entries, err := os.ReadDir(sysClass)
		if err != nil {
			return
		}
		for _, e := range entries {
			name := e.Name()
			if !strings.HasPrefix(name, "accel") {
				continue
			}
			data, err := os.ReadFile(sysClass + "/" + name + "/device/vendor")
			if err != nil {
				continue
			}
			if strings.TrimSpace(string(data)) != "0x1022" {
				continue
			}
			npuDevicePath = "/dev/accel/" + name
			return
		}
	})
	return npuDevicePath
}

// NPUAvailable returns true if an AMD XDNA NPU is bound to the amdxdna driver.
func NPUAvailable() bool {
	return findNPUDevice() != ""
}

// npuQuery issues DRM_AMDXDNA_GET_INFO with the given param. The driver
// writes the actual response size back into req.bufferSize; we return that
// trimmed slice. Starts with a generous buffer (16 sensors worth) and retries
// once if the driver signals it needs more space.
func npuQuery(param uint32, minBytes int) ([]byte, error) {
	path := findNPUDevice()
	if path == "" {
		return nil, fmt.Errorf("amdxdna NPU not found")
	}
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	size := minBytes
	if size < 16*amdxdnaSensorSize {
		size = 16 * amdxdnaSensorSize
	}
	for range 2 {
		buf := make([]byte, size)
		req := amdxdnaDrmGetInfo{
			param:      param,
			bufferSize: uint32(size),
			buffer:     unsafe.Pointer(&buf[0]),
		}
		_, _, errno := syscall.Syscall(
			syscall.SYS_IOCTL, f.Fd(),
			uintptr(amdxdnaIoctlGetInfo),
			uintptr(unsafe.Pointer(&req)),
		)
		runtime.KeepAlive(buf)
		if errno != 0 {
			// Driver returns ERANGE with the needed size written back when
			// the caller-supplied buffer is too small.
			if errno == syscall.ERANGE && int(req.bufferSize) > size {
				size = int(req.bufferSize)
				continue
			}
			return nil, fmt.Errorf("amdxdna ioctl %d: %w", param, errno)
		}
		return buf[:int(req.bufferSize)], nil
	}
	return nil, fmt.Errorf("amdxdna ioctl %d: buffer retry exhausted", param)
}

// ReadNPUSensors returns the NPU power draw (W) and the average per-column
// utilization (0–100). Returns (0, 0, err) when the NPU or sensors are
// unavailable. Multiple column-utilization sensors (Strix Halo exposes one
// per AIE column) are averaged to a single busy percentage.
//
// Power unit handling: the driver may report in mW (unitm < 0 or units
// string prefixed with "m"); we convert to W.
func ReadNPUSensors() (powerW, utilPct int, err error) {
	payload, err := npuQuery(drmAmdxdnaQuerySensors, 0)
	if err != nil {
		return 0, 0, err
	}
	var utilSum, utilCount int
	for off := 0; off+amdxdnaSensorSize <= len(payload); off += amdxdnaSensorSize {
		s := (*amdxdnaQuerySensor)(unsafe.Pointer(&payload[off]))
		switch s.typ {
		case amdxdnaSensorPower:
			v := int(s.input)
			units := strings.ToLower(strings.TrimSpace(string(s.units[:])))
			if s.unitm < 0 || strings.HasPrefix(units, "m") {
				v /= 1000
			}
			powerW = v
		case amdxdnaSensorColumnUtilization:
			utilSum += int(s.input)
			utilCount++
		}
	}
	if utilCount > 0 {
		utilPct = utilSum / utilCount
	}
	return powerW, utilPct, nil
}

// ReadNPUClockMHz returns the current MP-NPU clock (npuclk) in MHz, or
// (0, err) if the NPU or clock query is unavailable. This is the live npuclk
// from the current DPM level (driver updates ndev->npuclk_freq on every DPM
// switch — see aie2_smu.c). In practice on Strix Halo the driver pins DPM
// at max (1267 MHz) whenever any process holds /dev/accel/accelN open, so
// the value may appear static — but it is not metadata.
func ReadNPUClockMHz() (int, error) {
	payload, err := npuQuery(drmAmdxdnaQueryClockMetadata, amdxdnaClockMDSize)
	if err != nil {
		return 0, err
	}
	if len(payload) < amdxdnaClockMDSize {
		return 0, fmt.Errorf("amdxdna clock metadata: short response (%d)", len(payload))
	}
	md := (*amdxdnaQueryClockMetadata)(unsafe.Pointer(&payload[0]))
	return int(md.mpNpuClock.freqMHz), nil
}

// ReadNPUPowerMode returns the current NPU DPM power mode
// (0=DEFAULT, 1=LOW, 2=MEDIUM, 3=HIGH, 4=TURBO), or (0, err) if unavailable.
func ReadNPUPowerMode() (int, error) {
	payload, err := npuQuery(drmAmdxdnaGetPowerMode, amdxdnaPowerModeSz)
	if err != nil {
		return 0, err
	}
	if len(payload) < amdxdnaPowerModeSz {
		return 0, fmt.Errorf("amdxdna power mode: short response (%d)", len(payload))
	}
	pm := (*amdxdnaDrmPowerMode)(unsafe.Pointer(&payload[0]))
	return int(pm.mode), nil
}

// SetNPUPowerMode sets the NPU DPM power mode (0–4). The mode is volatile —
// it resets to DEFAULT on sleep/shutdown; the daemon does not persist it.
//
// NOTE: the amdxdna driver marks SET_STATE as DRM_ROOT_ONLY, so this returns
// EACCES unless the caller has CAP_SYS_ADMIN (root or setcap'd binary).
func SetNPUPowerMode(mode int) error {
	if mode < NPUPowerModeDefault || mode > NPUPowerModeTurbo {
		return fmt.Errorf("npu power mode %d out of range [0, 4]", mode)
	}
	path := findNPUDevice()
	if path == "" {
		return fmt.Errorf("amdxdna NPU not found")
	}
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer f.Close()

	payload := amdxdnaDrmPowerMode{mode: uint8(mode)}
	req := amdxdnaDrmGetInfo{
		param:      drmAmdxdnaSetPowerMode,
		bufferSize: amdxdnaPowerModeSz,
		buffer:     unsafe.Pointer(&payload),
	}
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL, f.Fd(),
		uintptr(amdxdnaIoctlSetState),
		uintptr(unsafe.Pointer(&req)),
	)
	runtime.KeepAlive(payload)
	if errno != 0 {
		return fmt.Errorf("amdxdna set power mode: %w", errno)
	}
	return nil
}

// NPUPowerModeName returns a lowercase human-readable name for a power mode.
func NPUPowerModeName(mode int) string {
	switch mode {
	case NPUPowerModeDefault:
		return "default"
	case NPUPowerModeLow:
		return "low"
	case NPUPowerModeMedium:
		return "medium"
	case NPUPowerModeHigh:
		return "high"
	case NPUPowerModeTurbo:
		return "turbo"
	default:
		return fmt.Sprintf("unknown(%d)", mode)
	}
}

// NPUPowerModeFromString parses a power mode by name or numeric string.
// Returns (mode, true) on success, (0, false) if the input is unrecognized.
// Matching is case-insensitive; names match NPUPowerModeName output.
func NPUPowerModeFromString(s string) (int, bool) {
	if n, err := strconv.Atoi(s); err == nil {
		if n >= NPUPowerModeDefault && n <= NPUPowerModeTurbo {
			return n, true
		}
		return 0, false
	}
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "default", "def":
		return NPUPowerModeDefault, true
	case "low":
		return NPUPowerModeLow, true
	case "medium", "med":
		return NPUPowerModeMedium, true
	case "high":
		return NPUPowerModeHigh, true
	case "turbo":
		return NPUPowerModeTurbo, true
	default:
		return 0, false
	}
}
