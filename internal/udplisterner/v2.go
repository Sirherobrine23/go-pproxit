package udplisterner

import (
	"bufio"
	"bytes"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"sirherobrine23.com.br/Minecraft-Server/go-pproxit/internal/pipe"
)

type writeRoot struct {
	conn *net.UDPConn
	to   *net.UDPAddr
}

func (wr *writeRoot) Write(w []byte) (int, error) {
	return wr.conn.WriteToUDP(w, wr.to)
}

type client struct {
	fromAgent, toClient net.Conn
	bufferCache         *bytes.Buffer
	bufioCache          *bufio.Reader
	LastPing            time.Time
}

type UDPServer struct {
	rootUdp   *net.UDPConn       // Root connection to read
	peers     map[string]*client // peers connected
	newPeer   chan net.Conn
	peerError chan error

	closed bool
	rw     sync.RWMutex
}

func ListenUDP(network string, laddr *net.UDPAddr) (*UDPServer, error) {
	conn, err := net.ListenUDP(network, laddr)
	if err != nil {
		return nil, err
	}
	var root = &UDPServer{
		rootUdp:   conn,
		peers:     make(map[string]*client),
		newPeer:   make(chan net.Conn),
		peerError: make(chan error),
	}
	go root.handler()
	return root, nil
}

func Listen(Network, address string) (net.Listener, error) {
	ip, err := net.ResolveUDPAddr(Network, address)
	if err != nil {
		return nil, err
	}
	return ListenUDP(Network, ip)
}

// Local address
func (udpListen *UDPServer) Addr() net.Addr {
	return udpListen.rootUdp.LocalAddr()
}

// Close peers and root connection
func (udpListen *UDPServer) Close() error {
	if udpListen.closed {
		return io.ErrClosedPipe
	}
	udpListen.closed = true
	for peerIndex := range udpListen.peers {
		log.Printf("closing %s", peerIndex)
		udpListen.peers[peerIndex].fromAgent.Close() // Close
		log.Printf("clearing %s", peerIndex)
		udpListen.peers[peerIndex].bufferCache.Reset()
	}
	log.Printf("closing udp root")
	return udpListen.rootUdp.Close()
}

// Accept new client
func (udpListen *UDPServer) Accept() (peer net.Conn, err error) {
	select {
	case peer = <-udpListen.newPeer:
		return
	case err = <-udpListen.peerError:
		return
	}
}

func (udpListen *UDPServer) handler() {
	buff := make([]byte, 32*1024)
	for {
		n, from, err := udpListen.rootUdp.ReadFromUDP(buff)
		if err != nil {
			return
		}

		udpListen.rw.Lock()
		if nt, exist := udpListen.peers[from.String()]; exist {
			if (time.Now().UnixMicro() - nt.LastPing.UnixMicro()) > 100_000_000 {
				delete(udpListen.peers, from.String())
			}
		}

		if _, exist := udpListen.peers[from.String()]; !exist {
			c := new(client)
			c.LastPing = time.Now()
			c.bufferCache = new(bytes.Buffer)
			c.bufioCache = bufio.NewReader(c.bufferCache)

			c.fromAgent, c.toClient = pipe.CreatePipe(from.AddrPort(), from.AddrPort())

			udpListen.peers[from.String()] = c
			go func() {
				for _, exist := udpListen.peers[from.String()]; exist; {
					io.Copy(c.fromAgent, c.bufioCache)
				}
			}()

			go func() {
				udpListen.newPeer <- c.toClient
				io.Copy(&writeRoot{udpListen.rootUdp, from}, c.fromAgent)
				udpListen.rw.Lock()
				delete(udpListen.peers, from.String())
				udpListen.rw.Unlock()
			}()
		}

		udpListen.rw.Unlock()
		udpListen.rw.RLock()
		udpListen.peers[from.String()].bufferCache.Write(buff[:n])
		udpListen.rw.RUnlock()
	}
}
