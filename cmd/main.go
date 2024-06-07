package main

import (
	"fmt"
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
	fmt.Printf("Server listen on :%d\n", port)

	go func() {
		client := client.NewClient(netip.AddrPortFrom(netip.IPv6Loopback(), port), [36]byte{})
		go client.Dial()
		for client.Conn == nil {
			<-time.After(time.Second)
		}
		fmt.Printf("Client remote: %s\n", client.Conn.RemoteAddr().String())
		fmt.Printf("Client Listened on %d\n", client.AgentInfo.LitenerPort)
	}()

	<-cctrl
	fmt.Println("Closing controller")
}
