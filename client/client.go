package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/netip"
	"time"

	"sirherobrine23.org/Minecraft-Server/go-pproxit/proto"
)

var (
	ErrAgentUnathorized error = errors.New("cannot auth agent and controller not accepted")
)

type Client struct {
	ControlAddr netip.AddrPort      // Controller address
	AgentInfo   *proto.AgentInfo    // Agent info
	Conn        net.Conn            // Agent controller connection
	Token       proto.AgentAuth     // Agent Token
	UDPClients  map[string]net.Conn // UDP Clients
	TCPClients  map[string]net.Conn // TCP Clients
}

func NewClient(ControlAddr netip.AddrPort, Token [36]byte) Client {
	return Client{
		Conn:        nil,
		ControlAddr: ControlAddr,
		AgentInfo:   nil,
		Token:       Token,
		UDPClients:  make(map[string]net.Conn),
		TCPClients:  make(map[string]net.Conn),
	}
}

func (client Client) Send(req proto.Request) error {
	buff, err := req.Wbytes()
	if err != nil {
		return err
	}

	debuglog.Printf("sending %+v\n", buff)
	if _, err = client.Conn.Write(buff); err != nil {
		return err
	}
	return nil
}


func (client *Client) auth() (err error) {
	var res proto.Response
	var req proto.Request
	req.AgentAuth = new(proto.AgentAuth)
	*req.AgentAuth = client.Token

	for {
		buff, err := req.Wbytes()
		debuglog.Printf("Auth: %+v\n", buff)
		if err != nil {
			client.Conn.Close()
			return nil
		} else if _, err = client.Conn.Write(buff); err != nil {
			client.Conn.Close()
			return err
		}
		buff = make([]byte, 2048)
		n, err := client.Conn.Read(buff)
		if err != nil {
			client.Conn.Close()
			return err
		}

		if err = res.Reader(bytes.NewBuffer(buff[:n])); err != nil {
			client.Conn.Close()
			return err
		}
		d, _ := json.Marshal(res)
		debuglog.Printf("Response data: %s\n", string(d))

		if res.BadRequest || res.SendAuth {
			// Wait seconds to resend token
			<-time.After(time.Second * 3)
			continue // Reload auth
		} else if res.Unauthorized {
			// Close tunnel and break loop-de-loop 🦔
			client.Conn.Close()
			return ErrAgentUnathorized
		}
		break
	}
	client.AgentInfo = res.AgentInfo
	return nil
}

func (client *Client) Dial() (err error) {
	if client.Conn, err = net.DialUDP("udp", nil, net.UDPAddrFromAddrPort(client.ControlAddr)); err != nil {
		return err
	} else if err= client.auth(); err != nil {
		return err
	}

	for {
		buff := make([]byte, 2048)
		n, err := client.Conn.Read(buff)
		if err == io.EOF {
			break
		} else if err != nil {
			debuglog.Println(err.Error())
			continue
		}

		var res proto.Response
		if err = res.Reader(bytes.NewBuffer(buff[:n])); err != nil {
			debuglog.Println(err.Error())
			debuglog.Printf("Buffer: %+v\n", buff[:n])
			continue
		}

		d, _ := json.Marshal(res)
		debuglog.Println(string(d))

		if res.BadRequest {
			continue
		} else if res.Unauthorized {
			return ErrAgentUnathorized
		} else if res.SendAuth {
			if err := client.auth(); err != nil {
				return err
			}
		} else if res.NewClient != nil {
		} else if res.DataRX != nil {
		}
	}
	return nil
}
