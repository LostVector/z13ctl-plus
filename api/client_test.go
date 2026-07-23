package api

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestSendRestoreRecommendedProtocol(t *testing.T) {
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
		_ = json.NewDecoder(bufio.NewReader(conn)).Decode(&got)
		requestCh <- got
		_, _ = conn.Write([]byte("{\"ok\":true}\n"))
	}()

	handled, err := SendRestoreRecommended()
	if err != nil || !handled {
		t.Fatalf("SendRestoreRecommended() = %t, %v", handled, err)
	}
	if got := <-requestCh; got.Cmd != "restore-recommended" {
		t.Fatalf("command = %q", got.Cmd)
	}
}

func TestSendTuningResetProtocol(t *testing.T) {
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
		_ = json.NewDecoder(bufio.NewReader(conn)).Decode(&got)
		requestCh <- got
		_, _ = conn.Write([]byte("{\"ok\":true}\n"))
	}()

	handled, err := SendTuningReset()
	if err != nil || !handled {
		t.Fatalf("SendTuningReset() = %t, %v", handled, err)
	}
	if got := <-requestCh; got.Cmd != "tuning-reset" {
		t.Fatalf("command = %q", got.Cmd)
	}
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
