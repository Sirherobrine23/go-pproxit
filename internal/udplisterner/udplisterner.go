package udplisterner

import (
	"io"
	"net"
	"net/netip"
	"sync"

	"sirherobrine23.org/Minecraft-Server/go-pproxit/internal/pipe"
)

type clientInfo struct {
	WriteSize int      // Size to Write
	Conn      net.Conn // Client Pipe
}

type UdpListerner struct {
	readSize   int          // UDPConn size to reader
	udpConn    *net.UDPConn // UDPConn root
	clientInfo *sync.Map    // Storage *clientInfo
	newClient  chan any     // Accept connection channel or error
}

func ListenUDPAddr(network string, address *net.UDPAddr) (UdpListen *UdpListerner, err error) {
	UdpListen = new(UdpListerner)
	if UdpListen.udpConn, err = net.ListenUDP(network, address); err != nil {
		return nil, err
	}
	UdpListen.readSize = 1024 // Initial buffer reader
	UdpListen.newClient = make(chan any)
	UdpListen.clientInfo = new(sync.Map)
	go UdpListen.backgroud() // Recive new requests
	return UdpListen, nil
}

func ListenAddrPort(network string, address netip.AddrPort) (*UdpListerner, error) {
	return ListenUDPAddr(network, net.UDPAddrFromAddrPort(address))
}

func Listen(network, address string) (*UdpListerner, error) {
	local, err := net.ResolveUDPAddr(network, address)
	if err != nil {
		return nil, err
	}
	return ListenUDPAddr(network, local)
}

// Close clients and close client channel
func (udp *UdpListerner) Close() error {
	close(udp.newClient) // Close channel to new accepts
	var toDelete map[string]*clientInfo
	udp.clientInfo.Range(func(key, value any) bool {
		toDelete[key.(string)] = value.(*clientInfo)
		return true
	})
	for key, info := range toDelete {
		info.Conn.Close()
		udp.clientInfo.Delete(key)
	}
	return nil
}

func (udp *UdpListerner) CloseClient(clientAddrPort string) {
	client, ok := udp.clientInfo.LoadAndDelete(clientAddrPort)
	if !ok {
		return
	}
	agent := client.(clientInfo)
	agent.Conn.Close()
}

func (udp UdpListerner) Addr() net.Addr {
	if udp.udpConn == nil {
		return &net.UDPAddr{}
	}
	return udp.udpConn.LocalAddr()
}

func (udp UdpListerner) Accept() (net.Conn, error) {
	if conn, ok := <-udp.newClient; ok {
		if err, isErr := conn.(error); isErr {
			return nil, err
		}
		return conn.(*clientInfo).Conn, nil
	}
	return nil, net.ErrClosed
}

func (udp *UdpListerner) backgroud() {
	for {
		readBuffer := make([]byte, udp.readSize) // Make reader size
		n, from, err := udp.udpConn.ReadFromUDP(readBuffer)
		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				udp.Close() // Close clients
				break
			}
			continue
		} else if n-udp.readSize == 0 {
			udp.readSize += 500 // Add 500 Bytes to reader
		}

		// Check if exists current connection
		if client, ok := udp.clientInfo.Load(from.String()); ok {
			toListener := client.(*clientInfo)
			toListener.Conn.Write(readBuffer[:n]) // n size from Buffer to client
			continue                              // Contine loop
		}
		go func() {
			// Create new client
			newClient := new(clientInfo)
			newClient.WriteSize = n // Same Size from reader buffer
			var agentPipe net.Conn
			newClient.Conn, agentPipe = pipe.CreatePipe(udp.udpConn.LocalAddr(), from)

			udp.newClient <- newClient // Send to accept
			udp.clientInfo.Store(from.String(), newClient)
			go agentPipe.Write(readBuffer[:n]) // n size from Buffer to client
			go func() {
				for {
					client, ok := udp.clientInfo.Load(from.String())
					if !ok {
						udp.clientInfo.Delete(from.String())
						agentPipe.Close()
						break // bye-bye
					}
					newClient := client.(*clientInfo)
					writeBuffer := make([]byte, newClient.WriteSize)
					n, err := agentPipe.Read(writeBuffer)
					if err != nil {
						udp.clientInfo.Delete(from.String())
						agentPipe.Close()
						break
					}
					go udp.udpConn.WriteToUDP(writeBuffer[:n], from)
				}
			}()
		}()
	}
}
