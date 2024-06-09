package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/netip"
	"os"
	"os/signal"
	"time"

	"sirherobrine23.org/Minecraft-Server/go-pproxit/client"
	"sirherobrine23.org/Minecraft-Server/go-pproxit/server"
)

func main() {
	cctrl := make(chan os.Signal, 1)
	signal.Notify(cctrl, os.Interrupt)

	var port uint16 = 5522
	server := server.NewServer(nil)
	go server.Listen(port)
	time.Sleep(time.Second)
	fmt.Printf("Server listen on :%d\n", port)

	go func() {
		client := client.NewClient(netip.AddrPortFrom(netip.IPv6Loopback(), port), [36]byte{})
		info, err := client.Dial()
		if err != nil {
			panic(err)
		}
		go client.Backgroud() // Recive data
		fmt.Printf("Client remote: %s\n", client.Conn.RemoteAddr().String())
		fmt.Printf("Client Listened on %d\n", info.LitenerPort)

		localConnect := "127.0.0.1:5201"
		for {
			select {
			case tcp := <-client.NewTCPClient:
				go func()  {
					conn, err := net.Dial("tcp", localConnect)
					if err != nil {
						log.Println(err)
						return
					}
					go io.Copy(conn, tcp)
					go io.Copy(tcp, conn)
				}()
			case udp := <-client.NewUDPClient:
				go func ()  {
					conn, err := net.DialUDP("udp", nil, net.UDPAddrFromAddrPort(netip.MustParseAddrPort(localConnect)))
					if err != nil {
						log.Println(err)
						return
					}
					go io.Copy(conn, udp)
					go io.Copy(udp, conn)
				}()
			}
		}
	}()

	<-cctrl
	fmt.Println("Closing controller")
}
