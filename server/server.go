package server

import (
	"bytes"
	"errors"
	"io"
	"log"
	"net"
	"net/netip"
	"time"

	"sirherobrine23.org/Minecraft-Server/go-pproxit/internal/udplisterner"
	"sirherobrine23.org/Minecraft-Server/go-pproxit/proto"
)

var (
	ErrNoAgent error = errors.New("agent not found")
)

type TunnelInfo struct {
	PortListen uint16 // Port to listen Listeners
	Proto      uint8  // (proto.ProtoTCP, proto.ProtoUDP or proto.ProtoBoth)
}

type Tunnel struct {
	Token          [36]byte            // Agent Token
	Authenticated  bool                // Agent Authenticated and avaible to recive/transmiter data
	ResponseBuffer uint64              // Send Reponse size
	UDPListener    net.Listener        // Accept connections from UDP Clients
	TCPListener    net.Listener        // Accept connections from TCP Clients
	UDPClients     map[string]net.Conn // Current clients connected in UDP Socket
	TCPClients     map[string]net.Conn // Current clients connected in TCP Socket
	SendToAgent    chan proto.Response // Send data to agent
}

// Interface to server accept and reject agents sessions
type ServerCalls interface {
	AgentInfo(Token [36]byte) (TunnelInfo, error)
	AgentShutdown(Token [36]byte) error
}

// Accept any agent in ramdom port
type DefaultCall struct{}

func (DefaultCall) getFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func (DefaultCall) AgentShutdown(Token [36]byte) error { return nil }
func (d DefaultCall) AgentInfo(Token [36]byte) (TunnelInfo, error) {
	port, err := d.getFreePort()
	if err == nil {
		return TunnelInfo{
			PortListen: uint16(port),
			Proto:      proto.ProtoBoth,
		}, nil
	}
	return TunnelInfo{}, err
}

type Server struct {
	RequestBuffer uint64            // Request Buffer
	Tunnels       map[string]Tunnel // Tunnels listened
	ServerCalls   ServerCalls       // Server call to auth and more
}

// Create new server struct
//
// if Calls is nil set DefaultCall and accept any new agent in random ports and TCP+UDP Proto
func NewServer(Calls ServerCalls) Server {
	if Calls == nil {
		Calls = DefaultCall{}
	}
	return Server{
		RequestBuffer: proto.DataSize,
		ServerCalls:   Calls,
		Tunnels:       make(map[string]Tunnel),
	}
}

// Close client and send dead to agent
func (tun *Tunnel) Close() {
	if tun.TCPListener != nil {
		tun.TCPListener.Close()
	}
	if tun.UDPListener != nil {
		tun.UDPListener.Close()
	}

	for key, conn := range tun.UDPClients {
		conn.Close()                // End connection
		delete(tun.UDPClients, key) // Delete from map
	}
	for key, conn := range tun.TCPClients {
		conn.Close()                // End connection
		delete(tun.UDPClients, key) // Delete from map
	}
	close(tun.SendToAgent)
}

// Process UDP Connections from listerner
func (tun *Tunnel) UDPAccepts() {
	for {
		conn, err := tun.UDPListener.Accept()
		if err != nil {
			time.Sleep(time.Second)
			continue
		}
		clientAddr := netip.MustParseAddrPort(conn.RemoteAddr().String())
		tun.UDPClients[conn.RemoteAddr().String()] = conn

		go func() {
			for {
				buff := make([]byte, tun.ResponseBuffer)
				n, err := conn.Read(buff)
				if err != nil {
					go conn.Close()
					tun.SendToAgent <- proto.Response{
						CloseClient: &proto.Client{
							Client: clientAddr,
							Proto:  proto.ProtoUDP,
						},
					}
					break
				}
				if tun.ResponseBuffer-uint64(n) == 0 {
					tun.ResponseBuffer += 500
					res := proto.Response{}
					res.ResizeBuffer = new(uint64)
					*res.ResizeBuffer = tun.ResponseBuffer
					tun.SendToAgent <- res
					<-time.After(time.Microsecond)
				}
				tun.SendToAgent <- proto.Response{
					DataRX: &proto.ClientData{
						Size: uint64(n),
						Data: buff[:n],
						Client: proto.Client{
							Client: clientAddr,
							Proto:  proto.ProtoUDP,
						},
					},
				}
			}
		}()
	}
}

// Process TCP Connections from listerner
func (tun *Tunnel) TCPAccepts() {
	for {
		conn, err := tun.TCPListener.Accept()
		if err != nil {
			time.Sleep(time.Second)
			continue
		}
		clientAddr := netip.MustParseAddrPort(conn.RemoteAddr().String())
		tun.TCPClients[conn.RemoteAddr().String()] = conn
		go func() {
			for {
				buff := make([]byte, tun.ResponseBuffer)
				n, err := conn.Read(buff)
				if err != nil {
					go conn.Close()
					tun.SendToAgent <- proto.Response{
						CloseClient: &proto.Client{
							Client: clientAddr,
							Proto:  proto.ProtoTCP,
						},
					}
					break
				}
				if tun.ResponseBuffer-uint64(n) == 0 {
					tun.ResponseBuffer += 500
					res := proto.Response{}
					res.ResizeBuffer = new(uint64)
					*res.ResizeBuffer = tun.ResponseBuffer
					tun.SendToAgent <- res
					<-time.After(time.Microsecond)
				}
				tun.SendToAgent <- proto.Response{
					DataRX: &proto.ClientData{
						Size: uint64(n),
						Data: buff[:n],
						Client: proto.Client{
							Client: clientAddr,
							Proto:  proto.ProtoTCP,
						},
					},
				}
			}
		}()
	}
}

func (tun *Tunnel) Request(req proto.Request) {
	if client := req.ClientClose; client != nil {
		addrStr := client.Client.String()
		if cl, exit := tun.TCPClients[addrStr]; exit && client.Proto == 1 {
			cl.Close()
			delete(tun.TCPClients, addrStr)
		} else if cl, exit := tun.UDPClients[addrStr]; exit && client.Proto == 2 {
			cl.Close()
			delete(tun.TCPClients, addrStr)
		}
		return
	} else if data := req.DataTX; data != nil {
		var conn net.Conn = nil
		var exist bool
		if data.Client.Proto == proto.ProtoTCP {
			if conn, exist = tun.TCPClients[data.Client.Client.String()]; !exist {
				conn = nil
			}
		} else if data.Client.Proto == proto.ProtoUDP {
			if conn, exist = tun.UDPClients[data.Client.Client.String()]; !exist {
				conn = nil
			}
		}
		if conn == nil {
			tun.SendToAgent <- proto.Response{CloseClient: client}
			return
		}

		if _, err := conn.Write(data.Data); err != nil {
			conn.Close()
			delete(tun.TCPClients, data.Client.Client.String())
			tun.SendToAgent <- proto.Response{CloseClient: client}
		}
		return
	}
}

// Listener controller and controller listener
func (server *Server) Listen(ControllerPort uint16) (err error) {
	var conn *net.UDPConn
	if conn, err = net.ListenUDP("udp", net.UDPAddrFromAddrPort(netip.AddrPortFrom(netip.IPv4Unspecified(), ControllerPort))); err != nil {
		return
	}

	for {
		var err error
		var req proto.Request
		var res proto.Response
		var readSize int
		var addr netip.AddrPort
		log.Println("waiting to request")
		buffer := make([]byte, proto.PacketSize+server.RequestBuffer)
		if readSize, addr, err = conn.ReadFromUDPAddrPort(buffer); err != nil {
			if err == io.EOF {
				break // End controller
			}
			continue
		}

		if err := req.Reader(bytes.NewBuffer(buffer[:readSize])); err != nil {
			res.BadRequest = true
			if buffer, err = res.Wbytes(); err != nil {
				continue // not send bad request to agent
			}
			conn.WriteToUDPAddrPort(buffer, addr) // Send bad request to agent
			continue                              // Continue parsing new requests
		}

		if ping := req.Ping; ping != nil {
			res.Pong = new(time.Time)
			*res.Pong = time.Now()
			data, _ := res.Wbytes()
			conn.WriteToUDPAddrPort(data, addr)
			continue
		}

		// Process request if tunnel is authenticated
		if tun, exist := server.Tunnels[addr.String()]; exist && tun.Authenticated {
			go tun.Request(req) // process request to tunnel
			continue            // Call next message
		}

		// Create tunnel
		if _, exist := server.Tunnels[addr.String()]; !exist {
			// Create new tunnel agent
			server.Tunnels[addr.String()] = Tunnel{
				Token:         [36]byte{},
				Authenticated: false,
				UDPClients:    make(map[string]net.Conn),
				TCPClients:    make(map[string]net.Conn),
				SendToAgent:   make(chan proto.Response),
			}

			go func() {
				tun := server.Tunnels[addr.String()]
				for {
					if res, ok := <-tun.SendToAgent; ok {
						data, err := res.Wbytes()
						if err != nil {
							continue
						}
						go conn.WriteToUDPAddrPort(data, addr) // send data to agent
						continue
					}
					break
				}
			}()
		}

		if !server.Tunnels[addr.String()].Authenticated && req.AgentAuth == nil {
			res.SendAuth = true
			data, _ := res.Wbytes()
			conn.WriteToUDPAddrPort(data, addr)
			continue
		}
		info, err := server.ServerCalls.AgentInfo([36]byte(req.AgentAuth[:]))
		if err != nil {
			if err == ErrNoAgent {
				// Client not found
				res.BadRequest = true
			} else {
				// Cannot process request resend
				res.SendAuth = true
			}
			data, _ := res.Wbytes()
			conn.WriteToUDPAddrPort(data, addr)
			continue
		}

		tun := server.Tunnels[addr.String()]
		if info.Proto == 3 || info.Proto == 1 {
			tun.TCPListener, err = net.ListenTCP("tcp", net.TCPAddrFromAddrPort(netip.AddrPortFrom(netip.IPv4Unspecified(), info.PortListen)))
			if err != nil {
				res.BadRequest = true
				data, _ := res.Wbytes()
				conn.WriteToUDPAddrPort(data, addr)
				continue
			}
			go tun.TCPAccepts() // Make accepts new requests
		}
		if info.Proto == 3 || info.Proto == 2 {
			tun.UDPListener, err = udplisterner.Listen("udp", netip.AddrPortFrom(netip.IPv4Unspecified(), info.PortListen), func() uint64 {return server.RequestBuffer})
			if err != nil {
				if tun.TCPListener != nil {
					tun.TCPListener.Close()
				}
				res.BadRequest = true
				data, _ := res.Wbytes()
				conn.WriteToUDPAddrPort(data, addr)
				continue
			}
			go tun.UDPAccepts() // Make accepts new requests
		}
		tun.Authenticated = true
		server.Tunnels[addr.String()] = tun

		res.AgentInfo = new(proto.AgentInfo)
		res.AgentInfo.Protocol = info.Proto
		res.AgentInfo.LitenerPort = info.PortListen
		res.AgentInfo.AddrPort = addr

		data, _ := res.Wbytes()
		conn.WriteToUDPAddrPort(data, addr)
		continue
	}
	return
}
