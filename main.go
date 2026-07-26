// z13ctl-plus — system control for the 2025 ASUS ROG Flow Z13
//
// Controls keyboard and lightbar RGB (via Linux hidraw), performance profile,
// and battery charge limit (via asus-wmi sysfs interfaces).
//
// Usage:
//
//	z13ctl [--device keyboard|lightbar|PATH] [--dry-run] <command>
//
// Commands: apply  brightness  list  off  profile  batterylimit  setup
package main

import (
	"github.com/dahui/z13ctl/cmd"
	"github.com/spf13/cobra"
)

func main() {
	cobra.CheckErr(cmd.Execute())
}
