package udplisterner

import (
	"bytes"
	"io"
	"net"
	"net/netip"
	"time"
)

type PipeConn struct {
	root          *net.UDPConn
	to, localAddr netip.AddrPort
	closed        bool
	closedChan    chan struct{}
	buff          *bytes.Buffer
}

func NewConn(root *net.UDPConn, LocalAddr, to netip.AddrPort) *PipeConn {
	return &PipeConn{
		root:       root,
		to:         to,
		localAddr:  LocalAddr,
		closedChan: make(chan struct{}),
		closed:     false,
		buff:       bytes.NewBuffer(make([]byte, 0)),
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
	count := 50
	for !conn.closed && conn.buff.Len() < len(r) {
		if count--; count == 0 {
			return 0, io.EOF
		}
		<-time.After(time.Second * 5)
	}
	return conn.buff.Read(r)
}
