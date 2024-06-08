package udplisterner

import (
	"net"
	"net/netip"

	"sirherobrine23.org/Minecraft-Server/go-pproxit/internal/pipe"
)

type UdpListerner struct {
	MTU       uint64
	udpConn   *net.UDPConn
	clients   map[string]net.Conn
	newClient chan any
}

func (udpConn UdpListerner) Close() error {
	for addr, cli := range udpConn.clients {
		cli.Close()
		delete(udpConn.clients, addr)
	}
	close(udpConn.newClient)
	return udpConn.udpConn.Close()
}

func (udpConn UdpListerner) Addr() net.Addr {
	return udpConn.udpConn.LocalAddr()
}

func (udpConn UdpListerner) Accept() (net.Conn, error) {
	if data, ok := <-udpConn.newClient; ok {
		if err, isErr := data.(error); isErr {
			return nil, err
		}
		return data.(net.Conn), nil
	}
	return nil, net.ErrClosed
}

func (udp *UdpListerner) backgroud() {
	for {
		buffer := make([]byte, udp.MTU)
		n, from, err := udp.udpConn.ReadFromUDP(buffer)
		if err != nil {
			udp.newClient <- err // Send to accept error
			return
		} else if toListener, exist := udp.clients[from.String()]; exist {
			// Send in backgroud
			go func() {
				if _, err := toListener.Write(buffer[:n]); err != nil {
					toListener.Close()
					delete(udp.clients, from.String()) // Remove from clients
				}
			}()
			continue // Call next request
		}

		// Create new connection and send to accept
		toClinet, toListener := pipe.CreatePipe(udp.udpConn.LocalAddr(), from)
		udp.clients[from.String()] = toListener // Set listerner clients
		udp.newClient <- toClinet               // return to accept

		go func() {
			toListener.Write(buffer[:n]) // Write buffer to new pipe
			for {
				buffer := make([]byte, udp.MTU)
				n, err := toListener.Read(buffer)
				if err != nil {
					toListener.Close()
					delete(udp.clients, from.String()) // Remove from clients
					return
				}
				udp.udpConn.WriteToUDP(buffer[:n], from)
			}
		}()
	}
}

func Listen(UdpProto string, Address netip.AddrPort, MTU uint64) (net.Listener, error) {
	conn, err := net.ListenUDP(UdpProto, net.UDPAddrFromAddrPort(Address))
	if err != nil {
		return nil, err
	}
	udp := new(UdpListerner)
	udp.udpConn = conn
	udp.newClient = make(chan any)
	udp.clients = make(map[string]net.Conn)
	udp.MTU = MTU
	go udp.backgroud()
	return udp, nil
}
