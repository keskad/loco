package app

import "github.com/keskad/loco/pkgs/commandstation"

// SetSpeedAction sets the speed and direction of a locomotive
func (app *LocoApp) SetSpeedAction(locoId uint8, speed uint8, forward bool, speedSteps uint8) error {
	if cmdErr := app.initializeCommandStation(); cmdErr != nil {
		return cmdErr
	}
	defer app.station.CleanUp()

	return app.station.SetSpeed(commandstation.LocoAddr(locoId), speed, forward, speedSteps)
}

// GetSpeedAction retrieves the current speed and direction of a locomotive
func (app *LocoApp) GetSpeedAction(locoId uint8) (speed uint8, forward bool, err error) {
	if cmdErr := app.initializeCommandStation(); cmdErr != nil {
		return 0, false, cmdErr
	}
	defer app.station.CleanUp()

	return app.station.GetSpeed(commandstation.LocoAddr(locoId))
}
