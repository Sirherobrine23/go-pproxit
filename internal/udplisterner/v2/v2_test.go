package udplisterner

import (
	"bytes"
	"net"
	"testing"
	"time"
)

func TestListen(t *testing.T) {
	addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	listen, err := Listen("udp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer listen.Close() // end test
	go func(){
		t.Logf("Waiting to accept client ...\n")
		conn, err := listen.Accept()
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		buff := make([]byte, 4)
		if _, err := conn.Read(buff); err != nil {
			t.Fatal(err)
		}
		if bytes.Compare(buff, []byte{1, 9, 9, 1}) != 0 {
			t.Fatalf("cannot get same buffer bytes")
		}
		conn.Write(buff)
	}()

	time.Sleep(time.Microsecond)
	t.Logf("Connecting to %s\n", listen.Addr().String())
	addr, _ = net.ResolveUDPAddr("udp", listen.Addr().String())
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.Write([]byte{1, 9, 9, 1})
	conn.Read(make([]byte, 4))
}