package main

import (
	"fmt"
	"os"
	"os/signal"

	"sirherobrine23.org/Minecraft-Server/go-pproxit/server"
)

func main() {
	server := server.NewServer(nil)
	conn, err := server.Listen(5522)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Server listen on %s\n", conn.LocalAddr().String())

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
	fmt.Println("Closing controller")
	conn.Close()
}