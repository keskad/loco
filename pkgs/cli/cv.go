package cli

import (
	"fmt"

	"github.com/keskad/loco/pkgs/app"
	"github.com/spf13/cobra"
)

func NewCVCommand(app *app.LocoApp) *cobra.Command {
	command := &cobra.Command{
		Use:   "cv",
		Short: "Read & Write CVs on the locomotives using a command station",
		RunE: func(command *cobra.Command, args []string) error {
			return nil
		},
	}

	command.AddCommand(NewSetCommand(app))
	command.AddCommand(NewGetCommand(app))
	return command
}

func NewSetCommand(app *app.LocoApp) *cobra.Command {
	type SetArgs struct {
		LocoId uint8
		Cv     uint8
		Value  uint16
	}

	cmdArgs := SetArgs{}
	command := &cobra.Command{
		Use:   "set",
		Short: "Send a CV value to the decoder",
		RunE: func(command *cobra.Command, args []string) error {
			if err := app.Initialize(); err != nil {
				return err
			}
			return app.SendCVAction("pom", cmdArgs.LocoId)
		},
	}

	command.Flags().BoolVarP(&app.Debug, "debug", "v", false, "Increase verbosity to the debug level")
	command.Flags().Uint8VarP(&cmdArgs.LocoId, "loco", "l", 0, "Use locomotive under specific address")

	return command
}

func NewGetCommand(app *app.LocoApp) *cobra.Command {
	type SetArgs struct {
		LocoId uint8
		Cv     uint16
		Track  string
	}

	cmdArgs := SetArgs{}
	command := &cobra.Command{
		Use:   "get",
		Short: "Retrieve a CV value from the decoder",
		RunE: func(command *cobra.Command, args []string) error {
			if err := app.Initialize(); err != nil {
				return err
			}

			// mode selection and validation
			track := cmdArgs.Track
			if track != "" && track != "pom" && track != "prog" {
				return fmt.Errorf("invalid track type: %s. Must be either 'pom', 'prog' or empty", track)
			}
			if track == "" {
				track = "pom"
				if cmdArgs.LocoId == 0 {
					track = "prog"
				}
			}
			return app.ReadCVAction(track, cmdArgs.LocoId, uint16(cmdArgs.Cv))
		},
	}

	command.Flags().BoolVarP(&app.Debug, "debug", "v", false, "Increase verbosity to the debug level")
	command.Flags().Uint8VarP(&cmdArgs.LocoId, "loco", "l", 0, "Use locomotive under specific address")
	command.Flags().Uint16VarP(&cmdArgs.Cv, "cv", "c", 0, "CV")
	command.Flags().StringVarP(&cmdArgs.Track, "track", "t", "", "Track type: 'pom' for programming on main, 'prog' for programming track, or empty for automatic selection")

	return command
}
