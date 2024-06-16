/*
Lidar com pacotes de varios tamanhos
*/
package udplisterner

import (
	"bytes"
	"log"
	"net"
	"net/netip"
	"os"
	"sync"
)

type Udplisterner struct {
	loger          *log.Logger
	conn           *net.UDPConn // root listen connection
	newClientErr   chan error
	newClient      chan net.Conn
	currentClients map[string]*PipeConn
	locker         sync.RWMutex
}

func Listen(network string, laddr netip.AddrPort) (net.Listener, error) {
	return Listener(network, net.UDPAddrFromAddrPort(laddr))
}

func Listener(network string, laddr *net.UDPAddr) (net.Listener, error) {
	conn, err := net.ListenUDP(network, laddr)
	if err != nil {
		return nil, err
	}
	listen := &Udplisterner{
		loger:          log.New(os.Stderr, conn.LocalAddr().String()+"  ", log.Ltime),
		conn:           conn,
		newClientErr:   make(chan error),
		newClient:      make(chan net.Conn),
		currentClients: make(map[string]*PipeConn),
	}
	go listen.handle()
	return listen, nil
}

func (list *Udplisterner) handle() {
	defer list.Close()
	var readBuff int = 1024 * 8
	for {
		buff := make([]byte, readBuff)
		list.loger.Printf("Reading from %d bytes\n", readBuff)
		n, from, err := list.conn.ReadFromUDPAddrPort(buff)
		if err != nil {
			if opt, isOpt := err.(*net.OpError); isOpt {
				if opt.Temporary() {
					continue
				}
			}
			list.loger.Printf("closing readers, err: %s", err.Error())
			list.newClientErr <- err
			break
		} else if n == readBuff {
			readBuff += 1024 // Grow buffer size
			list.loger.Printf("Growing from %d to %d\n", readBuff-1024, readBuff)
		}

		list.locker.RLock() // Read locker
		client, ok := list.currentClients[from.String()]
		list.locker.RUnlock() // Unlocker

		if !ok {
			list.loger.Printf("New client %s with %d bytes to storage, waiting locker\n", from.String(), n)
			list.loger.Println(buff[:n])

			list.locker.Lock() // Locker map
			list.loger.Printf("locked")

			list.currentClients[from.String()] = &PipeConn{
				root:       list.conn,
				to:         from,
				localAddr:  from,
				closedChan: make(chan struct{}),
				closed:     false,
				buff:       bytes.NewBuffer(make([]byte, 0)),
			}
			client = list.currentClients[from.String()]

			list.locker.Unlock() // Unlocker map
			list.loger.Printf("unlocked")

			go func() {
				list.newClient <- client
				<-client.closedChan
				list.locker.Lock()
				delete(list.currentClients, from.String())
				list.locker.Unlock()
				list.loger.Printf("Closing client %s\n", from.String())
			}()
		}

		list.loger.Printf("Caching %d bytes to %s\n", n, from.String())
		client.buff.Write(buff[:n])
	}
}

func (c *Udplisterner) Addr() net.Addr { return c.conn.LocalAddr() }
func (c *Udplisterner) Close() error {
	c.locker.Lock()
	defer c.locker.Unlock()
	for key := range c.currentClients {
		delete(c.currentClients, key)
	}
	c.conn.Close()
	return nil
}
func (c *Udplisterner) Accept() (net.Conn, error) {
	select {
	case client := <-c.newClient:
		return client, nil
	case err := <-c.newClientErr:
		return nil, err
	}
}
