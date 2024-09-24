package gopproxit

import (
	"maps"
	"net"
	"net/netip"
	"reflect"
	"slices"

	"sirherobrine23.com.br/Minecraft-Server/go-pproxit/proto"
)

type Tunnel struct {
	Controller net.Conn         // Tunnel client connection
	Agent      *proto.AgentInfo // Agent info
	TunErr     chan error       // Return errors
	Done       chan struct{}    // Closed connection

	TCPServer *net.TCPListener
	UDPServer *net.UDPConn
	TCPConns  map[string]net.Conn
	UDPConns  map[string]net.Conn
}

func NewTun(conn net.Conn, Agent *proto.AgentInfo) *Tunnel {
	var tun Tunnel
	tun.Agent = Agent
	tun.Controller = conn

	// Listen clients
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

func (tun *Tunnel) Handler() {}

func (tun *Tunnel) TCPServerhandler() {
	var err error
	if tun.TCPServer, err = net.ListenTCP("tcp", net.TCPAddrFromAddrPort(netip.AddrPortFrom(netip.IPv4Unspecified(), tun.Agent.TCPPort))); err != nil {
		tun.sendErr(err)
		return
	}
}

func (tun *Tunnel) UDPServerhandler() {}
