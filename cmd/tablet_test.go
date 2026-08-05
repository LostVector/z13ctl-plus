package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestTabletTwoFingerHoldContextMenuFlag(t *testing.T) {
	flag := tabletSettingsSetCmd.Flags().Lookup("two-finger-hold-context-menu")
	if flag == nil {
		t.Fatal("two-finger-hold-context-menu flag is missing")
	}
	if flag.DefValue != "false" {
		t.Fatalf("two-finger-hold-context-menu default = %q, want false", flag.DefValue)
	}
	if _, required := flag.Annotations[cobra.BashCompOneRequiredFlag]; !required {
		t.Fatal("two-finger-hold-context-menu flag is not required")
	}
}
