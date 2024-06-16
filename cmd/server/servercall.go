package server

import (
	"net/netip"
	"time"

	_ "modernc.org/sqlite"
	"sirherobrine23.org/Minecraft-Server/go-pproxit/server"
	"xorm.io/xorm"
	"xorm.io/xorm/names"
)

type serverCalls struct {
	XormEngine *xorm.Engine
}

type User struct {
	ID            int64     `xorm:"pk"`                                // Client ID
	Username      string    `xorm:"varchar(32) notnull unique 'user'"` // Username
	FullName      string    `xorm:"text notnull notnull 'name'"`       // Real name for user
	AccountStatus int8      `xorm:"BIT notnull 'status'"`              // Account Status
	CreateAt      time.Time `xorm:"created"`                           // Create date
	UpdateAt      time.Time `xorm:"updated"`                           // Update date
}

type Tun struct {
	ID        int64    `xorm:"pk"`                  // Tunnel ID
	User      int64    `xorm:"notnull"`             // Agent ID
	Token     [36]byte `xorm:"blob notnull unique"` // Tunnel Token
	Proto     uint8    `xorm:"default 3"`           // Proto accept
	TPCListen uint16   // Port listen TCP agent
	UDPListen uint16   // Port listen UDP agent
}

type Ping struct {
	ID         int64     `json:"-" xorm:"pk"` // Tunnel ID
	TunID      int64     `json:"-"`
	ServerTime time.Time `json:"server" xorm:"datetime notnull"`
	AgentTime  time.Time `json:"agent" xorm:"datetime notnull"`
}

func NewCall(DBConn string) (call *serverCalls, err error) {
	call = new(serverCalls)
	if call.XormEngine, err = xorm.NewEngine("sqlite", DBConn); err != nil {
		return
	}
	call.XormEngine.SetMapper(names.SameMapper{})
	session := call.XormEngine.NewSession()
	defer session.Close()
	session.CreateTable(User{})
	session.CreateTable(Tun{})
	session.CreateTable(Ping{})
	return
}

type TunCallbcks struct {
	tunID      int64
	XormEngine *xorm.Engine
}

func (tun *TunCallbcks) BlockedAddr(AddrPort netip.Addr) bool                    { return false }
func (tun *TunCallbcks) AgentPing(agent, server time.Time)                       {}
func (tun *TunCallbcks) AgentShutdown(onTime time.Time)                          {}
func (tun *TunCallbcks) RegisterRX(client netip.AddrPort, Size int, Proto uint8) {}
func (tun *TunCallbcks) RegisterTX(client netip.AddrPort, Size int, Proto uint8) {}

func (caller *serverCalls) AgentAuthentication(Token [36]byte) (server.TunnelInfo, error) {
	var tun = Tun{Token: Token}
	if ok, err := caller.XormEngine.Get(&tun); err != nil || !ok {
		if !ok {
			return server.TunnelInfo{}, server.ErrAuthAgentFail
		}
		return server.TunnelInfo{}, err
	}
	return server.TunnelInfo{
		Proto:     tun.Proto,
		TCPPort:   tun.TPCListen,
		UDPPort:   tun.UDPListen,
		Callbacks: &TunCallbcks{tunID: tun.ID, XormEngine: caller.XormEngine},
	}, nil
}
