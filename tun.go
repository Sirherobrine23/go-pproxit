package gopproxit

import (
	"io"
	"maps"
	"net"
	"net/netip"
	"reflect"
	"slices"
	"sync"

	"sirherobrine23.com.br/Minecraft-Server/go-pproxit/internal/structcode"
	"sirherobrine23.com.br/Minecraft-Server/go-pproxit/internal/udplisterner"
	"sirherobrine23.com.br/Minecraft-Server/go-pproxit/proto"
)

type Tunnel struct {
	Controller net.Conn         // Tunnel client connection
	Agent      *proto.AgentInfo // Agent info
	TunErr     chan error       // Return errors
	Done       chan struct{}    // Closed connection

	TCPServer *net.TCPListener
	UDPServer *udplisterner.UDPServer

	TCPLocker, UDPLocker sync.Locker
	TCPConns             map[string]net.Conn
	UDPConns             map[string]net.Conn
}

func NewTun(conn net.Conn, Agent *proto.AgentInfo) *Tunnel {
	var tun Tunnel
	tun.Agent = Agent
	tun.Controller = conn

	// Listen clients
	tun.TCPLocker, tun.UDPLocker = &sync.Mutex{}, &sync.Mutex{}
	tun.TCPConns, tun.UDPConns = make(map[string]net.Conn), make(map[string]net.Conn)
	tun.TunErr = make(chan error)
	tun.Done = make(chan struct{})

	if Agent.AddrPort.Compare(netip.AddrPortFrom(netip.IPv4Unspecified(), 0)) == 0 {
		switch Agent.Protocol {
		case proto.ProtoTCP:
			go tun.TCPServerhandler()
		case proto.ProtoUDP:
			go tun.UDPServerhandler()
		case proto.ProtoBoth:
			go tun.TCPServerhandler()
			go tun.UDPServerhandler()
		}
	}

	return &tun
}
func (tun *Tunnel) Close() error {
	if tun.Controller != nil {
		tun.Controller.Close()
	}
	if tun.TCPServer != nil {
		tun.TCPServer.Close()
		tun.TCPServer = nil
	}
	if tun.UDPServer != nil {
		tun.UDPServer.Close()
		tun.UDPServer = nil
	}

	for _, d := range slices.Collect(maps.Values(tun.TCPConns)) {
		d.Close()
	}
	for _, d := range slices.Collect(maps.Values(tun.UDPConns)) {
		d.Close()
	}

	tun.Done <- struct{}{}
	close(tun.Done)
	close(tun.TunErr)
	return nil
}

// Send error to WaitError, if closed catcher and ignored
func (tun *Tunnel) sendErr(err error) {
	if reflect.ValueOf(tun.TunErr).IsZero() {
		return
	}
	defer func() { recover() }()
	tun.TunErr <- err
}

func (tun *Tunnel) Handler() {
	defer tun.Close()
	for {
		var req proto.Request
		if err := structcode.NewDecode(tun.Controller, &req); err != nil {
			if err == io.EOF {
				return
			}
			continue
		}

		go func(req proto.Request) {
			if data := *req.ClientClose; req.ClientClose != nil {
				switch data.Proto {
				case proto.ProtoTCP:
					if client, ok := tun.TCPConns[data.Client.String()]; ok {
						client.Close()
						delete(tun.TCPConns, data.Client.String())
					}
				case proto.ProtoUDP:
					if client, ok := tun.UDPConns[data.Client.String()]; ok {
						client.Close()
						delete(tun.UDPConns, data.Client.String())
					}
				}
			}

			if data := *req.DataTX; req.DataTX != nil {
				var conn net.Conn
				var ok bool
				switch data.Client.Proto {
				case proto.ProtoTCP:
					if conn, ok = tun.TCPConns[data.Client.Client.String()]; !ok {
						return
					}
				case proto.ProtoUDP:
					if conn, ok = tun.UDPConns[data.Client.Client.String()]; !ok {
						return
					}
				}
				conn.Write(data.Data)
			}
		}(req)
	}
}

type rx struct {
	root  net.Conn
	proto proto.Protoc
}

func (t rx) Write(p []byte) (int, error) {
	err := structcode.NewEncode(t.root, proto.Response{
		DataRX: &proto.ClientData{
			Data: p,
			Client: proto.Client{
				Proto:  t.proto,
				Client: netip.MustParseAddrPort(t.root.RemoteAddr().String()),
			},
		},
	})
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func (tun *Tunnel) TCPServerhandler() {
	var err error
	if tun.TCPServer, err = net.ListenTCP("tcp", net.TCPAddrFromAddrPort(netip.AddrPortFrom(netip.IPv4Unspecified(), tun.Agent.TCPPort))); err != nil {
		tun.sendErr(err)
		return
	}
	defer tun.TCPServer.Close()
	for {
		conn, err := tun.TCPServer.Accept()
		if err != nil {
			tun.sendErr(err)
			return
		}
		go func() {
			tun.TCPLocker.Lock()
			tun.TCPConns[conn.RemoteAddr().String()] = conn
			tun.TCPLocker.Unlock()
			io.Copy(&rx{root: conn, proto: proto.ProtoTCP}, conn)
		}()
	}
}

func (tun *Tunnel) UDPServerhandler() {
	var err error
	if tun.UDPServer, err = udplisterner.ListenUDP("udp", net.UDPAddrFromAddrPort(netip.AddrPortFrom(netip.IPv4Unspecified(), tun.Agent.TCPPort))); err != nil {
		tun.sendErr(err)
		return
	}
	defer tun.UDPServer.Close()
	for {
		conn, err := tun.UDPServer.Accept()
		if err != nil {
			tun.sendErr(err)
			return
		}
		go func() {
			tun.UDPLocker.Lock()
			tun.UDPConns[conn.RemoteAddr().String()] = conn
			tun.UDPLocker.Unlock()
			io.Copy(&rx{root: conn, proto: proto.ProtoUDP}, conn)
		}()
	}
}
