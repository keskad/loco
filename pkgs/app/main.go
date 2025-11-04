package app

import (
	"fmt"

	"github.com/keskad/loco/pkgs/syntax"

	"github.com/keskad/loco/pkgs/commandstation"
	"github.com/keskad/loco/pkgs/config"
	"github.com/sirupsen/logrus"
)

type LocoApp struct {
	Config  *config.Configuration
	station commandstation.Station

	// runtime parameters
	Debug bool
}

// Initialize is running after parsing the arguments, so we know how to configure the app
func (app *LocoApp) Initialize() error {
	// logging
	if app.Debug {
		logrus.SetLevel(logrus.DebugLevel)
	}

	// configuration
	logrus.Debug("Reading configuration files")
	cfg, cfgErr := config.NewConfig()
	app.Config = cfg
	if cfgErr != nil {
		return fmt.Errorf("cannot initialize app: %s", cfgErr)
	}
	return nil
}

func (app *LocoApp) initializeCommandStation() error {
	// initialize Command Station communication
	logrus.Debug("Initializing command station")
	if app.Config.Server.Type == "z21" {
		cmd, cmdErr := commandstation.NewZ21Roco(app.Config.Server.Address, app.Config.Server.Port)
		app.station = cmd
		if cmdErr != nil {
			return fmt.Errorf("cannot initialize app: %s", cmdErr)
		}
	} else {
		return fmt.Errorf("unknown command station type '%s'", app.Config.Server.Type)
	}
	return nil
}

func (app *LocoApp) SendCVAction(mode string, locoId uint8) error {
	if cmdErr := app.initializeCommandStation(); cmdErr != nil {
		return cmdErr
	}

	// app.station.WriteCV(mode, )

	return nil
}

func (app *LocoApp) ReadCVAction(mode string, locoId uint8, cvNumRaw string) error {
	if cmdErr := app.initializeCommandStation(); cmdErr != nil {
		return cmdErr
	}
	defer app.station.CleanUp()

	// Try to parse as a single CV
	entries, parseErr := syntax.ParseCVString(cvNumRaw, ",")
	if parseErr == nil && len(entries) >= 1 {
		if len(entries) == 1 {
			entry := entries[0]
			result, err := app.station.ReadCV(commandstation.Mode(mode), commandstation.LocoCV{
				LocoId: commandstation.LocoAddr(locoId),
				Cv: commandstation.CV{
					Num: commandstation.CVNum(entry.Number),
				},
			})
			if err != nil {
				return err
			}
			fmt.Printf("%d\n", result)
		} else {
			for _, entry := range entries {
				result, err := app.station.ReadCV(commandstation.Mode(mode), commandstation.LocoCV{
					LocoId: commandstation.LocoAddr(locoId),
					Cv: commandstation.CV{
						Num: commandstation.CVNum(entry.Number),
					},
				})
				if err != nil {
					fmt.Printf("cv%d=ERROR\n", entry.Number)
				} else {
					fmt.Printf("cv%d=%d\n", entry.Number, result)
				}
			}
		}
		return nil
	}

	return fmt.Errorf("invalid format: %s", cvNumRaw)
}
