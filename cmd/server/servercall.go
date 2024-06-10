package server

import (
	"time"

	_ "modernc.org/sqlite"
	"sirherobrine23.org/Minecraft-Server/go-pproxit/server"
	"xorm.io/xorm"
	"xorm.io/xorm/names"
)

type serverCalls struct {
	XormEngine *xorm.Engine
}

type Tun struct {
	ID         int64    `xorm:"pk"` // Tunnel ID
	User       int64    // Agent ID
	Token      [36]byte // Tunnel Token
	Proto      uint8    // Proto accept
	PortListen uint16   // Port listen agent
}

type User struct {
	ID            int64     `xorm:"pk"`                                // Client ID
	Username      string    `xorm:"varchar(32) notnull unique 'user'"` // Username
	FullName      string    `xorm:"text notnull notnull 'name'"`       // Real name for user
	AccountStatus int8      `xorm:"BIT notnull 'status'"`              // Account Status
	CreateAt      time.Time `xorm:"created"`                           // Create date
	UpdateAt      time.Time `xorm:"updated"`                           // Update date
}

func NewCall(DBConn string) (call *serverCalls, err error) {
	call = new(serverCalls)
	if call.XormEngine, err = xorm.NewEngine("sqlite", DBConn); err != nil {
		return
	}
	call.XormEngine.SetMapper(names.SameMapper{})
	call.XormEngine.CreateTables(Tun{}, User{})
	return
}

func (call serverCalls) AgentShutdown(Token [36]byte) (err error) { return } // Ignore

func (call serverCalls) AgentInfo(Token [36]byte) (server.TunnelInfo, error) {
	var tun = Tun{Token: Token}
	if ok, err := call.XormEngine.Get(&tun); err != nil || !ok {
		if !ok {
			return server.TunnelInfo{}, server.ErrNoAgent
		}
		return server.TunnelInfo{}, err
	}
	return server.TunnelInfo{
		PortListen: tun.PortListen,
		Proto:      tun.Proto,
	}, nil
}
