package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCPUPowerReadAndWrite(t *testing.T) {
	base := t.TempDir()
	for _, policy := range []string{"policy0", "policy1"} {
		dir := filepath.Join(base, policy)
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		files := map[string]string{
			"scaling_min_freq":                         "2000000\n",
			"cpuinfo_min_freq":                         "625000\n",
			"cpuinfo_max_freq":                         "3000000\n",
			"energy_performance_preference":            "balance_performance\n",
			"energy_performance_available_preferences": "default performance balance_performance balance_power power\n",
		}
		for name, value := range files {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(value), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := os.WriteFile(filepath.Join(base, "boost"), []byte("0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	state, err := readCPUPower(base)
	if err != nil {
		t.Fatal(err)
	}
	if state.MinFrequencyKHz != 2000000 || state.MinLimitKHz != 625000 || state.MaxLimitKHz != 3000000 {
		t.Fatalf("unexpected frequency state: %+v", state)
	}
	if state.EPP != "balance_performance" || state.Boost {
		t.Fatalf("unexpected power state: %+v", state)
	}

	if err := setCPUMinFrequency(base, 625000); err != nil {
		t.Fatal(err)
	}
	if err := setCPUEPP(base, "balance_power"); err != nil {
		t.Fatal(err)
	}
	for _, policy := range []string{"policy0", "policy1"} {
		minFreq, err := readTrimmed(filepath.Join(base, policy, "scaling_min_freq"))
		if err != nil || minFreq != "625000" {
			t.Fatalf("%s minimum frequency = %q, %v", policy, minFreq, err)
		}
		epp, err := readTrimmed(filepath.Join(base, policy, "energy_performance_preference"))
		if err != nil || epp != "balance_power" {
			t.Fatalf("%s EPP = %q, %v", policy, epp, err)
		}
	}
}

func TestCPUPowerRejectsInvalidValues(t *testing.T) {
	base := t.TempDir()
	policy := filepath.Join(base, "policy0")
	if err := os.Mkdir(policy, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]string{
		"scaling_min_freq":                         "2000000\n",
		"cpuinfo_min_freq":                         "625000\n",
		"cpuinfo_max_freq":                         "3000000\n",
		"energy_performance_preference":            "balance_performance\n",
		"energy_performance_available_preferences": "performance balance_performance balance_power power\n",
	} {
		if err := os.WriteFile(filepath.Join(policy, name), []byte(value), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := setCPUMinFrequency(base, 400000); err == nil {
		t.Fatal("expected out-of-range minimum frequency error")
	}
	if err := setCPUEPP(base, "turbo"); err == nil {
		t.Fatal("expected unsupported EPP error")
	}
}
