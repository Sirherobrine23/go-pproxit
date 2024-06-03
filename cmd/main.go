package main

import (
	"context"

	_ "github.com/mattn/go-sqlite3"
	"sirherobrine23.org/Minecraft-Server/go-pproxit/server"
	"xorm.io/xorm"
)

func Mac() error {
	engine, err := xorm.NewEngine("sqlite3", "file:./pproxit.db?cache=shared&mode=rwc")
	if err != nil {
		return err
	}
	defer engine.Close()

	var serverController server.Controller
	serverController.ControlPort = 5525 // playit.gg port
	if err := serverController.Listen(context.Background(), engine); err != nil {
		return err
	}
	
	return nil
}

func main() {
	if err := Mac(); err != nil {
		panic(err)
	}
}