package udplisterner

import (
	"bytes"
	"io"
	"log"
	"net"
	"net/netip"
	"time"
)

type PipeConn struct {
	root          *net.UDPConn
	to, localAddr netip.AddrPort
	buff          *bytes.Buffer
	closed        bool
	closedChan    chan struct{}
}

func NewConn(root *net.UDPConn, LocalAddr, to netip.AddrPort, buff *bytes.Buffer) *PipeConn {
	return &PipeConn{
		root:       root,
		to:         to,
		localAddr:  LocalAddr,
		closedChan: make(chan struct{}),
		closed:     false,
		buff:       buff,
	}
}

// Send close to channel
func (conn *PipeConn) Close() error {
	conn.closedChan <- struct{}{}
	conn.closed = true // end channel
	return nil
}

func (conn PipeConn) LocalAddr() net.Addr {
	return net.UDPAddrFromAddrPort(conn.localAddr)
}
func (conn PipeConn) RemoteAddr() net.Addr {
	return net.UDPAddrFromAddrPort(conn.to)
}

// Write direct to root UDP Connection
func (conn PipeConn) Write(w []byte) (int, error) {
	return conn.root.WriteToUDPAddrPort(w, conn.to)
}

func (PipeConn) SetDeadline(time.Time) error      { return nil }
func (PipeConn) SetWriteDeadline(time.Time) error { return nil }
func (PipeConn) SetReadDeadline(time.Time) error  { return nil }

func (conn PipeConn) Read(r []byte) (int, error) {
	if conn.closed {
		return 0, io.EOF
	}

	doned := false
	defer func(){
		doned = true
	}()
	time.AfterFunc(time.Second*15, func() {
		if doned {
			return
		}
		doned = true
		conn.Close()
	})
	for conn.buff.Len() < len(r) && !conn.closed {
		log.Println("waiting")
		<-time.After(time.Second)
	}

	return conn.buff.Read(r)
}
