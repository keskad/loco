package cli

import (
	"errors"
	"fmt"

	"github.com/keskad/loco/pkgs/app"
	"github.com/spf13/cobra"
)

func NewCVCommand(app *app.LocoApp) *cobra.Command {
	command := &cobra.Command{
		Use:   "cv",
		Short: "Read & Write CVs on the locomotives using a command station",
		RunE: func(command *cobra.Command, args []string) error {
			return errors.New("please select a command")
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
	type GetArgs struct {
		LocoId uint8
		Track  string
	}

	cmdArgs := GetArgs{}
	command := &cobra.Command{
		Use:   "get",
		Short: "Retrieve a CV value from the decoder",
		Args:  cobra.ArbitraryArgs,
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

			// Join all args before '--' as CV string
			cvString := ""
			if len(args) > 0 {
				cvString = args[0]
				if len(args) > 1 {
					cvString = ""
					for i, a := range args {
						if a == "--" {
							break
						}
						if i > 0 {
							cvString += " "
						}
						cvString += a
					}
				}
			}
			if cvString == "" {
				return fmt.Errorf("no CV argument provided")
			}
			return app.ReadCVAction(track, cmdArgs.LocoId, cvString)
		},
	}

	command.Flags().BoolVarP(&app.Debug, "debug", "v", false, "Increase verbosity to the debug level")
	command.Flags().Uint8VarP(&cmdArgs.LocoId, "loco", "l", 0, "Use locomotive under specific address")
	command.Flags().StringVarP(&cmdArgs.Track, "track", "t", "", "Track type: 'pom' for programming on main, 'prog' for programming track, or empty for automatic selection")

	return command
}
