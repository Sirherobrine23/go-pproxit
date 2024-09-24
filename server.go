package gopproxit

import (
	"encoding/hex"
	"errors"
	"io"
	"net"
	"net/netip"
	"reflect"
	"sync"

	"sirherobrine23.com.br/Minecraft-Server/go-pproxit/internal/structcode"
	"sirherobrine23.com.br/Minecraft-Server/go-pproxit/proto"
)

var (
	ErrAuthInvalidToken error = errors.New("invalid token authentication") // Token is bad
	ErrAuthUnauthorized error = errors.New("unauthorized authentication")  // Token not exists
)

type ServerServices interface {
	Auth(token []byte) error                      // Authenticate client
	Agent(token []byte) (*proto.AgentInfo, error) // Informations to agent
	SkipAddress(addr netip.AddrPort) bool         // Close/skip address process
}

type Server struct {
	Controller net.Listener   // Controller connection
	Services   ServerServices // Services to auth and another options

	tunLock sync.Locker
	Tuns    map[string]*Tunnel

	errChannel chan error
}

func NewServer(addr string, service ServerServices) (*Server, error) {
	var server Server
	var err error
	if server.Controller, err = net.Listen("tcp", addr); err != nil {
		return nil, err
	}
	server.Services = service
	server.errChannel = make(chan error)
	server.tunLock = &sync.Mutex{}

	go server.accepts()
	return &server, nil
}

// Send error to WaitError, if closed catcher and ignored
func (server *Server) sendErr(err error) {
	if reflect.ValueOf(server.errChannel).IsZero() {
		return
	}
	defer func() { recover() }()
	server.errChannel <- err
}

// process new connections
func (server *Server) accepts() {
	for server.Controller != nil {
		conn, err := server.Controller.Accept()
		if err != nil {
			server.sendErr(err)
			return
		} else if server.Services.SkipAddress(netip.MustParseAddrPort(conn.RemoteAddr().String())) {
			conn.Close()
			continue
		}
		go func(conn net.Conn) {
			defer conn.Close()
			var req proto.Request
			i := 0
			for {
				if i++; i > 5 {
					return
				} else if err := structcode.NewDecode(conn, &req); err != nil {
					if err == io.EOF {
						return
					}
					server.sendErr(err)
					continue
				} else if err := server.Services.Auth(req.AgentAuth); err != nil {
					if err == ErrAuthUnauthorized {
						structcode.NewEncode(conn, proto.Response{Unauthorized: true})
						return
					} else if err == ErrAuthInvalidToken {
						structcode.NewEncode(conn, proto.Response{Unauthorized: true})
						return
					}
					structcode.NewEncode(conn, proto.Response{BadRequest: true})
					server.sendErr(err)
					continue
				}
				break
			}

			info, err := server.Services.Agent(req.AgentAuth)
			if err != nil {
				structcode.NewEncode(conn, proto.Response{Error: err})
				server.sendErr(err)
				conn.Close()
				return
			} else if err := structcode.NewEncode(conn, proto.Response{AgentInfo: info}); err != nil {
				server.sendErr(err)
				conn.Close()
				return
			}
			tokenHex := hex.EncodeToString(req.AgentAuth)

			server.tunLock.Lock()
			defer conn.Close()
			tun := NewTun(conn, info)
			if oldTun, ok := server.Tuns[tokenHex]; ok {
				oldTun.Close()
				delete(server.Tuns, tokenHex)
			}

			server.Tuns[tokenHex] = tun
			server.tunLock.Unlock()
			defer func() {
				server.tunLock.Lock()
				defer server.tunLock.Unlock()
				delete(server.Tuns, tokenHex)
			}()

			for {
				select {
				case err := <-tun.TunErr:
					server.sendErr(err)
				case <-tun.Done:
					return
				}
			}
		}(conn)
	}
}
