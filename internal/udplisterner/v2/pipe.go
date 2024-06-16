package udplisterner

import (
	"bufio"
	"io"
	"net"
	"net/netip"
	"time"
)

type PipeConn struct {
	root          *net.UDPConn
	to, localAddr netip.AddrPort
	buff          *bufio.ReadWriter
	closed        bool
	closedChan    chan struct{}
}

func NewConn(root *net.UDPConn, LocalAddr, to netip.AddrPort, buff *bufio.ReadWriter) net.Conn {
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
	conn.buff.Discard(conn.buff.Available())
	conn.buff.Flush()
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
	for {
		if conn.closed {
			return 0, io.EOF
		} else if conn.buff.Available() >= len(r) {
			return conn.buff.Read(r)
		}
		<-time.After(time.Millisecond) // wait 1ms
	}
}
