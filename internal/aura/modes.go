package aura

// modes.go — Mode and Speed types, constants, and name parsers.

import "fmt"

// Mode corresponds to g-helper's AuraMode enum.
type Mode byte

// Mode byte values correspond to g-helper's AuraMode enum.
// Z13 verified: static/breathe/cycle/rainbow/strobe work; the additional modes
// (rainbow2/star/rain/laser/reactive) follow g-helper's enum values for the
// per-key Aura keyboard and may not all be visually supported on every panel.
const (
	ModeStatic   Mode = 0
	ModeBreathe  Mode = 1
	ModeCycle    Mode = 2
	ModeRainbow  Mode = 3
	ModeRainbow2 Mode = 4 // strobing rainbow
	ModeStar     Mode = 6
	ModeRain     Mode = 7
	ModeLaser    Mode = 9
	ModeStrobe   Mode = 10
	ModeReactive Mode = 11
)

// Speed corresponds to g-helper's AuraSpeed enum speed byte values.
type Speed byte

// Speed byte values correspond to g-helper's AuraSpeed enum.
const (
	SpeedSlow   Speed = 0xe1
	SpeedNormal Speed = 0xeb
	SpeedFast   Speed = 0xf5
)

// ModeFromString parses a user-supplied mode name.
func ModeFromString(s string) (Mode, error) {
	switch s {
	case "static":
		return ModeStatic, nil
	case "breathe":
		return ModeBreathe, nil
	case "cycle":
		return ModeCycle, nil
	case "rainbow":
		return ModeRainbow, nil
	case "rainbow2":
		return ModeRainbow2, nil
	case "star":
		return ModeStar, nil
	case "rain":
		return ModeRain, nil
	case "laser":
		return ModeLaser, nil
	case "strobe":
		return ModeStrobe, nil
	case "reactive":
		return ModeReactive, nil
	}
	return 0, fmt.Errorf("unknown mode %q (valid: static breathe cycle rainbow rainbow2 star rain laser strobe reactive)", s)
}

// SpeedFromString parses a user-supplied speed name.
func SpeedFromString(s string) (Speed, error) {
	switch s {
	case "slow":
		return SpeedSlow, nil
	case "normal":
		return SpeedNormal, nil
	case "fast":
		return SpeedFast, nil
	}
	return 0, fmt.Errorf("unknown speed %q (valid: slow normal fast)", s)
}
