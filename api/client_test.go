package api

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
)

func assertCommandSent(t *testing.T, send func() (bool, error), want string) {
	t.Helper()
	runtimeDir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", runtimeDir)
	if err := os.MkdirAll(filepath.Dir(SocketPath()), 0o700); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("unix", SocketPath())
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	requestCh := make(chan request, 1)
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer conn.Close()
		var got request
		_ = json.NewDecoder(conn).Decode(&got)
		requestCh <- got
		_, _ = conn.Write([]byte("{\"ok\":true}\n"))
	}()

	handled, err := send()
	if err != nil || !handled {
		t.Fatalf("send() = %t, %v", handled, err)
	}
	if got := <-requestCh; got.Cmd != want {
		t.Fatalf("command = %q, want %q", got.Cmd, want)
	}
}

func TestSendRestoreRecommendedProtocol(t *testing.T) {
	assertCommandSent(t, SendRestoreRecommended, "restore-recommended")
}

func TestSendTuningResetProtocol(t *testing.T) {
	assertCommandSent(t, SendTuningReset, "tuning-reset")
}

func TestStateProtocolIncludesTuningFlags(t *testing.T) {
	data, err := json.Marshal(State{
		SchemaVersion:   StateSchemaVersion,
		Profile:         "balanced",
		FanCurveActive:  true,
		TDPActive:       true,
		UndervoltActive: true,
		FanSafetyActive: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"schema_version", "fan_curve_active", "tdp_active", "undervolt_active", "fan_safety_active"} {
		if _, ok := value[field]; !ok {
			t.Fatalf("state JSON %s missing %q", data, field)
		}
	}
}
