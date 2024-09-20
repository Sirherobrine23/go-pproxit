package client

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"reflect"
	"time"

	"sirherobrine23.com.br/Minecraft-Server/go-pproxit/internal/pipe"
	"sirherobrine23.com.br/Minecraft-Server/go-pproxit/internal/structcode"
	"sirherobrine23.com.br/Minecraft-Server/go-pproxit/proto"
)

var ErrUnauthorized error = errors.New("cannot auth to controller")

type remoteClient struct {
	Client proto.Client
	Conn   net.Conn
}

type toWr struct {
	Proto proto.Protoc
	To    netip.AddrPort
	tun   *Client
}

func (t toWr) Write(w []byte) (int, error) {
	data := proto.Request{
		DataTX: &proto.ClientData{
			Data: w,
			Client: proto.Client{
				Client: t.To,
				Proto:  t.Proto,
			},
		},
	}
	if err := structcode.NewEncode(t.tun.Conn, data); err != nil {
		return 0, err
	}
	return len(w), nil
}

type Client struct {
	Token    []byte                  // Token to auth in Controller if required
	Conn     net.Conn                // Connection from Controller
	Agent    *proto.AgentInfo        // Agent info to show in UI and listened on controller
	TCPConns map[string]*net.TCPConn // Clients connections to TCP
	UDPConns map[string]*net.UDPConn // Clients connections to UDP
	LastPong time.Time               // Last Pong time
	Latency  int64                   // Latency response in ms from last Pong

	newListen chan remoteClient // new clients listener in Controller
	errListen chan error        // new clients listener in Controller
}

func NewClient(Address string, AuthToken []byte) (*Client, error) {
	var clientStr Client
	var err error

	// Dial connection
	if clientStr.Conn, err = net.Dial("tcp", Address); err != nil {
		return nil, err
	}

	var res proto.Response
	if err := structcode.NewEncode(clientStr.Conn, proto.Request{Ping: proto.Point(time.Now())}); err != nil {
		return nil, err
	} else if err := structcode.NewDecode(clientStr.Conn, &res); err != nil {
		return nil, err
	} else if res.SendAuth && len(AuthToken) == 0 {
		return nil, ErrUnauthorized
	}

	// Auth Session
	if clientStr.Token = AuthToken; len(clientStr.Token) > 0 {
		if err := clientStr.Auth(); err != nil {
			clientStr.Conn.Close()
			return nil, err
		}
	}

	clientStr.errListen = make(chan error)
	clientStr.newListen = make(chan remoteClient)
	clientStr.TCPConns = make(map[string]*net.TCPConn)
	clientStr.UDPConns = make(map[string]*net.UDPConn)
	go clientStr.handle() // Process requests
	return &clientStr, nil
}

// Wait for any error
func (client *Client) WaitError() error {
	if err, ok := <-client.errListen; ok {
		return err
	}
	return io.EOF
}

// Wait for any error, if catch error close connection and return error
func (client *Client) WaitCloseError() error {
	err := client.WaitError()
	if err != nil && client.Conn != nil {
		client.Close()
	}
	return err
}

// Close Clients and Controller connection
func (client *Client) Close() error {
	for remoteClient := range client.TCPConns {
		d := client.TCPConns[remoteClient]
		if err := d.Close(); err != nil {
			return err
		}
	}
	for remoteClient := range client.UDPConns {
		d := client.UDPConns[remoteClient]
		if err := d.Close(); err != nil {
			return err
		}
	}
	if client.Conn != nil {
		if err := client.Conn.Close(); err != nil {
			return err
		}
		client.Conn = nil
	}
	close(client.newListen)
	close(client.errListen)
	return nil
}

// Send error to WaitError, if closed catcher and ignored
func (client *Client) sendErr(err error) {
	if reflect.ValueOf(client.errListen).IsZero() {
		return
	}
	defer func() { recover() }()
	client.errListen <- err
}

// Send auth to controller
func (client *Client) Auth() error {
	if err := structcode.NewEncode(client.Conn, proto.Request{AgentAuth: &client.Token}); err != nil {
		return err
	}
	var res proto.Response
	if err := structcode.NewDecode(client.Conn, &res); err != nil {
		return err
	} else if res.BadRequest || res.Unauthorized {
		return ErrUnauthorized
	} else if res.AgentInfo == nil {
		return fmt.Errorf("cannot get agent info")
	}

	client.Agent = res.AgentInfo
	return nil
}

// Process Controller responses
func (client *Client) handle() {
	for {
		var res proto.Response
		if err := structcode.NewDecode(client.Conn, &res); err != nil {
			client.sendErr(err)
			return
		}

		// Resend auth to Controller
		if res.SendAuth {
			if err := client.Auth(); err != nil {
				client.sendErr(err) // Send to handler
				return
			}
			continue
		} else if res.Unauthorized {
			client.sendErr(ErrUnauthorized) // Close connection
			return
		}

		// Update current agent info
		if res.AgentInfo != nil {
			client.Agent = res.AgentInfo
		}

		// Server pong time
		if res.Pong != nil {
			client.Latency = time.Now().UnixMilli() - res.Pong.UnixMilli()
			client.LastPong = *res.Pong
		}

		// Write data to Client
		if data := res.DataRX; res.DataRX != nil {
			var ok bool
			var clientConn net.Conn
			clientAddr := res.DataRX.Client.Client.String()
			switch res.DataRX.Client.Proto {
			case proto.ProtoTCP:
				if clientConn, ok = client.TCPConns[clientAddr]; !ok {
					toClient, toAgent := pipe.CreatePipe(data.Client.Client, data.Client.Client)
					go io.Copy(&toWr{data.Client.Proto, data.Client.Client, client}, toAgent)
					client.newListen <- remoteClient{res.DataRX.Client, toClient}
					clientConn = client.TCPConns[clientAddr]
				}
			case proto.ProtoUDP:
				if clientConn, ok = client.UDPConns[clientAddr]; !ok {
					toClient, toAgent := pipe.CreatePipe(data.Client.Client, data.Client.Client)
					go io.Copy(&toWr{data.Client.Proto, data.Client.Client, client}, toAgent)
					client.newListen <- remoteClient{res.DataRX.Client, toClient}
					clientConn = client.UDPConns[clientAddr]
				}
			default:
				continue
			}
			if _, err := clientConn.Write(res.DataRX.Data); err != nil {
				client.sendErr(err)
				continue
			}
		}

		if res.CloseClient != nil {
			clientAddr := res.DataRX.Client.Client.String()
			switch res.DataRX.Client.Proto {
			case proto.ProtoTCP:
				if clientConn, ok := client.TCPConns[clientAddr]; ok {
					delete(client.TCPConns, clientAddr)
					if err := clientConn.Close(); err != nil {
						client.sendErr(err)
					}
				}
			case proto.ProtoUDP:
				if clientConn, ok := client.UDPConns[clientAddr]; ok {
					delete(client.UDPConns, clientAddr)
					if err := clientConn.Close(); err != nil {
						client.sendErr(err)
					}
				}
			}
		}
	}
}

// Accept connection from Controller
func (client *Client) Accept() (proto.Protoc, net.Conn, error) {
	if conn, ok := <-client.newListen; ok {
		return conn.Client.Proto, conn.Conn, nil
	}
	return 0, nil, io.EOF
}
