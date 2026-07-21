package cmd

// display.go - publish posture and rotation intent to the display coordinator.

import (
	"fmt"

	"github.com/dahui/z13ctl/api"

	"github.com/spf13/cobra"
)

var displayCmd = &cobra.Command{
	Use:   "display",
	Short: "Coordinate internal display posture and rotation",
}

func displayIntentCommand(use, short string, send func(string) (bool, error)) *cobra.Command {
	return &cobra.Command{
		Use:   use + " VALUE",
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if dryRunFlag {
				fmt.Printf("Would set display %s to %q\n", use, args[0])
				return nil
			}
			handled, err := send(args[0])
			if err != nil {
				return err
			}
			if !handled {
				return fmt.Errorf("display coordination requires the z13ctl daemon")
			}
			return nil
		},
	}
}

func init() {
	displayCmd.AddCommand(displayIntentCommand("posture", "Set desktop, laptop, or tablet posture", api.SendDisplayPosture))
	displayCmd.AddCommand(displayIntentCommand("rotation", "Set none, left, right, or inverted rotation", api.SendDisplayRotation))
	rootCmd.AddCommand(displayCmd)
}
