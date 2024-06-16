package main

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"
	"sirherobrine23.org/Minecraft-Server/go-pproxit/cmd/client"
	"sirherobrine23.org/Minecraft-Server/go-pproxit/cmd/server"
)

var description string = `pproxit is a proxy that allows anyone to host a server without port forwarding. We use tunneling. Only the server needs to run the program, not every player!`

func main() {
	app := cli.NewApp()
	app.Name = "pproxit"
	app.Description = description
	app.EnableBashCompletion = true
	app.HideHelpCommand = true
	app.Commands = []*cli.Command{
		&server.CmdServer,
		&client.CmdClient,
	}
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(app.ErrWriter, "Error: %v\n", err)
		os.Exit(1)
	}
}
