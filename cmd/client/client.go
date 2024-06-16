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
	Name:    "client",
	Aliases: []string{"c"},
	Usage:   "connect to controller server and bind new requests to local port",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "url",
			Required: true,
			Aliases:  []string{"host", "u"},
			Usage:    `host string to connect to controller, example: "example.com:5522"`,
		},
		&cli.StringFlag{
			Name:     "token",
			Required: true,
			Usage:    "agent token",
			Aliases:  []string{"t"},
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
			Name:     "dial",
			Required: true,
			Usage:    `dial connection, default is "localhost:80"`,
			Aliases:  []string{"d"},
		},
	},
	Action: func(ctx *cli.Context) (err error) {
		var addr netip.AddrPort
		if addr, err = netip.ParseAddrPort(ctx.String("url")); err != nil {
			return
		}
		client, err := client.CreateClient([]netip.AddrPort{addr}, [36]byte([]byte(ctx.String("token"))))
		if err != nil {
			return err
		}
		fmt.Printf("Connected, Remote address: %s\n", client.AgentInfo.AddrPort.String())
		if client.AgentInfo.Protocol == proto.ProtoUDP {
			fmt.Printf("           Port: UDP %d\n", client.AgentInfo.UDPPort)
		} else if client.AgentInfo.Protocol == proto.ProtoTCP {
			fmt.Printf("           Port: TCP %d\n", client.AgentInfo.TCPPort)
		} else if client.AgentInfo.Protocol == proto.ProtoBoth {
			fmt.Printf("           Ports UDP %d and TCP %d\n", client.AgentInfo.UDPPort, client.AgentInfo.TCPPort)
		}

		localConnect := ctx.String("dial")
		for {
			client := <-client.NewClient
			var dial net.Conn
			if client.Client.Proto == proto.ProtoTCP {
				if dial, err = net.Dial("tcp", localConnect); err != nil {
					continue
				}
			} else {
				if dial, err = net.DialUDP("udp", nil, net.UDPAddrFromAddrPort(netip.MustParseAddrPort(localConnect))); err != nil {
					continue
				}
			}
			go io.Copy(client.Writer, dial)
			go io.Copy(dial, client.Writer)
		}
	},
}
