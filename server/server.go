package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/netip"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/google/uuid"
	"sirherobrine23.org/Minecraft-Server/go-pproxit/api"
	"sirherobrine23.org/Minecraft-Server/go-pproxit/proto"
	"xorm.io/xorm"
)

type AgentDB struct {
	ID       uuid.UUID `xorm:"'id' primary"`      // Agent ID
	Token    []string  `xorm:"'tokens' not null"` // Tokens
	CreateAt time.Time `xorm:"'createat' not null"`
	UpdateAt time.Time `xorm:"'updateat' not null"`
}

type TunnelDB struct {
	AgentID   uuid.UUID `xorm:"'agent' not null"` // Agent assined id
	TunnelID  uuid.UUID `xorm:"'id' not null"`    // Tunnel ID
	TunelPort uint16    `xorm:"'port' not null"`  // Port to listener
}

type Tunnel struct {
	Proto     uint8               // 1 => TCP, 2 => UDP or 3 => Both
	Port      uint16              // Port to listen and watcher
	running   bool                // Set if listening accept new connections
	AgentAddr netip.AddrPort      // Agent ip address and port
	TCPDial   *net.TCPListener    // TCP Server connections
	UDPDial   *net.UDPConn        // UDP Server connections
	TCPClient map[string]net.Conn // TCP Clients to redirect connections
	UDPClient map[string]net.Conn // UDP Clients to redirect connections
}

// End current connections from current Cliets
func (tun *Tunnel) Close() {
	tun.running = false
	if tun.TCPDial != nil {
		tun.TCPDial.Close()
	}
	if tun.UDPDial != nil {
		tun.TCPDial.Close()
	}
	for _, client := range tun.TCPClient {
		client.Close()
	}
	for _, client := range tun.UDPClient {
		client.Close()
	}
}

// Listen tunnel
func (tun *Tunnel) Listen() (err error) {
	if tun.Proto == 3 || tun.Proto == 1 {
		if tun.TCPDial, err = net.ListenTCP("tcp", net.TCPAddrFromAddrPort(netip.AddrPortFrom(netip.IPv4Unspecified(), tun.Port))); err != nil {
			return // Dont call next listner if both request
		}
	}
	if tun.Proto == 3 || tun.Proto == 2 {
		if tun.UDPDial, err = net.ListenUDP("udp", net.UDPAddrFromAddrPort(netip.AddrPortFrom(netip.IPv4Unspecified(), tun.Port))); err != nil {
			if tun.TCPDial != nil {
				tun.TCPDial.Close() // Close TCP if listened
			}
		}
	}
	return
}

// Call this function in goroutine
func (tun *Tunnel) Run(connCallback func(proto uint8, conn net.Conn)) error {
	tun.running = true

	// pipe tcp clients
	if tun.TCPDial != nil {
		defer tun.TCPDial.Close() // Close if sessions ends
		go func() {
			for tun.running {
				conn, err := tun.TCPDial.Accept()
				if err != nil {
					continue // Ignore error
				}
				tun.TCPClient[conn.RemoteAddr().String()] = conn // Set to tcp clients
				connCallback(1, conn)                            // return connection to controller
			}
		}()
	}

	// Get new clients from UDP Connection
	if tun.UDPDial != nil {
		defer tun.UDPDial.Close() // Close if sessions ends
		go func() {
			for tun.running {
				buff := make([]byte, 1024)
				size, remote, err := tun.UDPDial.ReadFromUDP(buff)
				if err != nil {
					continue // Continue reading
				} else if r, exist := tun.UDPClient[remote.String()]; exist {
					r.Write(buff[:size])
					continue
				}

				// New client listened
				c, r := net.Pipe() // Create duplex connection to pipe same for TCP Connection callback
				go func() {
					for {
						buff := make([]byte, 1024)
						size, err := r.Read(buff)
						if err == io.EOF {
							delete(tun.UDPClient, remote.String()) // Remove from UDP Clients
							return
						} else if err != nil {
							continue
						}
						tun.UDPDial.WriteToUDP(buff[:size], remote)
					}
				}()

				tun.UDPClient[remote.String()] = r // Set client write
				connCallback(2, c)                 // return client read
			}
		}()
	}
	return nil
}

type Controller struct {
	ControlPort uint16               // Port to controller server
	Tunnels     map[uuid.UUID]Tunnel // Tunnels
}

func (com *Controller) AgentRemoteExist(addr netip.AddrPort) (uuid.UUID, bool) {
	for uid, tun := range com.Tunnels {
		if tun.AgentAddr.String() == addr.String() {
			return uid, true
		}
	}
	return uuid.UUID{}, false
}

func (com *Controller) CloseTunnels() {
	for _, tun := range com.Tunnels {
		tun.Close() // End tunnel process
	}
}

func (com *Controller) Listen(ctx context.Context, XormEngine *xorm.Engine) error {
	XormEngine.CreateTables(TunnelDB{}, AgentDB{}) // Create tables
	go func() {
		app := fiber.New(fiber.Config{
			ETag: false,
			ErrorHandler: func(ctx *fiber.Ctx, err error) error {
				code := fiber.StatusInternalServerError
				if fiError, is := err.(*fiber.Error); is {
					code = fiError.Code
				}
				ctx.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSONCharsetUTF8)
				return ctx.Status(code).JSON(fiber.Map{"error": err.Error()})
			},
		})

		app.Use(limiter.New(limiter.Config{
			Max: 50,
			Expiration: time.Second * 12,
			LimitReached: func(c *fiber.Ctx) error {
				return fiber.NewError(fiber.StatusTooManyRequests, "Wait seconds to make request")
			},
		}))

		app.Use(func(c *fiber.Ctx) error {
			req := c.Request()
			var token uuid.UUID
			if Auth := req.Header.Peek("Autorization"); len(Auth) > 0 && strings.HasPrefix(strings.ToLower(string(Auth)), "token") {
				token = uuid.MustParse(strings.TrimSpace(string(Auth[5:])))
			} else {
				return fiber.NewError(401, "Require authetication to maneger API")
			}
			agentData := new(AgentDB)
			if has, err := XormEngine.In("tokens", token.String()).Get(agentData); err != nil || !has {
			if !has {
					return fiber.NewError(401, "Require valid token")
				}
				return fiber.NewError(500, err.Error())
			}
			c.Locals("ApiToken", token)
			c.Locals("AgentData", agentData)
			return c.Next()
		})

		app.Get("/agent", func(c *fiber.Ctx) error {
			var body api.AgentRouting
			return json.NewEncoder(c.Status(200)).Encode(body)
		})

		tunnelApp := app.Group("/tunnel")
		tunnelApp.Get("/", func(c *fiber.Ctx) error {
			agentInfo := c.Locals("AgentData").(*AgentDB)
			var Tuns []TunnelDB
			XormEngine.In("agent", agentInfo.ID.String()).Find(&Tuns)
			c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSONCharsetUTF8)
			return c.Status(200).JSON(Tuns)
		})

		app.Use(func(c *fiber.Ctx) error {
			c.Response().Header.Add("Content-Type", "application/json")
			return json.NewEncoder(c.Status(404)).Encode(struct {
				Path    string `json:"path"`
				Message string `json:"message"`
			}{
				string(c.Request().URI().Path()),
				"Path request not registred or is another request method",
			})
		})
		go app.Listen(fmt.Sprintf(":%d", com.ControlPort))
		app.Server().ShutdownWithContext(ctx)
	}()
	conn, err := net.ListenUDP("udp", net.UDPAddrFromAddrPort(netip.AddrPortFrom(netip.IPv4Unspecified(), com.ControlPort)))
	if err != nil {
		return err
	}
	listening := true
	go func() {
		<-ctx.Done()
		listening = false
	}()
	defer com.CloseTunnels() // end tunnels and clients
	for listening {
		buff := make([]byte, 2048)
		size, remote, err := conn.ReadFromUDPAddrPort(buff)
		if err != nil {
			return err
		}
		var req proto.Request
		if err := req.Reader(bytes.NewBuffer(buff[:size])); err != nil {
			continue
		}
		if req.Ping != nil {
			buff := new(bytes.Buffer)
			res := proto.Response{}
			res.Pong = new(time.Time)
			*res.Pong = time.Now()
			if err := res.Writer(buff); err != nil {
				continue
			}
			conn.WriteToUDPAddrPort(buff.Bytes(), remote)
			continue
		}
	}
	return ctx.Err()
}
