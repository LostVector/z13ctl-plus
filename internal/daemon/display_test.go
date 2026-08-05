package daemon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSetDisplayPostureExtendsDockSettle(t *testing.T) {
	d := &Daemon{displayCh: make(chan struct{}, 1)}
	before := time.Now()
	if err := d.setDisplayPosture("desktop"); err != nil {
		t.Fatal(err)
	}
	first := d.display.notBefore
	if first.Before(before.Add(dockSettleInterval - time.Second)) {
		t.Fatalf("dock settle deadline = %v", first)
	}
	time.Sleep(time.Millisecond)
	if err := d.setDisplayPosture("desktop"); err != nil {
		t.Fatal(err)
	}
	if !d.display.notBefore.After(first) {
		t.Fatal("repeated posture did not extend dock settle interval")
	}
}

func TestDisplayIntentValidation(t *testing.T) {
	d := &Daemon{displayCh: make(chan struct{}, 1)}
	if err := d.setDisplayPosture("tent"); err == nil {
		t.Fatal("setDisplayPosture(tent) error = nil")
	}
	if err := d.setDisplayRotation("upsideways"); err == nil {
		t.Fatal("setDisplayRotation(upsideways) error = nil")
	}
	if err := d.setDisplayRefresh(120); err == nil {
		t.Fatal("setDisplayRefresh(120) error = nil")
	}
	if err := d.setDisplayRefresh(180); err != nil {
		t.Fatal(err)
	}
}

func TestReconcileDisplayTabletScalePreservesRotation(t *testing.T) {
	dir := t.TempDir()
	argsPath := filepath.Join(dir, "args")
	script := "#!/bin/sh\nif [ \"$1\" = \"-o\" ]; then\n  printf 'Output: 1 eDP-1 abc\\n\\tenabled\\n\\tModes: 1:2560x1600@180.00*\\n\\tScale: 1.5\\n\\tRotation: 2\\n'\n  exit 0\nfi\nprintf '%s\\n' \"$*\" > \"$Z13CTL_ARGS\"\n"
	if err := os.WriteFile(filepath.Join(dir, "kscreen-doctor"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("Z13CTL_ARGS", argsPath)

	d := &Daemon{display: displayIntent{posture: "tablet"}}
	changed, retry, err := d.reconcileDisplay()
	if err != nil || !changed || retry {
		t.Fatalf("reconcileDisplay() = changed %t, retry %t, err %v", changed, retry, err)
	}
	args, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.TrimSpace(string(args)), "output.eDP-1.scale.1.75"; got != want {
		t.Fatalf("kscreen-doctor args = %q, want %q", got, want)
	}
}
