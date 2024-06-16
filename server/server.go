package server

import (
	"errors"
	"net"
	"net/netip"

	"sirherobrine23.org/Minecraft-Server/go-pproxit/internal/udplisterner/v2"
	"sirherobrine23.org/Minecraft-Server/go-pproxit/proto"
)

var (
	ErrAuthAgentFail error = errors.New("cannot authenticate agent") // Send unathorized client and close new accepts from current port
)

type ServerCall interface {
	// Authenticate agents
	AgentAuthentication(Token [36]byte) (TunnelInfo, error)
}

type Server struct {
	ControllConn net.Listener
	ProcessError chan error
	ControlCalls ServerCall
	Agents       map[string]*Tunnel
}

func NewController(calls ServerCall, local netip.AddrPort) (*Server, error) {
	conn, err := udplisterner.Listen("udp", local)
	if err != nil {
		return nil, err
	}
	tuns := &Server{
		ControllConn: conn,
		ControlCalls: calls,
		Agents:       make(map[string]*Tunnel),
		ProcessError: make(chan error),
	}
	go tuns.handler()
	return tuns, nil
}

func (controller *Server) handlerConn(conn net.Conn) {
	defer conn.Close() // End agent accepts
	var req *proto.Request
	var tunnelInfo TunnelInfo
	var err error
	for {
		if req, err = proto.ReaderRequest(conn); err != nil {
			break
		} else if req.AgentAuth == nil {
			proto.WriteResponse(conn, proto.Response{SendAuth: true})
			continue
		}
		if tunnelInfo, err = controller.ControlCalls.AgentAuthentication([36]byte(req.AgentAuth[:])); err != nil {
			if err == ErrAuthAgentFail {
				proto.WriteResponse(conn, proto.Response{Unauthorized: true})
				return
			}
			proto.WriteResponse(conn, proto.Response{BadRequest: true})
			continue
		}
		break
	}
	// Close current tunnel
	if tun, ok := controller.Agents[string(req.AgentAuth[:])]; ok {
		tun.Close() // Close connection
	}

	var tun = &Tunnel{
		RootConn: conn,
		TunInfo:  tunnelInfo,

		UDPClients: make(map[string]net.Conn),
		TCPClients: make(map[string]net.Conn),
	}
	controller.Agents[string(req.AgentAuth[:])] = tun
	go tun.Setup()
}

func (controller *Server) handler() {
	defer controller.ControllConn.Close()
	for {
		conn, err := controller.ControllConn.Accept()
		if err != nil {
			break
		}
		go controller.handlerConn(conn)
	}
}
