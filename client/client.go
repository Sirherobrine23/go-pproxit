package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/netip"
	"reflect"
	"time"

	"sirherobrine23.org/Minecraft-Server/go-pproxit/internal/pipe"
	"sirherobrine23.org/Minecraft-Server/go-pproxit/proto"
)

var (
	ErrAgentUnathorized error = errors.New("cannot auth agent and controller not accepted")
)

type Client struct {
	ControlAddr    netip.AddrPort      // Controller address
	Conn           net.Conn            // Agent controller connection
	Token          proto.AgentAuth     // Agent Token
	ResponseBuffer uint64              // Agent Reponse Buffer size, Initial size from proto.DataSize
	RequestBuffer  uint64              // Controller send bytes, initial size from proto.DataSize
	LastPong       *time.Time          // Last pong response
	UDPClients     map[string]net.Conn // UDP Clients
	TCPClients     map[string]net.Conn // TCP Clients
	NewUDPClient   chan net.Conn       // Accepts new UDP Clients
	NewTCPClient   chan net.Conn       // Accepts new TCP Clients
}

func NewClient(ControlAddr netip.AddrPort, Token [36]byte) Client {
	return Client{
		ControlAddr: ControlAddr,
		Conn:        nil,
		Token:       Token,

		ResponseBuffer: proto.DataSize,
		RequestBuffer:  proto.DataSize,

		UDPClients:   make(map[string]net.Conn),
		TCPClients:   make(map[string]net.Conn),
		NewUDPClient: make(chan net.Conn),
		NewTCPClient: make(chan net.Conn),
	}
}

// Close client.Conn and Clients
func (client *Client) Close() error {
	client.Conn.Close()
	close(client.NewTCPClient)
	close(client.NewUDPClient)
	for addr, tunUDP := range client.UDPClients {
		tunUDP.Close()
		delete(client.UDPClients, addr)
	}
	for addr, tunTCP := range client.TCPClients {
		tunTCP.Close()
		delete(client.TCPClients, addr)
	}
	return nil
}

func (client Client) Recive() (res *proto.Response, err error) {
	recBuff := make([]byte, client.ResponseBuffer+proto.PacketSize)
	var n int
	if n, err = client.Conn.Read(recBuff); err != nil {
		if opErr, isOp := err.(*net.OpError); isOp {
			log.Println()
			err = opErr.Err
			if reflect.TypeOf(opErr.Err).String() == "poll.errNetClosing" {
				return nil, io.EOF
			}
		}
		return
	}

	res = new(proto.Response)
	if err = res.Reader(bytes.NewBuffer(recBuff[:n])); err != nil {
		return
	}
	d,_:=json.Marshal(res)
	log.Println(string(d))
	return
}

func (client Client) Send(req proto.Request) error {
	buff, err := req.Wbytes()
	if err != nil {
		return err
	} else if _, err = client.Conn.Write(buff); err != nil {
		return err
	}
	return nil
}

// Send token to controller to connect to tunnel
func (client *Client) auth() (info *proto.AgentInfo, err error) {
	attemps := 0
	var res *proto.Response
	for {
		if err = client.Send(proto.Request{AgentAuth: &client.Token}); err != nil {
			client.Conn.Close()
			return
		} else if res, err = client.Recive(); err != nil {
			client.Conn.Close()
			return
		}

		if res.BadRequest || res.SendAuth {
			// Wait seconds to resend token
			<-time.After(time.Second * 3)
			if attemps++; attemps >= 25 {
				err = ErrAgentUnathorized // Cannot auth
				return
			}
			continue // Reload auth
		} else if res.Unauthorized {
			// Close tunnel and break loop-de-loop 🦔
			client.Conn.Close()
			err = ErrAgentUnathorized
			return
		}
		break
	}
	return res.AgentInfo, nil
}

// Dial to controller and auto accept new responses from controller
func (client *Client) Dial() (info *proto.AgentInfo, err error) {
	if client.Conn, err = net.DialUDP("udp", nil, net.UDPAddrFromAddrPort(client.ControlAddr)); err != nil {
		return
	}
	go client.backgroud()
	return client.auth()
}

// Watcher response from controller
func (client *Client) backgroud() (err error) {
	go func(){
		for {
			var current = time.Now()
			client.Send(proto.Request{Ping: &current})
			<-time.After(time.Second * 5)
		}
	}()
	for {
		log.Println("waiting response from controller")
		var res *proto.Response
		if res, err = client.Recive(); err != nil {
			log.Println(err.Error())
			if err == io.EOF {
				break
			}
			continue
		}

		if res.ResizeBuffer != nil {
			client.ResponseBuffer = *res.ResizeBuffer
		} else if res.Pong != nil {
			client.LastPong = res.Pong
			continue // Wait to next response
		} else if res.BadRequest {
			continue
		} else if res.Unauthorized {
			return ErrAgentUnathorized
		} else if res.SendAuth {
			if _, err := client.auth(); err != nil {
				return err
			}
		} else if data := res.DataRX; data != nil {
			if _, exist := client.TCPClients[data.Client.Client.String()]; !exist && data.Client.Proto == proto.ProtoTCP {
				toAgent, toClient := pipe.CreatePipe(client.Conn.RemoteAddr(), net.TCPAddrFromAddrPort(data.Client.Client))
				client.TCPClients[data.Client.Client.String()] = toClient
				client.NewTCPClient <- toAgent // send to Accept
				go func() {
					for {
						buff := make([]byte, client.RequestBuffer)
						n, err := toClient.Read(buff)
						if err != nil {
							if err == io.EOF {
								delete(client.TCPClients, data.Client.Client.String())
								go client.Send(proto.Request{ClientClose: &data.Client})
								go toClient.Close()
								break
							}
							continue
						} else {
							if client.RequestBuffer-uint64(n) == 0 {
								client.RequestBuffer += 500
								var req proto.Request
								req.ResizeBuffer = new(uint64)
								*req.ResizeBuffer = client.RequestBuffer
								client.Send(req)
								<-time.After(time.Microsecond)
							}
							go client.Send(proto.Request{
								DataTX: &proto.ClientData{
									Client: data.Client,
									Size:   uint64(n),
									Data:   buff[:n],
								},
							})
						}
					}
				}()
			} else if _, exist := client.UDPClients[data.Client.Client.String()]; !exist && data.Client.Proto == proto.ProtoUDP {
				toAgent, toClient := pipe.CreatePipe(client.Conn.RemoteAddr(), net.UDPAddrFromAddrPort(data.Client.Client))
				client.UDPClients[data.Client.Client.String()] = toClient
				client.NewUDPClient <- toAgent // send to Accept
				go func() {
					for {
						buff := make([]byte, client.RequestBuffer)
						n, err := toClient.Read(buff)
						if err != nil {
							if err == io.EOF {
								delete(client.UDPClients, data.Client.Client.String())
								go client.Send(proto.Request{ClientClose: &data.Client})
								go toClient.Close()
								break
							}
							continue
						} else {
							if client.RequestBuffer-uint64(n) == 0 {
								var req proto.Request
								req.ResizeBuffer = new(uint64)
								*req.ResizeBuffer = uint64(n)
								go client.Send(req)
							}
							go client.Send(proto.Request{
								DataTX: &proto.ClientData{
									Client: data.Client,
									Size:   uint64(n),
									Data:   buff[:n],
								},
							})
						}
					}
				}()
			}

			if tcpConn, exist := client.TCPClients[data.Client.Client.String()]; exist && data.Client.Proto == proto.ProtoTCP {
				go tcpConn.Write(data.Data)
			} else if udpConn, exist := client.UDPClients[data.Client.Client.String()]; exist && data.Client.Proto == proto.ProtoUDP {
				go udpConn.Write(data.Data)
			}
		} else if closeClient := res.CloseClient; closeClient != nil {
			if tcpConn, exist := client.TCPClients[closeClient.Client.String()]; exist && closeClient.Proto == proto.ProtoTCP {
				delete(client.TCPClients, closeClient.Client.String())
				go tcpConn.Close()
			} else if udpConn, exist := client.UDPClients[closeClient.Client.String()]; exist && closeClient.Proto == proto.ProtoUDP {
				delete(client.UDPClients, closeClient.Client.String())
				go udpConn.Close()
			}
		}
	}
	return nil
}
