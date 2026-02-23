package cli

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/keskad/loco/pkgs/app"
	"github.com/keskad/loco/pkgs/decoders"
	"github.com/spf13/cobra"
)

func NewDecoderCommand(app *app.LocoApp) *cobra.Command {
	command := &cobra.Command{
		Use:   "decoder",
		Short: "Decoder-specific commands",
		RunE: func(command *cobra.Command, args []string) error {
			return errors.New("please select a command")
		},
	}

	command.AddCommand(NewDecoderRBCommand(app))

	return command
}

func NewDecoderRBCommand(app *app.LocoApp) *cobra.Command {
	command := &cobra.Command{
		Use:   "rb",
		Short: "Commands for Railbox RB23xx decoders",
		RunE: func(command *cobra.Command, args []string) error {
			return errors.New("please select a command")
		},
	}

	command.AddCommand(NewDecoderRBSoundCommand(app))

	return command
}

func NewDecoderRBSoundCommand(app *app.LocoApp) *cobra.Command {
	command := &cobra.Command{
		Use:   "sound",
		Short: "Sound management for Railbox RB23xx decoders",
		RunE: func(command *cobra.Command, args []string) error {
			return errors.New("please select a command")
		},
	}

	command.AddCommand(NewDecoderRBSoundClearCommand(app))
	command.AddCommand(NewDecoderRBSoundSyncCommand(app))

	return command
}

func NewDecoderRBSoundClearCommand(app *app.LocoApp) *cobra.Command {
	type Args struct {
		Timeout uint16
	}
	cmdArgs := Args{}

	command := &cobra.Command{
		Use:   "clear <slot>",
		Short: "Clear sound files from a slot on the Railbox RB23xx decoder",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			slot64, err := strconv.ParseUint(args[0], 10, 8)
			if err != nil {
				return fmt.Errorf("invalid slot number %q: %w", args[0], err)
			}

			return app.ClearSoundSlot(uint8(slot64), decoders.WithTimeout(cmdArgs.Timeout))
		},
	}

	command.Flags().BoolVarP(&app.Debug, "debug", "v", false, "Increase verbosity to the debug level")
	command.Flags().Uint16VarP(&cmdArgs.Timeout, "timeout", "", 10, "HTTP connection timeout in seconds")

	return command
}

func NewDecoderRBSoundSyncCommand(app *app.LocoApp) *cobra.Command {
	type Args struct {
		Timeout uint16
		DryRun  bool
	}
	cmdArgs := Args{}

	command := &cobra.Command{
		Use:   "sync <slot> <local-dir>",
		Short: "Synchronise a local directory with a sound slot on the Railbox RB23xx decoder",
		Long: `Compares the contents of a local directory with the given sound slot on the decoder.
Files present locally but missing on the decoder are uploaded.
Files present on the decoder but missing locally are deleted from the decoder.
Files present on both sides but differing in size are re-uploaded.`,
		Args: cobra.ExactArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			slot64, err := strconv.ParseUint(args[0], 10, 8)
			if err != nil {
				return fmt.Errorf("invalid slot number %q: %w", args[0], err)
			}

			return app.SyncSoundSlot(uint8(slot64), args[1], cmdArgs.DryRun, decoders.WithTimeout(cmdArgs.Timeout))
		},
	}

	command.Flags().BoolVarP(&app.Debug, "debug", "v", false, "Increase verbosity to the debug level")
	command.Flags().Uint16VarP(&cmdArgs.Timeout, "timeout", "", 10, "HTTP connection timeout in seconds")
	command.Flags().BoolVar(&cmdArgs.DryRun, "dry-run", false, "Preview changes without uploading or deleting any files")

	return command
}
