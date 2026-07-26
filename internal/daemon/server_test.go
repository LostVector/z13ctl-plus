package daemon

import "testing"

func TestProfileProtocolRejectsCustom(t *testing.T) {
	d := &Daemon{}
	resp := d.dispatch(request{Cmd: "profile", Set: "custom"})
	if resp.OK || resp.Error == "" {
		t.Fatalf("custom profile response = %+v", resp)
	}
}
