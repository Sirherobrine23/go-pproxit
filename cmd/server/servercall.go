package server

import (
	"fmt"
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

type AddrBlocked struct {
	ID      int64 `json:"-" xorm:"pk"` // Tunnel ID
	TunID   int64 `json:"-"`
	Enabled bool
	Address string
}

type RTX struct {
	ID     int64 `json:"-" xorm:"pk"` // Tunnel ID
	TunID  int64 `json:"-"`
	Client netip.AddrPort
	TXSize int
	RXSize int
	Proto  uint8
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
	session.CreateTable(AddrBlocked{})
	session.CreateTable(Ping{})
	session.CreateTable(RTX{})
	return
}

type TunCallbcks struct {
	tunID      int64
	XormEngine *xorm.Engine
}

func (tun *TunCallbcks) AgentShutdown(onTime time.Time) {}

func (tun *TunCallbcks) BlockedAddr(AddrPort string) bool {
	var addr = AddrBlocked{Address: AddrPort, TunID: tun.tunID}
	ok, err := tun.XormEngine.Get(&addr)
	if err != nil {
		fmt.Println(err)
		return true
	} else if ok {
		return addr.Enabled
	}
	var addrs []AddrBlocked
	if err := tun.XormEngine.Find(&addrs); err != nil {
		fmt.Println(err)
		return true
	}
	for ind := range addrs {
		if addrs[ind].Enabled {
			return true
		}
	}
	return false
}

func (tun *TunCallbcks) AgentPing(agent, server time.Time) {
	c, _ := tun.XormEngine.Count(Ping{})
	tun.XormEngine.InsertOne(&Ping{
		ID: c,
		TunID:      tun.tunID,
		ServerTime: server,
		AgentTime:  agent,
	})
}

func (tun *TunCallbcks) RegisterRX(client netip.AddrPort, Size int, Proto uint8) {
	tun.XormEngine.InsertOne(&RTX{
		TunID:  tun.tunID,
		Client: client,
		Proto:  Proto,
		RXSize: Size,
		TXSize: 0,
	})
}
func (tun *TunCallbcks) RegisterTX(client netip.AddrPort, Size int, Proto uint8) {
	tun.XormEngine.InsertOne(&RTX{
		TunID:  tun.tunID,
		Client: client,
		Proto:  Proto,
		TXSize: Size,
		RXSize: 0,
	})
}

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
