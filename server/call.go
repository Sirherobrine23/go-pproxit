package server

import (
	"net"

	"sirherobrine23.org/Minecraft-Server/go-pproxit/proto"
)

func getFreePort() (int, error) {
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

// Accept any agent in ramdom port
type DefaultCall struct{}

func (DefaultCall) AgentShutdown(Token [36]byte) error { return nil }
func (d DefaultCall) AgentInfo(Token [36]byte) (TunnelInfo, error) {
	port, err := getFreePort()
	if err == nil {
		return TunnelInfo{
			PortListen: uint16(port),
			Proto:      proto.ProtoBoth,
		}, nil
	}
	return TunnelInfo{}, err
}
