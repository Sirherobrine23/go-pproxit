package server

import (
	"net/netip"

	"github.com/urfave/cli/v2"
	"sirherobrine23.org/Minecraft-Server/go-pproxit/server"
)

var CmdServer = cli.Command{
	Name:    "server",
	Usage:   "Create local server and open controller ports",
	Aliases: []string{"s"},
	Flags: []cli.Flag{
		&cli.IntFlag{
			Name:    "port",
			Value:   5522,
			Aliases: []string{"p"},
			Usage:   "Set controller port to watcher UDP requests",
		},
		&cli.StringFlag{
			Name:    "log",
			Value:   "silence",
			Aliases: []string{"l"},
			Usage:   "set server log: silence, 0 or verbose, 2",
		},
		&cli.StringFlag{
			Name:    "db",
			Value:   "./pproxit.db",
			Aliases: []string{"d"},
			Usage:   "sqlite file path",
		},
	},
	Action: func(ctx *cli.Context) error {
		calls, err := NewCall(ctx.String("db"))
		if err != nil {
			return err
		}
		pproxitServer, err := server.NewController(calls, netip.AddrPortFrom(netip.IPv4Unspecified(), uint16(ctx.Int("port"))))
		if err != nil {
			return err
		}
		return <-pproxitServer.ProcessError
	},
}
