package server

import (
	"io"
	"net"
	"net/netip"
	"time"

	"sirherobrine23.org/Minecraft-Server/go-pproxit/internal/udplisterner/v2"
	"sirherobrine23.org/Minecraft-Server/go-pproxit/proto"
)

type TunnelCall interface {
	BlockedAddr(AddrPort netip.Addr) bool                    // Ignore request from this address
	AgentPing(agent, server time.Time)                       // Register ping to Agent
	AgentShutdown(onTime time.Time)                          // Agend end connection
	RegisterRX(client netip.AddrPort, Size int, Proto uint8) // Register Recived data from client
	RegisterTX(client netip.AddrPort, Size int, Proto uint8) // Register Transmitted data from client
}

type TunnelInfo struct {
	Proto            uint8      // Protocol listen tunnel, use proto.ProtoTCP, proto.ProtoUDP or proto.ProtoBoth
	UDPPort, TCPPort uint16     // Port to Listen UDP and TCP listeners
	Callbacks        TunnelCall // Tunnel Callbacks
}

type Tunnel struct {
	RootConn net.Conn   // Current client connection
	TunInfo  TunnelInfo // Tunnel info

	UDPClients map[string]net.Conn // Current clients connected
	TCPClients map[string]net.Conn // Current clients connected
}

func (tun *Tunnel) Close() error {
	// Stop TCP Clients
	for k := range tun.TCPClients {
		tun.TCPClients[k].Close()
		delete(tun.TCPClients, k)
	}

	// Stop UDP Clients
	for k := range tun.UDPClients {
		tun.UDPClients[k].Close()
		delete(tun.UDPClients, k)
	}

	tun.RootConn.Close()                            // End root conenction
	tun.TunInfo.Callbacks.AgentShutdown(time.Now()) // Register shutdown
	return nil
}

func (tun *Tunnel) send(res proto.Response) error {
	return proto.WriteResponse(tun.RootConn, res)
}

type toWr struct {
	Proto uint8
	To    netip.AddrPort
	tun   *Tunnel
}

func (t toWr) Write(w []byte) (int, error) {
	go t.tun.TunInfo.Callbacks.RegisterRX(t.To, len(w), t.Proto)
	err := t.tun.send(proto.Response{
		DataRX: &proto.ClientData{
			Client: proto.Client{
				Proto:  t.Proto,
				Client: t.To,
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

func (tun *Tunnel) GetTargetWrite(Proto uint8, To netip.AddrPort) io.Writer {
	return &toWr{Proto: Proto, To: To, tun: tun}
}

// Setup connections and maneger connections from agent
func (tun *Tunnel) Setup() {
	defer tun.Close()
	tun.send(proto.Response{
		AgentInfo: &proto.AgentInfo{
			Protocol: tun.TunInfo.Proto,
			AddrPort: netip.MustParseAddrPort(tun.RootConn.RemoteAddr().String()),
			UDPPort:  tun.TunInfo.UDPPort,
			TCPPort:  tun.TunInfo.TCPPort,
		},
	})

	if proto.ProtoBoth == tun.TunInfo.Proto || proto.ProtoTCP == tun.TunInfo.Proto {
		go tun.TCP() // Setup TCP Listerner
	}
	if proto.ProtoBoth == tun.TunInfo.Proto || proto.ProtoUDP == tun.TunInfo.Proto {
		go tun.UDP() // Setup UDP Listerner
	}

	for {
		req, err := proto.ReaderRequest(tun.RootConn)
		if err != nil {
			break
		} else if ping := *req.Ping; req.Ping != nil {
			var now = time.Now()
			tun.send(proto.Response{Pong: &now})
			go tun.TunInfo.Callbacks.AgentPing(ping, now) // backgroud process
		} else if clClose := *req.ClientClose; req.ClientClose != nil {
			if clClose.Proto == proto.ProtoTCP {
				if cl, ok := tun.TCPClients[clClose.Client.String()]; ok {
					cl.Close()
				}
			} else if clClose.Proto == proto.ProtoUDP {
				if cl, ok := tun.UDPClients[clClose.Client.String()]; ok {
					cl.Close()
				}
			}
		} else if data := *req.DataTX; req.DataTX != nil {
			go tun.TunInfo.Callbacks.RegisterTX(data.Client.Client, int(data.Size), data.Client.Proto)
			if data.Client.Proto == proto.ProtoTCP {
				if cl, ok := tun.TCPClients[data.Client.Client.String()]; ok {
					go cl.Write(data.Data) // Process in backgroud
				}
			} else if data.Client.Proto == proto.ProtoUDP {
				if cl, ok := tun.UDPClients[data.Client.Client.String()]; ok {
					go cl.Write(data.Data) // Process in backgroud
				}
			}
		}
	}
}

// Listen TCP
func (tun *Tunnel) TCP() {
	dial, err := net.ListenTCP("tcp", net.TCPAddrFromAddrPort(netip.AddrPortFrom(netip.IPv4Unspecified(), tun.TunInfo.TCPPort)))
	if err != nil {
		tun.send(proto.Response{BadRequest: true}) // TODO: require new option to shutdown agent
		tun.Close()                                // End process listen
		return
	}
	for {
		conn, err := dial.AcceptTCP()
		if err != nil {
			panic(err) // TODO: fix accepts in future
		}
		remote := netip.MustParseAddrPort(conn.RemoteAddr().String())
		if tun.TunInfo.Callbacks.BlockedAddr(remote.Addr()) {
			conn.Close() // Close connection
			continue
		}
		tun.TCPClients[remote.String()] = conn
		go io.Copy(tun.GetTargetWrite(proto.ProtoTCP, remote), conn)
	}
}

// Listen UDP
func (tun *Tunnel) UDP() {
	dial, err := udplisterner.Listen("udp", netip.AddrPortFrom(netip.IPv4Unspecified(), tun.TunInfo.UDPPort))
	if err != nil {
		tun.send(proto.Response{BadRequest: true}) // TODO: require new option to shutdown agent
		tun.Close()                                // End process listen
		return
	}
	for {
		conn, err := dial.Accept()
		if err != nil {
			panic(err) // TODO: fix accepts in future
		}
		remote := netip.MustParseAddrPort(conn.RemoteAddr().String())
		if tun.TunInfo.Callbacks.BlockedAddr(remote.Addr()) {
			conn.Close() // Close connection
			continue
		}
		tun.UDPClients[remote.String()] = conn
		go io.Copy(tun.GetTargetWrite(proto.ProtoUDP, remote), conn)
	}
}
