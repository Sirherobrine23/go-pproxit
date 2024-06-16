package client

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"time"

	"sirherobrine23.org/Minecraft-Server/go-pproxit/internal/pipe"
	"sirherobrine23.org/Minecraft-Server/go-pproxit/proto"
)

var (
	ErrCannotConnect error = errors.New("cannot connect to controller")
)

type NewClient struct {
	Client proto.Client
	Writer net.Conn
}

type Client struct {
	Token        [36]byte
	RemoteAdress []netip.AddrPort
	clientsTCP   map[string]net.Conn
	clientsUDP   map[string]net.Conn
	NewClient    chan NewClient

	Conn      *net.UDPConn
	AgentInfo *proto.AgentInfo
}

func CreateClient(Addres []netip.AddrPort, Token [36]byte) (*Client, error) {
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

func (client *Client) Send(req proto.Request) error {
	return proto.WriteRequest(client.Conn, req)
}

func (client *Client) Setup() error {
	for _, addr := range client.RemoteAdress {
		var err error
		if client.Conn, err = net.DialUDP("udp", nil, net.UDPAddrFromAddrPort(addr)); err != nil {
			continue
		}
		client.Conn.SetReadDeadline(time.Now().Add(time.Second * 5))
		var auth = proto.AgentAuth(client.Token)
		for {
			client.Send(proto.Request{AgentAuth: &auth})
			res, err := proto.ReaderResponse(client.Conn)
			if err != nil {
				// return err
				break
			} else if res.Unauthorized {
				return ErrCannotConnect
			} else if res.AgentInfo == nil {
				continue
			}
			client.AgentInfo = res.AgentInfo
			client.Conn.SetReadDeadline(*new(time.Time)) // clear timeout
			go client.handlers()
			return nil
		}
	}
	return ErrCannotConnect
}

type toWr struct {
	Proto uint8
	To    netip.AddrPort
	tun   *Client
}

func (t toWr) Write(w []byte) (int, error) {
	err := t.tun.Send(proto.Request{
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
	for {
		res, err := proto.ReaderResponse(client.Conn)
		if err != nil {
			if err == proto.ErrInvalidBody {
				continue
			}
			fmt.Println(err)
			panic(err) // TODO: Require fix to agent shutdown graced
		} else if res.Unauthorized || res.NotListened {
			panic(fmt.Errorf("cannot recive requests")) // TODO: Require fix to agent shutdown graced
		} else if res.SendAuth {
			var auth = proto.AgentAuth(client.Token)
			for {
				client.Send(proto.Request{AgentAuth: &auth})
				res, err := proto.ReaderResponse(client.Conn)
				if err != nil {
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
			}
		}
	}
}
