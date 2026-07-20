package cli

// refresh.go — display refresh rate switching via kscreen-doctor.
//
// The Z13's eDP panel runs at 2560x1600 natively and supports 60 Hz and 180 Hz
// at that resolution. kscreen-doctor is the Plasma/Wayland tool for atomic
// mode setting; it is unprivileged and present on every Plasma 6 system.
//
// Mode specification uses resolution@rate (mode IDs are unstable across
// kscreen reconfigurations). We hardcode 2560x1600 because it is the only
// native resolution worth switching refresh rate for — lower resolutions are
// GPU-scaled and have no battery benefit.

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	nativeMode = "2560x1600"
	kscreenBin = "kscreen-doctor"
	outputName = "eDP-1"

	refreshTimeout = 3 * time.Second
)

// supportedRefreshRates is the whitelist we expose in the UI. The panel
// advertises many modes; only these two are useful (60 = battery saver,
// 180 = full speed). Order defines UI order.
var supportedRefreshRates = []int{60, 180}

// SupportedRefreshRates returns the refresh rates selectable in the UI.
func SupportedRefreshRates() []int {
	return append([]int(nil), supportedRefreshRates...)
}

// ReadRefreshRate returns the current eDP-1 refresh rate in Hz (integer).
// Returns 0 if kscreen-doctor is missing or eDP-1 is not connected.
func ReadRefreshRate() int {
	ctx, cancel := context.WithTimeout(context.Background(), refreshTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, kscreenBin, "-o").CombinedOutput()
	if err != nil {
		return 0
	}
	return parseCurrentRefresh(out)
}

// parseCurrentRefresh finds the eDP-1 line marked with "*" (current mode)
// and extracts the integer refresh rate.
func parseCurrentRefresh(out []byte) int {
	// kscreen-doctor marks the active mode with a trailing "*". Example:
	//   Modes:  1:2560x1600@180.00*  2:2560x1600@60.00  ...
	// We look for "@<digits>.<digits>*" inside a Modes: block.
	// To stay robust if multiple outputs are connected, restrict to the
	// eDP-1 output block.
	re := regexp.MustCompile(`(?m)^Output:.*\b` + outputName + `\b.*\n(?s:.*?)(?:^Output:|\z)`)
	block := re.Find(out)
	if block == nil {
		block = out // single-output fallback
	}
	rateRe := regexp.MustCompile(`@(\d+)\.\d+\*`)
	m := rateRe.FindSubmatch(block)
	if m == nil {
		return 0
	}
	n, err := strconv.Atoi(string(m[1]))
	if err != nil {
		return 0
	}
	return n
}

// SetRefreshRate switches eDP-1 to the requested rate (must be in
// supportedRefreshRates). No-op if the requested rate is already current.
func SetRefreshRate(hz int) error {
	if !isSupportedRate(hz) {
		return fmt.Errorf("unsupported refresh rate %d (valid: %v)", hz, supportedRefreshRates)
	}
	if ReadRefreshRate() == hz {
		return nil
	}
	arg := "output." + outputName + ".mode." + fmt.Sprintf("%s@%d", nativeMode, hz)
	ctx, cancel := context.WithTimeout(context.Background(), refreshTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, kscreenBin, arg)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("kscreen-doctor %s: %w%s", arg, err, suffixNonEmpty(stderr.String()))
	}
	return nil
}

func isSupportedRate(hz int) bool {
	for _, r := range supportedRefreshRates {
		if r == hz {
			return true
		}
	}
	return false
}

func suffixNonEmpty(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	return ": " + s
}
