package daemon

// state.go — State persistence via $XDG_STATE_HOME.
//
// State and LightingState types are defined in the public api package
// (github.com/dahui/z13ctl/api) and used here for persistence.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/dahui/z13ctl/api"
)

var stateSaveMu sync.Mutex

func defaultState() api.State {
	return api.State{
		SchemaVersion: api.StateSchemaVersion,
		Profile:       "balanced",
		Lighting: api.LightingState{
			Enabled:    true,
			Mode:       "static",
			Color:      "FF0000",
			Color2:     "000000",
			Speed:      "normal",
			Brightness: 3,
		},
	}
}

// statePath returns the XDG-compliant path for the state file.
func statePath() string {
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(base, "z13ctl", "state.json")
}

// loadState reads persisted state. The bool reports whether no state file
// exists yet; malformed existing state must not be treated as a fresh install.
func loadState() (api.State, bool, error) {
	data, err := os.ReadFile(statePath())
	if err != nil {
		if os.IsNotExist(err) {
			return defaultState(), true, nil
		}
		return api.State{}, false, err
	}
	var s api.State
	if err := json.Unmarshal(data, &s); err != nil {
		return api.State{}, false, fmt.Errorf("parse state: %w", err)
	}
	if s.SchemaVersion > api.StateSchemaVersion {
		return api.State{}, false, fmt.Errorf("state schema %d is newer than supported schema %d", s.SchemaVersion, api.StateSchemaVersion)
	}
	changed := migrateState(&s, readProfileFromSysfs())
	if !isPhysicalProfile(s.Profile) {
		s.Profile = "balanced"
		changed = true
	}
	if changed {
		_ = saveState(s)
	}
	return s, false, nil
}

func migrateState(s *api.State, physicalProfile string) bool {
	if s.SchemaVersion >= api.StateSchemaVersion {
		return false
	}

	legacyCustom := s.Profile == "custom"
	if legacyCustom {
		if !isPhysicalProfile(physicalProfile) {
			physicalProfile = "balanced"
		}
		s.Profile = physicalProfile
		s.FanCurveActive = s.FanCurve != nil
		s.TDPActive = s.TDP != nil
		s.UndervoltActive = s.Undervolt != nil
	} else {
		if !isPhysicalProfile(s.Profile) {
			s.Profile = "balanced"
		}
		s.FanCurve = nil
		s.TDP = nil
		s.Undervolt = nil
	}
	if s.Undervolt != nil {
		s.Undervolt.Active = s.UndervoltActive
	}

	for name, preset := range s.Presets {
		switch {
		case preset.Profile == "custom":
			preset.Profile = preset.BaseProfile
			if !isPhysicalProfile(preset.Profile) {
				preset.Profile = "balanced"
			}
		case isPhysicalProfile(preset.Profile):
			preset.FanCurve = nil
			preset.TDP = nil
			preset.Undervolt = nil
		default:
			preset.Profile = "balanced"
			preset.FanCurve = nil
			preset.TDP = nil
			preset.Undervolt = nil
		}
		preset.BaseProfile = ""
		s.Presets[name] = preset
	}

	s.SchemaVersion = api.StateSchemaVersion
	return true
}

func isPhysicalProfile(profile string) bool {
	return profile == "quiet" || profile == "balanced" || profile == "performance"
}

// saveState atomically writes state to disk.
func saveState(s api.State) error {
	stateSaveMu.Lock()
	defer stateSaveMu.Unlock()
	path := statePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
