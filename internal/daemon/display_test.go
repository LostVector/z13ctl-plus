package daemon

import (
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
