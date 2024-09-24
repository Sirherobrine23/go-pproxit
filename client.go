package gopproxit

import (
	"fmt"
	"io"
	"maps"
	"net"
	"net/netip"
	"reflect"
	"slices"
	"time"

	"sirherobrine23.com.br/Minecraft-Server/go-pproxit/internal/pipe"
	"sirherobrine23.com.br/Minecraft-Server/go-pproxit/internal/structcode"
	"sirherobrine23.com.br/Minecraft-Server/go-pproxit/proto"
)

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
	Token    []byte           // Token to auth in Controller if required
	Conn     net.Conn         // Connection from Controller
	Agent    *proto.AgentInfo // Agent info to show in UI and listened on controller
	LastPong time.Time        // Last Pong time
	Latency  int64            // Latency response in ms from last Pong

	TCPConns                   map[string]*net.TCPConn // Clients connections to TCP
	UDPConns                   map[string]*net.UDPConn // Clients connections to UDP
	newListenTCP, newListenUDP chan net.Conn           // Channel to accept new connections

	errListen chan error // new clients listener in Controller
}

// Dial TCP Connection and return Client
func NewTCPClient(Address string, AuthToken []byte) (*Client, error) {
	conn, err := net.Dial("tcp", Address)
	if err != nil {
		return nil, err
	}
	return NewClient(conn, AuthToken)
}

// Setup net.Conn and return client
func NewClient(conn net.Conn, AuthToken []byte) (*Client, error) {
	var clientStr Client
	clientStr.Conn = conn

	var res proto.Response
	if err := structcode.NewEncode(clientStr.Conn, proto.Request{Ping: proto.Point(time.Now())}); err != nil {
		return nil, err
	} else if err := structcode.NewDecode(clientStr.Conn, &res); err != nil {
		return nil, err
	} else if res.SendAuth && len(AuthToken) == 0 {
		return nil, ErrAuthUnauthorized
	}

	// Auth Session
	if clientStr.Token = AuthToken; len(clientStr.Token) > 0 {
		if err := clientStr.Auth(); err != nil {
			clientStr.Conn.Close()
			return nil, err
		}
	}

	clientStr.newListenTCP, clientStr.newListenUDP = make(chan net.Conn), make(chan net.Conn)
	clientStr.TCPConns, clientStr.UDPConns = make(map[string]*net.TCPConn), make(map[string]*net.UDPConn)
	clientStr.errListen = make(chan error)
	go clientStr.handle() // Process requests
	return &clientStr, nil
}

// Return addr from Dial
func (Client *Client) Addr() net.Addr { return Client.Conn.RemoteAddr() }

// Accept connection from Controller
func (client *Client) Accept() (net.Conn, error) {
	select {
	case v := <-client.newListenTCP:
		return v, nil
	case v := <-client.newListenUDP:
		return v, nil
	case err := <-client.errListen:
		return nil, err
	}
}

// Close Clients and Controller connection
func (client *Client) Close() error {
	if client.Conn != nil {
		client.Conn.Close()
	}

	for _, d := range slices.Collect(maps.Values(client.TCPConns)) {
		d.Close()
	}
	for _, d := range slices.Collect(maps.Values(client.UDPConns)) {
		d.Close()
	}

	close(client.newListenTCP)
	close(client.newListenUDP)
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
	if err := structcode.NewEncode(client.Conn, proto.Request{AgentAuth: client.Token}); err != nil {
		return err
	}
	var res proto.Response
	if err := structcode.NewDecode(client.Conn, &res); err != nil {
		return err
	} else if res.BadRequest || res.Unauthorized {
		return ErrAuthUnauthorized
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
			client.sendErr(ErrAuthUnauthorized) // Close connection
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
					client.newListenTCP <- toClient
					clientConn = client.TCPConns[clientAddr]
				}
				if _, err := clientConn.Write(res.DataRX.Data); err != nil {
					client.sendErr(err)
				}
			case proto.ProtoUDP:
				if clientConn, ok = client.UDPConns[clientAddr]; !ok {
					toClient, toAgent := pipe.CreatePipe(data.Client.Client, data.Client.Client)
					go io.Copy(&toWr{data.Client.Proto, data.Client.Client, client}, toAgent)
					client.newListenUDP <- toClient
					clientConn = client.UDPConns[clientAddr]
				}
				if _, err := clientConn.Write(res.DataRX.Data); err != nil {
					client.sendErr(err)
				}
			}
		}
	}
}
