package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"time"

	"sirherobrine23.org/Minecraft-Server/go-pproxit/internal/pipe"
	"sirherobrine23.org/Minecraft-Server/go-pproxit/internal/structcode"
	"sirherobrine23.org/Minecraft-Server/go-pproxit/proto"
)

var (
	ErrCannotConnect error = errors.New("cannot connect to controller")
	ErrUnathorized   error = errors.New("cannot auth in controller")
)

type NewClient struct {
	Client proto.Client
	Writer net.Conn
}

type Client struct {
	Token        []byte
	RemoteAdress netip.AddrPort
	clientsTCP   map[string]net.Conn
	clientsUDP   map[string]net.Conn
	NewClient    chan NewClient

	Conn      net.Conn
	AgentInfo *proto.AgentInfo
}

func CreateClient(Addres netip.AddrPort, Token []byte) (*Client, error) {
	cli := &Client{
		Token:        Token,
		RemoteAdress: Addres,
		clientsTCP:   make(map[string]net.Conn),
		clientsUDP:   make(map[string]net.Conn),
		NewClient:    make(chan NewClient),
	}
	if err := cli.Setup(); err != nil {
		return cli, err
	}
	return cli, nil
}

func (client *Client) Setup() error {
	var err error
	if client.Conn, err = net.DialTCP("tcp", nil, net.TCPAddrFromAddrPort(client.RemoteAdress)); err != nil {
		return err
	}
	for attemps := 0; attemps < 18; attemps++ {
		if err := structcode.NewEncode(client.Conn, proto.Request{AgentAuth: &client.Token}); err != nil {
			return err
		}

		var res proto.Response
		if err = structcode.NewDecode(client.Conn, &res); err != nil {
			return fmt.Errorf("decode status from server, error: %s", err.Error())
		} else if res.Unauthorized {
			return ErrUnathorized
		} else if res.AgentInfo == nil {
			continue
		}

		client.AgentInfo = res.AgentInfo
		go client.handlers()
		return nil
	}
	return ErrCannotConnect
}

type toWr struct {
	Proto uint8
	To    netip.AddrPort
	tun   *Client
}

func (t toWr) Write(w []byte) (int, error) {
	err := structcode.NewEncode(t.tun.Conn, proto.Request{
		DataTX: &proto.ClientData{
			Client: proto.Client{
				Client: t.To,
				Proto:  t.Proto,
			},
			Size: uint64(len(w)),
			Data: w[:],
		},
	})
	if err == nil {
		return len(w), nil
	}
	return 0, err
}

func (tun *Client) GetTargetWrite(Proto uint8, To netip.AddrPort) io.Writer {
	return &toWr{Proto: Proto, To: To, tun: tun}
}

func (client *Client) handlers() {
	var lastPing int64 = 0
	for {
		if time.Now().UnixMilli()-lastPing > 3_000 {
			var req proto.Request
			req.Ping = new(time.Time)
			*req.Ping = time.Now()
			go structcode.NewEncode(client.Conn, req)
		}

		var res proto.Response
		err := structcode.NewDecode(client.Conn, &res)
		if err != nil {
			fmt.Println(err)
			if err == proto.ErrInvalidBody {
				continue
			}
			panic(err) // TODO: Require fix to agent shutdown graced
		}

		d, _ := json.Marshal(res)
		fmt.Println(string(d))

		if res.Pong != nil {
			lastPing = res.Pong.UnixMilli()
			continue
		}
		if res.Unauthorized || res.NotListened {
			panic(fmt.Errorf("cannot recive requests")) // TODO: Require fix to agent shutdown graced
		} else if res.SendAuth {
			var auth = client.Token
			for {
				structcode.NewEncode(client.Conn, proto.Request{AgentAuth: &auth})
				var res proto.Response
				if err = structcode.NewDecode(client.Conn, &res); err != nil {
					panic(err) // TODO: Require fix to agent shutdown graced
				} else if res.Unauthorized {
					return
				} else if res.AgentInfo == nil {
					continue
				}
				client.AgentInfo = res.AgentInfo
				break
			}
		} else if cl := res.CloseClient; res.CloseClient != nil {
			if cl.Proto == proto.ProtoTCP {
				if tun, ok := client.clientsTCP[cl.Client.String()]; ok {
					tun.Close()
				}
			} else if cl.Proto == proto.ProtoUDP {
				if tun, ok := client.clientsUDP[cl.Client.String()]; ok {
					tun.Close()
				}
			}
		} else if data := res.DataRX; res.DataRX != nil {
			if data.Client.Proto == proto.ProtoTCP {
				if _, ok := client.clientsTCP[data.Client.Client.String()]; !ok {
					toClient, toAgent := pipe.CreatePipe(net.TCPAddrFromAddrPort(data.Client.Client), net.TCPAddrFromAddrPort(data.Client.Client))
					client.NewClient <- NewClient{
						Client: data.Client,
						Writer: toClient,
					}
					client.clientsTCP[data.Client.Client.String()] = toAgent
					go func() {
						io.Copy(client.GetTargetWrite(proto.ProtoTCP, data.Client.Client), toAgent)
						delete(client.clientsTCP, data.Client.Client.String())
					}()
				}
			} else if data.Client.Proto == proto.ProtoUDP {
				if _, ok := client.clientsUDP[data.Client.Client.String()]; !ok {
					toClient, toAgent := pipe.CreatePipe(net.UDPAddrFromAddrPort(data.Client.Client), net.UDPAddrFromAddrPort(data.Client.Client))
					client.NewClient <- NewClient{
						Client: data.Client,
						Writer: toClient,
					}
					client.clientsUDP[data.Client.Client.String()] = toAgent
					go func() {
						io.Copy(client.GetTargetWrite(proto.ProtoUDP, data.Client.Client), toAgent)
						delete(client.clientsUDP, data.Client.Client.String())
						toAgent.Close()
					}()
				}
			}

			if data.Client.Proto == proto.ProtoTCP {
				if tun, ok := client.clientsTCP[data.Client.Client.String()]; ok {
					go tun.Write(data.Data)
				}
			} else if data.Client.Proto == proto.ProtoUDP {
				if tun, ok := client.clientsUDP[data.Client.Client.String()]; ok {
					go tun.Write(data.Data)
				}
			} else if res.Pong != nil {
				fmt.Println(res.Pong.String())
			}
		}
	}
}
