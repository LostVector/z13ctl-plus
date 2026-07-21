package daemon

import (
	"testing"

	"github.com/dahui/z13ctl/api"
)

func TestValidatePreset(t *testing.T) {
	panel := 1
	valid := api.Preset{Profile: "quiet", RefreshRate: 60, PanelOverdrive: &panel}
	if err := validatePreset(valid); err != nil {
		t.Fatalf("validatePreset(valid) = %v", err)
	}

	invalid := valid
	invalid.RefreshRate = 120
	if err := validatePreset(invalid); err == nil {
		t.Fatal("validatePreset(120 Hz) error = nil")
	}

	if err := validatePreset(api.Preset{Profile: "custom"}); err == nil {
		t.Fatal("validatePreset(empty custom) error = nil")
	}
}

func TestClonePresetIsIndependent(t *testing.T) {
	original := api.Preset{
		Profile:   "custom",
		FanCurve:  &api.FanCurveState{Mode: 1, Points: []api.FanCurvePoint{{Temp: 30, PWM: 50}}},
		Undervolt: &api.UndervoltState{CPUCO: -20},
	}
	cloned := clonePreset(original)
	cloned.FanCurve.Points[0].PWM = 100
	cloned.Undervolt.Active = true
	if original.FanCurve.Points[0].PWM != 50 || original.Undervolt.Active {
		t.Fatal("clonePreset shares mutable preset state")
	}
}

func TestValidatePresetName(t *testing.T) {
	if got, err := validatePresetName("  Gaming  "); err != nil || got != "Gaming" {
		t.Fatalf("validatePresetName() = %q, %v", got, err)
	}
	if _, err := validatePresetName("bad\nname"); err == nil {
		t.Fatal("validatePresetName(newline) error = nil")
	}
}
