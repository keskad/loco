package app

import "github.com/keskad/loco/pkgs/commandstation"

func (app *LocoApp) SendFnAction(mode string, locoId uint8, fnNum int, toggle bool) error {
	if cmdErr := app.initializeCommandStation(); cmdErr != nil {
		return cmdErr
	}
	defer app.station.CleanUp()
	return app.station.SendFn(commandstation.Mode(mode), commandstation.LocoAddr(locoId), commandstation.FuncNum(fnNum), toggle)
}

func (app *LocoApp) ListFnAction(locoId uint8) error {
	if cmdErr := app.initializeCommandStation(); cmdErr != nil {
		return cmdErr
	}
	defer app.station.CleanUp()

	activeFunctions, err := app.station.ListFunctions(commandstation.LocoAddr(locoId))
	if err != nil {
		return err
	}

	// Format output: each function on a new line in format "F0 = On"
	if len(activeFunctions) == 0 {
		app.P.Printf("No active functions\n")
	} else {
		for _, fnNum := range activeFunctions {
			app.P.Printf("F%d = On\n", fnNum)
		}
	}

	return nil
}
