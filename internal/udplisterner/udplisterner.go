package udplisterner

import (
	"io"
	"net"
	"net/netip"
)

type UdpListerner struct {
	udpConn   *net.UDPConn
	clients   map[string]net.Conn
	newClient chan any
}

func (udpConn UdpListerner) Close() error {
	for addr, cli := range udpConn.clients {
		cli.Close()
		delete(udpConn.clients, addr)
	}
	return udpConn.udpConn.Close()
}

func (udpConn UdpListerner) Addr() net.Addr {
	return udpConn.udpConn.LocalAddr()
}

func (udpConn UdpListerner) Accept() (net.Conn, error) {
	data := <- udpConn.newClient
	if err, isErr := data.(error); isErr {
		return nil, err
	}
	return data.(net.Conn), nil
}

func Listen(UdpProto string, Address netip.AddrPort) (net.Listener, error) {
	conn, err := net.ListenUDP(UdpProto, net.UDPAddrFromAddrPort(Address));
	if err != nil {
		return nil, err
	}

	udp := UdpListerner{conn, make(map[string]net.Conn), make(chan any)}
	go func (){
		for {
			buffer := make([]byte, 1024)
			n, from, err := conn.ReadFromUDP(buffer)
			if err != nil {
				udp.newClient <- err // Send to accept error
				return
			}
			buffer = buffer[:n] // Slice buffer
			if toListener, exist := udp.clients[from.String()]; exist {
				// Send in backgroud
				go func()  {
					if _, err := toListener.Write(buffer); err != nil {
						if err == io.EOF {
							toListener.Close()
							delete(udp.clients, from.String()) // Remove from clients
						}
					}
				}()
				continue // Call next request
			}

			// Create new connection and send to accept
			toClinet, toListener := createPipe(conn.LocalAddr(), from)
			udp.clients[from.String()] = toListener // Set listerner clients
			go func () {
				for {
					buffer := make([]byte, 2048)
					n, err := toListener.Read(buffer)
					if err != nil {
						if err == io.EOF {
							toListener.Close()
							delete(udp.clients, from.String()) // Remove from clients
							return
						}
						continue
					}
					conn.WriteToUDP(buffer[:n], from)
				}
			}()
			udp.newClient <- toClinet // return to accept
		}
	}()
	return udp, nil
}