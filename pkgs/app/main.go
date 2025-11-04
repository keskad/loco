package app

import (
	"fmt"

	"github.com/keskad/rb/pkgs/commandstation"
	"github.com/keskad/rb/pkgs/config"
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

func (app *LocoApp) ReadCVAction(mode string, locoId uint8, cvNum uint16) error {
	if cmdErr := app.initializeCommandStation(); cmdErr != nil {
		return cmdErr
	}
	defer app.station.CleanUp()

	result, err := app.station.ReadCV(commandstation.Mode(mode), commandstation.LocoCV{
		LocoId: commandstation.LocoAddr(locoId),
		Cv: commandstation.CV{
			Num: commandstation.CVNum(cvNum),
		},
	})
	if err != nil {
		return err
	}

	print(fmt.Sprintf("%d", result))
	return nil
}
