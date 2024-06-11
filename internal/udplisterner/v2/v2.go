package udplisterner

import (
	"io"
	"log"
	"net"
)

type client struct {
	ClientConn            *pipe
	rdRx, wrTx            chan []byte
	rdTx, wrRx            chan int
	localDone, remoteDone chan struct{}
}

type UDPListener struct {
	conn     *net.UDPConn      // Root listener
	toAccept chan any          // Return accept connections
	clients  map[string]client // Clients
}

// Get address from UDP Listener
func (lis UDPListener) Addr() net.Addr {
	return lis.conn.LocalAddr()
}

func (lis *UDPListener) Close() error {
	for _, client := range lis.clients {
		client.localDone <- struct{}{} // Close and wait response, ignoraing errors
	}
	close(lis.toAccept) // end channel
	return lis.conn.Close()
}

func (lis UDPListener) Accept() (net.Conn, error) {
	if rec, ok := <-lis.toAccept; ok {
		if err, isErr := rec.(error); isErr {
			return nil, err
		}
		return rec.(net.Conn), nil
	}
	return nil, io.ErrClosedPipe
}

func Listen(network string, address *net.UDPAddr) (net.Listener, error) {
	var conn *net.UDPConn
	var err error
	if conn, err = net.ListenUDP(network, address); err != nil {
		return nil, err
	}
	accepts := make(chan any)
	listen := &UDPListener{conn, accepts, make(map[string]client)}
	go func() {
		var maxSize int = 1024
		for {
			log.Println("waiting request")
			buff := make([]byte, maxSize)
			n, from, err := conn.ReadFromUDPAddrPort(buff)
			if err != nil {
				break // end loop-de-loop
			}
			log.Printf("Request from: %s", from.String())
			if tun, ok := listen.clients[from.String()]; ok {
				tun.wrTx <- buff[:n]
				<-tun.rdTx // but ignore
				continue
			}
			go func() {
				rdRx := make(chan []byte)
				wrTx := make(chan []byte)
				rdTx := make(chan int)
				wrRx := make(chan int)
				localDone := make(chan struct{})
				remoteDone := make(chan struct{})
				newClient := client{
					rdRx: rdRx, rdTx: rdTx,
					wrTx: wrTx, wrRx: wrRx,
					localDone: localDone, remoteDone: remoteDone,
					ClientConn: &pipe{
						localAddr:  conn.LocalAddr(),
						remoteAddr: net.UDPAddrFromAddrPort(from),

						rdRx: wrTx, rdTx: wrRx,
						wrTx: rdRx, wrRx: rdTx,
						localDone: remoteDone, remoteDone: localDone,
						readDeadline:  makePipeDeadline(),
						writeDeadline: makePipeDeadline(),
					},
				}
				listen.clients[from.String()] = newClient // Set to clients map
				listen.toAccept <- newClient.ClientConn   // Send to accept
				newClient.wrTx <- buff[:n]
				<-newClient.rdTx // but ignore
				for {
					if data, ok := <-rdRx; ok {
						n, err := conn.WriteToUDPAddrPort(data, from)
						if err != nil {
							localDone <- struct{}{}
							<-remoteDone // wait remote
							break        // end
						}
						wrRx <- n // send write data
						continue
					}
					break
				}
			}()
		}
	}()
	return listen, nil
}
