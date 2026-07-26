package daemon

// state_test.go — Tests for state persistence: saveState, loadState, defaultState.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dahui/z13ctl/api"
)

func TestSaveAndLoadState_RoundTrip(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	s := api.State{
		Lighting: api.LightingState{
			Enabled:    true,
			Mode:       "cycle",
			Color:      "FF0000",
			Color2:     "000000",
			Speed:      "normal",
			Brightness: 3,
		},
		Profile: "performance",
		Battery: 80,
	}
	if err := saveState(s); err != nil {
		t.Fatalf("saveState: %v", err)
	}
	got, fresh, err := loadState()
	if err != nil {
		t.Fatal(err)
	}
	if fresh {
		t.Fatal("saved state reported as fresh")
	}
	if got.Lighting != s.Lighting {
		t.Errorf("Lighting mismatch: got %+v, want %+v", got.Lighting, s.Lighting)
	}
	if got.Profile != s.Profile {
		t.Errorf("Profile: got %q, want %q", got.Profile, s.Profile)
	}
	if got.Battery != s.Battery {
		t.Errorf("Battery: got %d, want %d", got.Battery, s.Battery)
	}
	if got.Devices != nil {
		t.Errorf("Devices should be nil for all-device state, got %v", got.Devices)
	}
}

func TestSaveAndLoadState_WithDevices(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	s := api.State{
		Lighting: api.LightingState{
			Enabled: true, Mode: "cycle", Color: "FF0000",
			Color2: "000000", Speed: "normal", Brightness: 3,
		},
		Devices: map[string]api.LightingState{
			"keyboard": {
				Enabled: true, Mode: "static", Color: "00FFFF",
				Color2: "000000", Speed: "normal", Brightness: 2,
			},
			"lightbar": {
				Enabled: true, Mode: "cycle", Color: "FF0000",
				Color2: "000000", Speed: "fast", Brightness: 3,
			},
		},
	}
	if err := saveState(s); err != nil {
		t.Fatalf("saveState: %v", err)
	}
	got, fresh, err := loadState()
	if err != nil {
		t.Fatal(err)
	}
	if fresh {
		t.Fatal("saved state reported as fresh")
	}

	if got.Lighting != s.Lighting {
		t.Errorf("Lighting mismatch: got %+v, want %+v", got.Lighting, s.Lighting)
	}
	if len(got.Devices) != 2 {
		t.Fatalf("Devices len: got %d, want 2", len(got.Devices))
	}
	if got.Devices["keyboard"] != s.Devices["keyboard"] {
		t.Errorf("keyboard mismatch: got %+v, want %+v", got.Devices["keyboard"], s.Devices["keyboard"])
	}
	if got.Devices["lightbar"] != s.Devices["lightbar"] {
		t.Errorf("lightbar mismatch: got %+v, want %+v", got.Devices["lightbar"], s.Devices["lightbar"])
	}
}

func TestLoadState_MissingFile(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", filepath.Join(t.TempDir(), "nonexistent"))

	got, fresh, err := loadState()
	if err != nil {
		t.Fatal(err)
	}
	if !fresh {
		t.Fatal("missing state file not reported as fresh")
	}
	def := defaultState()
	if got.Lighting != def.Lighting {
		t.Errorf("missing file: got Lighting %+v, want default %+v", got.Lighting, def.Lighting)
	}
}

func TestLoadState_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)

	path := filepath.Join(dir, "z13ctl", "state.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("not valid json"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, fresh, err := loadState(); err == nil || fresh {
		t.Fatalf("invalid state load = fresh %t, err %v", fresh, err)
	}
}

func TestLoadState_ExistingEmptyStateIsNotFresh(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	path := filepath.Join(dir, "z13ctl", "state.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, fresh, err := loadState(); err != nil || fresh {
		t.Fatal("existing empty state reported as fresh")
	}
}

func TestLoadStateRejectsNewerSchema(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	path := filepath.Join(dir, "z13ctl", "state.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	data := []byte(`{"schema_version":999,"profile":"balanced"}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadState(); err == nil {
		t.Fatal("newer state schema was accepted")
	}
}

func TestMigrateLegacyCustomState(t *testing.T) {
	s := api.State{
		Profile:   "custom",
		FanCurve:  &api.FanCurveState{Mode: 1, Points: make([]api.FanCurvePoint, 8)},
		TDP:       &api.TDPState{PL1SPL: 80},
		Undervolt: &api.UndervoltState{CPUCO: -20},
		Presets: map[string]api.Preset{
			"Custom":   {Profile: "custom", BaseProfile: "quiet", TDP: &api.TDPState{PL1SPL: 60}},
			"Physical": {Profile: "performance", TDP: &api.TDPState{PL1SPL: 60}},
		},
	}
	if !migrateState(&s, "performance") {
		t.Fatal("legacy state was not migrated")
	}
	if s.SchemaVersion != api.StateSchemaVersion || s.Profile != "performance" {
		t.Fatalf("migrated identity = schema %d profile %q", s.SchemaVersion, s.Profile)
	}
	if !s.FanCurveActive || !s.TDPActive || !s.UndervoltActive || !s.Undervolt.Active {
		t.Fatalf("active flags = fan %t TDP %t UV %t/%t", s.FanCurveActive, s.TDPActive, s.UndervoltActive, s.Undervolt.Active)
	}
	if got := s.Presets["Custom"]; got.Profile != "quiet" || got.BaseProfile != "" || got.TDP == nil {
		t.Fatalf("custom preset migration = %+v", got)
	}
	if got := s.Presets["Physical"]; got.TDP != nil {
		t.Fatalf("physical preset retained dormant tuning: %+v", got)
	}
}

func TestMigrateLegacyPhysicalStateClearsDormantTuning(t *testing.T) {
	s := api.State{
		Profile:   "quiet",
		FanCurve:  &api.FanCurveState{Mode: 1},
		TDP:       &api.TDPState{PL1SPL: 60},
		Undervolt: &api.UndervoltState{CPUCO: -10, Active: true},
	}
	migrateState(&s, "performance")
	if s.Profile != "quiet" || s.FanCurve != nil || s.TDP != nil || s.Undervolt != nil {
		t.Fatalf("migrated physical state = %+v", s)
	}
}
