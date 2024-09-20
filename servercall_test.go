package gopproxit_test

import (
	"net/netip"
	"sync"
	"time"

	"sirherobrine23.com.br/Minecraft-Server/go-pproxit/proto"
	"sirherobrine23.com.br/Minecraft-Server/go-pproxit/server"
)

type serverCalls struct {
	Locker sync.Locker
	Tables map[string]map[int64]any
}

type User struct {
	ID            int64     // Client ID
	Username      string    // Username
	FullName      string    // Real name for user
	AccountStatus int8      // Account Status
	CreateAt      time.Time // Create date
	UpdateAt      time.Time // Update date
}

type Tun struct {
	ID        int64        // Tunnel ID
	User      int64        // Agent ID
	Token     []byte       // Tunnel Token
	Proto     proto.Protoc // Proto accept
	TPCListen uint16       // Port listen TCP agent
	UDPListen uint16       // Port listen UDP agent
}

type Ping struct {
	ID         int64     `json:"-"` // Tunnel ID
	TunID      int64     `json:"-"`
	ServerTime time.Time `json:"server"`
	AgentTime  time.Time `json:"agent"`
}

type AddrBlocked struct {
	ID      int64 `json:"-"` // Tunnel ID
	TunID   int64 `json:"-"`
	Enabled bool
	Address string
}

type RTX struct {
	ID     int64 `json:"-"` // Tunnel ID
	TunID  int64 `json:"-"`
	Client netip.AddrPort
	TXSize int
	RXSize int
	Proto  proto.Protoc
}

func NewCall() (call *serverCalls, err error) {
	call = new(serverCalls)
	call.Locker = &sync.Mutex{}
	call.Tables = make(map[string]map[int64]any)
	call.Tables["User"] = make(map[int64]any)
	call.Tables["Tun"] = make(map[int64]any)
	call.Tables["AddrBlocked"] = make(map[int64]any)
	call.Tables["Ping"] = make(map[int64]any)
	call.Tables["RTX"] = make(map[int64]any)
	return
}

type TunCallbcks struct {
	tunID  int64
	Locker sync.Locker
}

func (tun *TunCallbcks) BlockedAddr(AddrPort string) bool                               { return false }
func (*TunCallbcks) AgentShutdown(onTime time.Time)                                     {}
func (tun *TunCallbcks) AgentPing(agent, server time.Time)                              {}
func (tun *TunCallbcks) RegisterRX(client netip.AddrPort, Size int, Proto proto.Protoc) {}
func (tun *TunCallbcks) RegisterTX(client netip.AddrPort, Size int, Proto proto.Protoc) {}

func (caller *serverCalls) AgentAuthentication(Token []byte) (server.TunnelInfo, error) {
	for _, tunInfo := range caller.Tables["Tun"] {
		return server.TunnelInfo{
			Proto:   tunInfo.(Tun).Proto,
			TCPPort: tunInfo.(Tun).TPCListen,
			UDPPort: tunInfo.(Tun).UDPListen,
			Callbacks: &TunCallbcks{
				Locker: caller.Locker,
				tunID:  tunInfo.(Tun).ID,
			},
		}, nil
	}
	return server.TunnelInfo{}, server.ErrAuthAgentFail
}

func (caller *serverCalls) RegisterRandomUser() []byte {
	token := []byte{0, 0, 12, 14, 22, 89, 255, 81}
	caller.Tables["User"][0] = User{ID: 0, AccountStatus: 1, FullName: "Radon user", Username: "random"}
	caller.Tables["Tun"][0] = Tun{
		User:      0,
		Token:     token,
		Proto:     proto.ProtoBoth,
		TPCListen: 5522,
		UDPListen: 5522,
	}
	return token
}
