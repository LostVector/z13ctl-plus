package cmd

// npu.go — "npu" subcommand: get or set the AMD XDNA NPU DPM power mode via
// the amdxdna DRM accel driver. The mode is volatile (resets on sleep); the
// daemon does not persist it across reboots.

import (
	"fmt"

	"github.com/dahui/z13ctl/api"
	"github.com/dahui/z13ctl/internal/cli"

	"github.com/spf13/cobra"
)

var (
	npuGetFlag bool
	npuSetFlag string
)

var npuCmd = &cobra.Command{
	Use:   "npu",
	Short: "Get or set the NPU power mode via amdxdna DRM ioctl",
	Long: `Get or set the AMD XDNA NPU DPM power mode.

The mode controls the NPU's Dynamic Power Management profile. It is
volatile: it resets to DEFAULT on sleep and shutdown; the daemon does
not persist it.

Values (accepts name or number, case-insensitive):
  0 / default — calculated DPM (on Strix Halo: max, with clk_gating on)
  3 / high    — highest DPM, clk_gating on (identical to DEFAULT on NPU5)
  4 / turbo   — highest DPM, clk_gating off; refused if any NPU context
                 is active (e.g. a model is loaded)

The uAPI also defines LOW (1) and MEDIUM (2), but the amdxdna driver
returns EOPNOTSUPP for them — they are not exposed by this command.

Note: SET_STATE is DRM_ROOT_ONLY in the amdxdna driver, so --set requires
root (use sudo, or setcap cap_sys_admin+ep on the binary). --get is
unprivileged.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if !npuGetFlag && npuSetFlag == "" {
			return cmd.Help()
		}

		if npuSetFlag != "" {
			mode, ok := cli.NPUPowerModeFromString(npuSetFlag)
			if !ok {
				return fmt.Errorf("invalid value %q: must be 0/3/4 or default/high/turbo", npuSetFlag)
			}

			if dryRunFlag {
				cli.DryRunNPUPowerMode(mode)
				return nil
			}

			if handled, err := api.SendNPUPowerModeSet(mode); handled {
				if err != nil {
					return err
				}
				fmt.Printf("NPU power mode set to %d (%s)\n", mode, cli.NPUPowerModeName(mode))
				return nil
			}

			if err := cli.SetNPUPowerMode(mode); err != nil {
				return fmt.Errorf("setting NPU power mode: %w\n  (amdxdna SET_STATE is DRM_ROOT_ONLY — try: sudo z13ctl npu --set %s)", err, npuSetFlag)
			}
			fmt.Printf("NPU power mode set to %d (%s)\n", mode, cli.NPUPowerModeName(mode))
			return nil
		}

		// --get: read directly via ioctl for ground truth.
		mode, err := cli.ReadNPUPowerMode()
		if err != nil {
			return fmt.Errorf("reading NPU power mode: %w", err)
		}
		fmt.Printf("%d (%s)\n", mode, cli.NPUPowerModeName(mode))
		return nil
	},
}

func init() {
	npuCmd.Flags().BoolVar(&npuGetFlag, "get", false, "Print the current NPU power mode")
	npuCmd.Flags().StringVar(&npuSetFlag, "set", "", "Set NPU power mode (0/3/4 or default/high/turbo)")
	rootCmd.AddCommand(npuCmd)
}
