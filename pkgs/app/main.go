package app

import (
	"fmt"
	"time"

	"github.com/keskad/loco/pkgs/output"
	"github.com/keskad/loco/pkgs/syntax"

	"github.com/keskad/loco/pkgs/commandstation"
	"github.com/keskad/loco/pkgs/config"
	"github.com/sirupsen/logrus"
)

//
// Actions - a controller level
// prints are allowed only via Printer interface
//
// The controller level is intended to provide a layer of performing actions - everything needed to perform a single action e.g. Read list of given CV's
//

type LocoApp struct {
	Config  *config.Configuration
	station commandstation.Station

	// runtime parameters
	Debug bool
	P     output.Printer
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

func (app *LocoApp) SendCVAction(mode string, locoId uint8, cvNumRaw string, verify bool, timeout time.Duration, settle time.Duration) error {
	if cmdErr := app.initializeCommandStation(); cmdErr != nil {
		return cmdErr
	}
	defer app.station.CleanUp()

	entries, parseErr := syntax.ParseCVString(cvNumRaw, ",")
	if parseErr != nil {
		return parseErr
	}

	var writeErr error
	for _, entry := range entries {
		writeErr = app.station.WriteCV(commandstation.Mode(mode), commandstation.LocoCV{
			LocoId: commandstation.LocoAddr(locoId),
			Cv: commandstation.CV{
				Num:   commandstation.CVNum(entry.Number),
				Value: int(entry.Value),
			},
		},
			commandstation.Verify(verify),
			commandstation.Timeout(timeout))

		time.Sleep(settle)

		if writeErr != nil {
			return writeErr
		}
	}

	return nil
}

func (app *LocoApp) ReadCVAction(mode string, locoId uint8, cvNumRaw string, verify bool, timeout time.Duration, retries uint8) error {
	if cmdErr := app.initializeCommandStation(); cmdErr != nil {
		return cmdErr
	}
	defer app.station.CleanUp()

	// Try to parse as a single CV
	entries, parseErr := syntax.ParseCVString(cvNumRaw, ",")
	if parseErr == nil {
		var lastError error

		for _, entry := range entries {
			result, err := app.station.ReadCV(commandstation.Mode(mode), commandstation.LocoCV{
				LocoId: commandstation.LocoAddr(locoId),
				Cv: commandstation.CV{
					Num: commandstation.CVNum(entry.Number),
				},
			}, commandstation.Verify(verify),
				commandstation.Timeout(timeout),
				commandstation.Retries(retries))

			// different formatting mode for multiple than for single entry
			if len(entries) > 1 {
				if err != nil {
					app.P.Printf("cv%d=ERROR\n", entry.Number)
					logrus.Error(err)
					lastError = err
				} else {
					app.P.Printf("cv%d=%d\n", entry.Number, result)
				}
			} else {
				if err != nil {
					return err
				}
				app.P.Printf("%d\n", result)
			}
		}
		return lastError
	}

	return fmt.Errorf("invalid format: %s", cvNumRaw)
}
