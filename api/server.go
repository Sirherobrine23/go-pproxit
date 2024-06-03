package api

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"xorm.io/xorm"
)

type Server struct {
	PortListen uint16       // Port to listen API
	XormEngine *xorm.Engine // xorm engine connection to maneger API routes
}

func (api Server) Listen() error {
	api.XormEngine.CreateTables(Tunnel{}, Agent{}) // Create tables
	app := fiber.New()

	app.Use("/api", func(c *fiber.Ctx) error {
		req := c.Request()
		var token string
		if cookie := req.Header.Cookie("WebToken"); len(cookie) > 0 {
			token = string(cookie)
		} else if Auth := req.Header.Peek("Autorization"); len(Auth) > 0 {
			token = string(Auth)
		} else {
			c.Response().Header.Add("Content-Type", "application/json")
			return json.NewEncoder(c.Status(401)).Encode(struct{ message string }{"Require authetication to maneger API"})
		}
		if has, err := api.XormEngine.Get(&Agent{}); err != nil || !has {
			if !has {
				c.Response().Header.Add("Content-Type", "application/json")
				return json.NewEncoder(c.Status(401)).Encode(struct{ message string }{"Require authetication to maneger API"})
			}
			c.Response().Header.Add("Content-Type", "application/json")
			return json.NewEncoder(c.Status(500)).Encode(struct{ message string }{err.Error()})
		}
		c.Locals("ApiToken", token)
		return c.Next()
	})

	app.Get("/api/agent", func(c *fiber.Ctx) error {

		return nil
	})

	return app.Listen(fmt.Sprintf(":%d", api.PortListen))
}

type Agent struct {
	ID       uuid.UUID `xorm:"'id' primary"`      // Agent ID
	Token    []string  `xorm:"'tokens' not null"` // Tokens
	CreateAt time.Time `xorm:"'createat' not null"`
	UpdateAt time.Time `xorm:"'updateat' not null"`
}

type Tunnel struct {
	AgentID   uuid.UUID `xorm:"'agent' not null"` // Agent assined id
	TunnelID  uuid.UUID `xorm:"'id' not null"`    // Tunnel ID
	TunelPort uint16    `xorm:"'port' not null"`  // Port to listener
}
