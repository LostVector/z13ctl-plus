package cli

import "testing"

func TestReadMemory(t *testing.T) {
	used, total, err := ReadMemory()
	if err != nil {
		t.Fatal(err)
	}
	if total <= 0 || used < 0 || used > total {
		t.Fatalf("invalid memory usage: %d / %d MiB", used, total)
	}
}
