package client

import (
	"fmt"
	"io"
	"net"
	"net/netip"

	"github.com/google/uuid"
	"github.com/urfave/cli/v2"
	"sirherobrine23.org/Minecraft-Server/go-pproxit/client"
	"sirherobrine23.org/Minecraft-Server/go-pproxit/proto"
)

var CmdClient = cli.Command{
	Name: "client",
	Aliases: []string{"c"},
	Usage: "connect to controller server and bind new requests to local port",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name: "url",
			Required: true,
			Aliases: []string{"host", "u"},
			Usage: `host string to connect to controller, example: "example.com:5522"`,
		},
		&cli.StringFlag{
			Name: "token",
			Required: true,
			Usage: "agent token",
			Aliases: []string{"t"},
			Action: func(ctx *cli.Context, s string) error {
				if _, err := uuid.Parse(s); err == nil {
					return nil
				} else if len(s) == len(proto.AgentAuth{}) {
					return nil
				}
				return fmt.Errorf("set valid token")
			},
		},
		&cli.StringFlag{
			Name: "dial",
			Required: true,
			Usage: `dial connection, default is "localhost:80"`,
			Aliases: []string{"d"},
		},
	},
	Action: func(ctx *cli.Context) (err error) {
		var addr netip.AddrPort
		if addr, err = netip.ParseAddrPort(ctx.String("url")); err != nil {
			return
		}
		client := client.NewClient(addr, [36]byte([]byte(ctx.String("token"))))
		var info *proto.AgentInfo
		if info, err = client.Dial(); err != nil {
			return err
		}
		fmt.Printf("Connected, Remote port: %d\n", info.LitenerPort)
		fmt.Printf("           Remote address: %s\n", info.AddrPort.String())
		go client.Backgroud()

		localConnect := ctx.String("dial")
		for {
			select {
			case tcp := <-client.NewTCPClient:
				go func()  {
					conn, err := net.Dial("tcp", localConnect)
					if err != nil {
						return
					}
					go io.Copy(conn, tcp)
					go io.Copy(tcp, conn)
				}()
			case udp := <-client.NewUDPClient:
				go func ()  {
					conn, err := net.DialUDP("udp", nil, net.UDPAddrFromAddrPort(netip.MustParseAddrPort(localConnect)))
					if err != nil {
						return
					}
					go io.Copy(conn, udp)
					go io.Copy(udp, conn)
				}()
			}
		}
	},
}