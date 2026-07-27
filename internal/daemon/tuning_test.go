package daemon

import (
	"testing"

	"github.com/dahui/z13ctl/api"
	"github.com/dahui/z13ctl/internal/cli"
)

func TestPlanTuningHighTDPSafetyIsTemporary(t *testing.T) {
	desired := &api.FanCurveState{Mode: 1, Points: []api.FanCurvePoint{{Temp: 40, PWM: 80}}}
	state := api.State{
		Profile:        "performance",
		FanCurve:       desired,
		FanCurveActive: true,
		TDP:            &api.TDPState{PL1SPL: cli.TDPMaxSafe + 1},
		TDPActive:      true,
	}

	high := planTuning(state)
	if !high.FanSafety || high.Fan == nil || high.Fan.Points[0].PWM != cli.HighTDPMinPWM {
		t.Fatalf("high-TDP plan = %+v", high)
	}
	if state.FanCurve.Points[0].PWM != 80 {
		t.Fatal("safety planning changed desired fan curve")
	}

	state.TDP.PL1SPL = cli.TDPMaxSafe
	safe := planTuning(state)
	if safe.FanSafety || safe.Fan == nil || safe.Fan.Points[0].PWM != 80 {
		t.Fatalf("safe plan did not restore desired fan: %+v", safe)
	}

	state.FanCurveActive = false
	auto := planTuning(state)
	if auto.Fan != nil {
		t.Fatalf("inactive fan override did not restore auto: %+v", auto)
	}
}

func TestPlanTuningIndependentOverrides(t *testing.T) {
	state := api.State{
		FanCurve:        &api.FanCurveState{Mode: 1},
		TDP:             &api.TDPState{PL1SPL: 50},
		TDPActive:       true,
		Undervolt:       &api.UndervoltState{CPUCO: -15},
		UndervoltActive: true,
	}
	plan := planTuning(state)
	if plan.Fan != nil || plan.TDP == nil || plan.Undervolt == nil {
		t.Fatalf("independent plan = %+v", plan)
	}
}

func TestPlanTuningQuietUsesDefaultCurve(t *testing.T) {
	plan := planTuning(api.State{Profile: "quiet"})
	if plan.Fan == nil || len(plan.Fan.Points) != 8 {
		t.Fatalf("quiet fan plan = %+v", plan.Fan)
	}
	if plan.Fan.Points[1].Temp != 55 || plan.Fan.Points[1].PWM != 0 {
		t.Errorf("quiet fan-off threshold = %+v, want 55C at PWM 0", plan.Fan.Points[1])
	}

	custom := &api.FanCurveState{Mode: 1, Points: []api.FanCurvePoint{{Temp: 40, PWM: 10}}}
	plan = planTuning(api.State{Profile: "quiet", FanCurve: custom, FanCurveActive: true})
	if plan.Fan.Points[0] != custom.Points[0] {
		t.Errorf("custom curve did not override quiet default: %+v", plan.Fan)
	}
}
