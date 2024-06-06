package server

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"

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
	Token         [36]byte            // Agent Token
	Authenticated bool                // Agent Authenticated and avaible to recive/transmiter data
	UDPListener   net.Listener        // Accept connections from UDP Clients
	TCPListener   net.Listener        // Accept connections from TCP Clients
	UDPClients    map[string]net.Conn // Current clients connected in UDP Socket
	TCPClients    map[string]net.Conn // Current clients connected in TCP Socket
	SendToAgent   chan proto.Response // Send data to agent
}

// Interface to server accept and reject agents sessions
type ServerCalls interface {
	AgentInfo(Token [36]byte) (TunnelInfo, error)
}

// Accept any agent in ramdom port
type DefaultCall struct {}
func (DefaultCall) AgentInfo(Token [36]byte) (TunnelInfo, error) {
	return TunnelInfo{
		PortListen: 0,
		Proto: proto.ProtoBoth,
	}, nil
}

type Server struct {
	Tunnels     map[string]Tunnel // Tunnels listened
	ServerCalls ServerCalls       // Server call to auth and more
}

// Create new server struct
//
// if Calls is nil set DefaultCall and accept any new agent in random ports and TCP+UDP Proto
func NewServer(Calls ServerCalls) Server {
	if Calls == nil {
		Calls = DefaultCall{}
	}
	return Server{
		Tunnels:     make(map[string]Tunnel),
		ServerCalls: Calls,
	}
}

// Close client and send dead to agent
func (tun *Tunnel) Close() {
	close(tun.SendToAgent)
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
}

// Process UDP Connections from listerner
func (tun *Tunnel) UDPAccepts() {
	for {
		conn, _ := tun.UDPListener.Accept()
		tun.UDPClients[conn.RemoteAddr().String()] = conn
		go func() {
			for {
				var res proto.Response
				buff := make([]byte, 1024)
				n, err := conn.Read(buff)
				if err != nil {
					if err == io.EOF {
						break
					}
					continue
				}
				res.DataRX = new(proto.ClientData)
				res.DataRX.Client.Client = netip.MustParseAddrPort(conn.RemoteAddr().String())
				res.DataRX.Client.Proto = proto.ProtoUDP // UDP Proto
				res.DataRX.Size = uint64(n)
				copy(res.DataRX.Data, buff[:n])
				tun.SendToAgent <- res
			}
		}()
	}
}

// Process TCP Connections from listerner
func (tun *Tunnel) TCPAccepts() {
	for {
		conn, err := tun.TCPListener.Accept()
		if err != nil {
			continue
		}
		tun.TCPClients[conn.RemoteAddr().String()] = conn
		go func() {
			for {
				var res proto.Response
				buff := make([]byte, 1024)
				n, err := conn.Read(buff)
				if err != nil {
					if err == io.EOF {
						break
					}
					continue
				}
				res.DataRX = new(proto.ClientData)
				res.DataRX.Client.Client = netip.MustParseAddrPort(conn.RemoteAddr().String())
				res.DataRX.Client.Proto = proto.ProtoUDP // UDP Proto
				res.DataRX.Size = uint64(n)
				copy(res.DataRX.Data, buff[:n])
				tun.SendToAgent <- res
			}
		}()
	}
}

func (tun *Tunnel) Request(req proto.Request) {
	var res proto.Response
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
		if conn, exist := tun.TCPClients[data.Client.Client.String()]; data.Client.Proto == 1 && exist {
			if _, err := conn.Write(data.Data); err == io.EOF {
				conn.Close()
				delete(tun.TCPClients, data.Client.Client.String())
				res.CloseClient = &proto.Client{Client: data.Client.Client, Proto: 1}
				tun.SendToAgent <- res
			}
		} else if conn, exist := tun.UDPClients[data.Client.Client.String()]; data.Client.Proto == 2 && exist {
			if _, err := conn.Write(data.Data); err == io.EOF {
				conn.Close()
				delete(tun.TCPClients, data.Client.Client.String())
				res.CloseClient = &proto.Client{Client: data.Client.Client, Proto: 2}
				tun.SendToAgent <- res
			}
		}
		return
	}
}

// Listener controller and controller listener
func (server *Server) Listen(ControllerPort uint16) (conn *net.UDPConn, err error) {
	if conn, err = net.ListenUDP("udp", net.UDPAddrFromAddrPort(netip.AddrPortFrom(netip.IPv4Unspecified(), ControllerPort))); err != nil {
		return
	}

	go func() {
		for {
			var err error
			var req proto.Request
			var res proto.Response
			var readSize int
			var addr netip.AddrPort
			buffer := make([]byte, 2048)
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

			var exist bool
			var tun Tunnel
			// Process request if tunnel is authenticated
			if tun, exist = server.Tunnels[addr.String()]; exist && tun.Authenticated {
				if req.AgentBye {
					tun.Close()                           // wait close clients
					delete(server.Tunnels, addr.String()) // Delete tunnel from  tunnels list
					continue
				}
				go tun.Request(req) // process request to tunnel
				continue            // Call next message
			} else if exist {
				if !tun.Authenticated && req.AgentAuth == nil {
					res.SendAuth = true
					data, _ := res.Wbytes()
					conn.WriteToUDPAddrPort(data, addr)
					continue
				}
				info, err := server.ServerCalls.AgentInfo(*req.AgentAuth)
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

				if info.Proto == 3 || info.Proto == 1 {
					tun.TCPListener, err = net.Listen("tcp", fmt.Sprintf(":%d", info.PortListen))
					if err != nil {
						res.BadRequest = true
						data, _ := res.Wbytes()
						conn.WriteToUDPAddrPort(data, addr)
						continue
					}
					go tun.TCPAccepts() // Make accepts new requests
				}
				if info.Proto == 3 || info.Proto == 2 {
					tun.UDPListener, err = udplisterner.Listen("udp", fmt.Sprintf(":%d", info.PortListen))
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
				continue
			}

			// Create new tunnel agent
			tun = Tunnel{
				Token:         [36]byte{},
				Authenticated: false,
				UDPClients:    make(map[string]net.Conn),
				TCPClients:    make(map[string]net.Conn),
				SendToAgent:   make(chan proto.Response),
			}

			go func() {
				for {
					res, closed := <-tun.SendToAgent
					if closed {
						return
					}
					data, err := res.Wbytes()
					if err != nil {
						continue
					}
					go conn.WriteToUDPAddrPort(data, addr) // send data to agent
				}
			}()
		}
	}()

	return // Return controller to caller
}
