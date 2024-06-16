/*
Lidar com pacotes de varios tamanhos
*/
package udplisterner

import (
	"bufio"
	"bytes"
	"log"
	"net"
	"net/netip"
	"sync"
)

type Udplisterner struct {
	Log            *log.Logger  // Write request to Debug
	conn           *net.UDPConn // root listen connection
	newClientErr   chan error
	newClient      chan net.Conn
	currentClients map[string]*bufio.ReadWriter
	locker         sync.RWMutex
}

func (list *Udplisterner) handle() {
	defer list.conn.Close()
	localAddr := list.conn.LocalAddr().String()
	var readBuff int = 1024 * 8
	for {
		buff := make([]byte, readBuff)
		list.Log.Printf("%s: Reading from %d bytes\n", localAddr, readBuff)
		n, from, err := list.conn.ReadFromUDPAddrPort(buff)
		if err != nil {
			list.Log.Printf("closing readers, err: %s", err.Error())
			list.newClientErr <- err
			break
		} else if n == readBuff {
			readBuff += 1024 // Grow buffer size
			list.Log.Printf("%s: Growing from %d to %d\n", localAddr, readBuff-1024, readBuff)
		}

		list.locker.RLock() // Read locker
		if client, ok := list.currentClients[from.String()]; ok {
			list.Log.Printf("%s: Caching %d bytes to %s\n", localAddr, n, from.String())
			client.Write(buff[:n])
			list.locker.RUnlock() // Unlocker
			continue
		}
		list.Log.Printf("%s: New client %s\n", localAddr, from.String())
		list.locker.RUnlock() // Unlocker
		list.locker.Lock()    // Locker map
		bufferd := bufio.NewReadWriter(bufio.NewReader(bytes.NewReader(buff[:n])), &bufio.Writer{})
		clientConn := NewConn(list.conn, netip.MustParseAddrPort(list.conn.LocalAddr().String()), from, bufferd)
		list.currentClients[from.String()] = bufferd
		list.locker.Unlock() // Unlocker map
		go func() {
			list.newClient <- clientConn
			<-clientConn.(*PipeConn).closedChan
		}()
	}
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
		conn: conn,
	}
	go listen.handle()
	return listen, nil
}

func (c *Udplisterner) Addr() net.Addr { return c.conn.LocalAddr() }
func (c *Udplisterner) Close() error {
	c.conn.Close()
	c.locker.Lock()
	defer c.locker.Unlock()
	for key := range c.currentClients {
		delete(c.currentClients, key)
	}
	return nil
}
func (c *Udplisterner) Accept() (net.Conn, error) {
	select {
	case client := <-c.newClient:
		return client, nil
	case err := <-c.newClientErr:
		return nil, err
	default:
		return nil, net.ErrClosed
	}
}
