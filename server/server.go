package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
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
type DefaultCall struct{}

func (DefaultCall) AgentInfo(Token [36]byte) (TunnelInfo, error) {
	var fun = func () (port uint16, err error) {
		var a *net.TCPAddr
		if a, err = net.ResolveTCPAddr("tcp", "localhost:0"); err == nil {
			var l *net.TCPListener
			if l, err = net.ListenTCP("tcp", a); err == nil {
				defer l.Close()
				return uint16(l.Addr().(*net.TCPAddr).Port), nil
			}
		}
		return
	}
	port, err := fun()
	if err != nil {
		return TunnelInfo{}, err
	}
	return TunnelInfo{
		PortListen: port,
		Proto:      proto.ProtoBoth,
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
		clientAddr := netip.MustParseAddrPort(conn.RemoteAddr().String())
		tun.TCPClients[conn.RemoteAddr().String()] = conn
		tun.SendToAgent <- proto.Response{
			NewClient: &proto.Client{
				Client: clientAddr,
				Proto: proto.ProtoUDP,
			},
		}

		go func() {
			for {
				buff := make([]byte, 1024)
				n, err := conn.Read(buff)
				if err != nil {
					if err == io.EOF {
						tun.SendToAgent <- proto.Response{
							CloseClient: &proto.Client{
								Client: clientAddr,
								Proto: proto.ProtoUDP,
							},
						}
						break
					}
					continue
				}
				tun.SendToAgent <- proto.Response{
					DataRX: &proto.ClientData{
						Size: uint64(n),
						Data: buff[:n],
						Client: proto.Client{
							Client: clientAddr,
							Proto: proto.ProtoUDP,
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
			continue
		}
		clientAddr := netip.MustParseAddrPort(conn.RemoteAddr().String())
		tun.TCPClients[conn.RemoteAddr().String()] = conn
		tun.SendToAgent <- proto.Response{
			NewClient: &proto.Client{
				Client: clientAddr,
				Proto: proto.ProtoTCP,
			},
		}

		go func() {
			for {
				buff := make([]byte, 1024)
				n, err := conn.Read(buff)
				if err != nil {
					if err == io.EOF {
						tun.SendToAgent <- proto.Response{
							CloseClient: &proto.Client{
								Client: clientAddr,
								Proto: proto.ProtoTCP,
							},
						}
						break
					}
					continue
				}
				tun.SendToAgent <- proto.Response{
					DataRX: &proto.ClientData{
						Size: uint64(n),
						Data: buff[:n],
						Client: proto.Client{
							Client: clientAddr,
							Proto: proto.ProtoTCP,
						},
					},
				}
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
		buffer := make([]byte, 2048)
		if readSize, addr, err = conn.ReadFromUDPAddrPort(buffer); err != nil {
			if err == io.EOF {
				break // End controller
			}
			continue
		}

		debuglog.Printf("Controller request from %s data %+v\n", addr.String(), buffer[:readSize])
		if err := req.Reader(bytes.NewBuffer(buffer[:readSize])); err != nil {
			res.BadRequest = true
			if buffer, err = res.Wbytes(); err != nil {
				continue // not send bad request to agent
			}
			conn.WriteToUDPAddrPort(buffer, addr) // Send bad request to agent
			continue                              // Continue parsing new requests
		}

		d, _ := json.Marshal(res)
		debuglog.Println(string(d))
		d, _ = json.Marshal(req)
		debuglog.Println(string(d))

		if ping := req.Ping; ping != nil {
			res.Pong = new(time.Time)
			*res.Pong = time.Now()
			data, _ := res.Wbytes()
			conn.WriteToUDPAddrPort(data, addr)
			continue
		}

		// Process request if tunnel is authenticated
		if tun, exist := server.Tunnels[addr.String()]; exist && tun.Authenticated {
			if req.AgentBye {
				tun.Close()                           // wait close clients
				delete(server.Tunnels, addr.String()) // Delete tunnel from  tunnels list
				continue
			}
			debuglog.Printf("Request from %s redirecting to tunnel", addr.String())
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
				for {
					if res, ok := <-server.Tunnels[addr.String()].SendToAgent; ok {
						d,_:=json.Marshal(res)
						debuglog.Println(string(d))

						data, err := res.Wbytes()
						if err != nil {
							continue
						}
						go conn.WriteToUDPAddrPort(data, addr) // send data to agent
					}
				}
			}()
		}

		debuglog.Printf("Request from %s checking to auth", addr.String())
		if !server.Tunnels[addr.String()].Authenticated && req.AgentAuth == nil {
			debuglog.Printf("Request from %s rejected", addr.String())
			res.SendAuth = true
			data, _ := res.Wbytes()
			conn.WriteToUDPAddrPort(data, addr)
			continue
		}
		debuglog.Printf("Checking agent from %s is valid\n", addr.String())
		info, err := server.ServerCalls.AgentInfo([36]byte(req.AgentAuth[:]))
		if err != nil {
			debuglog.Println(err.Error())
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
			tun.UDPListener, err = udplisterner.Listen("udp", netip.AddrPortFrom(netip.IPv4Unspecified(), info.PortListen))
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
