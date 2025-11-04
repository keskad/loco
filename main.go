package main

import (
	"os"

	"github.com/keskad/loco/pkgs/app"
	"github.com/keskad/loco/pkgs/cli"
)

func main() {
	app := app.LocoApp{}
	cmd := cli.NewRootCommand(&app)
	args := os.Args
	if args != nil {
		args = args[1:]
		cmd.SetArgs(args)
	}
	err := cmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
