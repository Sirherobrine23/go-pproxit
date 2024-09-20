package gopproxit_test

import (
	"testing"

	"sirherobrine23.com.br/Minecraft-Server/go-pproxit/client"
	"sirherobrine23.com.br/Minecraft-Server/go-pproxit/server"
)

func TestClientServer(t *testing.T) {
	calls, err := NewCall("./pproxit.db")
	if err != nil {
		t.Error(err)
		return
	}
	controller, err := server.NewController(calls, 8881)
	if err != nil {
		t.Error(err)
		return
	}
	defer controller.ControllConn.Close()
	token := calls.RegisterRandomUser()

	clientConn, err := client.NewClient(controller.ControllConn.Addr().String(), token)
	if err != nil {
		t.Error(err)
		return
	}
	defer clientConn.Close()
}
