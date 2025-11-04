package cli

import (
	"github.com/keskad/loco/pkgs/app"
	"github.com/spf13/cobra"
)

func NewRootCommand(app *app.LocoApp) *cobra.Command {
	command := &cobra.Command{
		Use:   "rb",
		Short: "Unofficial Railbox Command Station & Decoder CLI",
		RunE: func(command *cobra.Command, args []string) error {
			return nil
		},
	}

	command.AddCommand(NewCVCommand(app))

	return command
}
