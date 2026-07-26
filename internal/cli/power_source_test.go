package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadPowerSource(t *testing.T) {
	tests := []struct {
		name   string
		online string
		want   string
	}{
		{"ac", "1\n", PowerSourceAC},
		{"battery", "0\n", PowerSourceBattery},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			writePowerSupplyFile(t, root, "AC0/type", "Mains\n")
			writePowerSupplyFile(t, root, "AC0/online", tt.online)
			writePowerSupplyFile(t, root, "BAT0/type", "Battery\n")
			got, err := readPowerSource(root)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("readPowerSource() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestReadPowerSourceRejectsBatteryOnly(t *testing.T) {
	root := t.TempDir()
	writePowerSupplyFile(t, root, "BAT0/type", "Battery\n")
	if _, err := readPowerSource(root); err == nil {
		t.Fatal("readPowerSource() error = nil, want error")
	}
}

func writePowerSupplyFile(t *testing.T, root, name, value string) {
	t.Helper()
	path := root + "/" + name
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
		t.Fatal(err)
	}
}
