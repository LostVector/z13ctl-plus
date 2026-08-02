package cmd

// preset.go - named performance presets and AC/battery assignments.

import (
	"fmt"
	"maps"
	"slices"

	"github.com/dahui/z13ctl/api"

	"github.com/spf13/cobra"
)

var presetCmd = &cobra.Command{
	Use:   "preset",
	Short: "Manage named performance presets and automatic power switching",
}

var presetListCmd = &cobra.Command{
	Use:   "list",
	Short: "List saved presets and power assignments",
	Args:  cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		handled, state, err := api.SendGetState()
		if err != nil {
			return err
		}
		if !handled {
			return fmt.Errorf("preset management requires the z13ctl-plus daemon")
		}
		names := slices.Sorted(maps.Keys(state.Presets))
		for _, name := range names {
			preset := state.Presets[name]
			active := ""
			if name == state.ActivePreset {
				active = " (active)"
			}
			fmt.Printf("%s%s: %s, %d Hz\n", name, active, preset.Profile, preset.RefreshRate)
		}
		if policy := state.PowerPolicy; policy != nil {
			fmt.Printf("Auto: %t; AC=%s; Battery=%s; source=%s\n",
				policy.Enabled, policy.ACPreset, policy.BatteryPreset, state.PowerSource)
		}
		return nil
	},
}

var presetRestoreRecommendedCmd = &cobra.Command{
	Use:   "restore-recommended",
	Short: "Restore and apply the recommended AC/battery presets",
	Args:  cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		if dryRunFlag {
			fmt.Println("Would restore and apply the recommended power presets")
			return nil
		}
		handled, err := api.SendRestoreRecommended()
		if err != nil {
			return err
		}
		if !handled {
			return fmt.Errorf("restoring recommended presets requires the z13ctl-plus daemon")
		}
		return nil
	},
}

func presetAction(use, short string, send func(string) (bool, error)) *cobra.Command {
	return &cobra.Command{
		Use:   use + " NAME",
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if dryRunFlag {
				fmt.Printf("Would %s preset %q\n", use, args[0])
				return nil
			}
			handled, err := send(args[0])
			if err != nil {
				return err
			}
			if !handled {
				return fmt.Errorf("preset management requires the z13ctl-plus daemon")
			}
			return nil
		},
	}
}

var (
	policyEnable  bool
	policyDisable bool
	policyAC      string
	policyBattery string
)

var presetPolicyCmd = &cobra.Command{
	Use:   "policy",
	Short: "Configure automatic AC/battery preset switching",
	Args:  cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		if policyEnable == policyDisable {
			return fmt.Errorf("specify exactly one of --enable or --disable")
		}
		if dryRunFlag {
			fmt.Printf("Would set automatic preset switching: enabled=%t AC=%q Battery=%q\n", policyEnable, policyAC, policyBattery)
			return nil
		}
		if policyAC == "" || policyBattery == "" {
			handled, state, err := api.SendGetState()
			if err != nil {
				return err
			}
			if !handled || state.PowerPolicy == nil {
				return fmt.Errorf("specify both --ac and --battery")
			}
			if policyAC == "" {
				policyAC = state.PowerPolicy.ACPreset
			}
			if policyBattery == "" {
				policyBattery = state.PowerPolicy.BatteryPreset
			}
		}
		handled, err := api.SendPowerPolicySet(policyEnable, policyAC, policyBattery)
		if err != nil {
			return err
		}
		if !handled {
			return fmt.Errorf("preset management requires the z13ctl-plus daemon")
		}
		return nil
	},
}

func init() {
	presetCmd.AddCommand(presetListCmd)
	presetCmd.AddCommand(presetRestoreRecommendedCmd)
	presetCmd.AddCommand(presetAction("save", "Save current settings as a preset", api.SendPresetSave))
	presetCmd.AddCommand(presetAction("apply", "Apply a named preset", api.SendPresetApply))
	presetCmd.AddCommand(presetAction("delete", "Delete an unassigned preset", api.SendPresetDelete))
	presetPolicyCmd.Flags().BoolVar(&policyEnable, "enable", false, "Enable automatic switching")
	presetPolicyCmd.Flags().BoolVar(&policyDisable, "disable", false, "Disable automatic switching")
	presetPolicyCmd.Flags().StringVar(&policyAC, "ac", "", "Preset assigned to AC power")
	presetPolicyCmd.Flags().StringVar(&policyBattery, "battery", "", "Preset assigned to battery power")
	presetCmd.AddCommand(presetPolicyCmd)
	rootCmd.AddCommand(presetCmd)
}
