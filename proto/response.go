package proto

import (
	"bytes"
	"io"
	"net/netip"
	"time"

	"sirherobrine23.org/Minecraft-Server/go-pproxit/internal/bigendian"
)

const (
	ResUnauthorized uint64 = 1 // Request not processed and ignored
	ResBadRequest   uint64 = 2 // Request cannot process and ignored
	ResCloseClient  uint64 = 3 // Controller closed connection
	ResClientData   uint64 = 4 // Controller accepted data
	ResSendAuth     uint64 = 5 // Send token to controller
	ResAgentInfo    uint64 = 6 // Agent info
	ResPong         uint64 = 7 // Ping response
	ResNotListening uint64 = 8 // Resize buffer size
)

type AgentInfo struct {
	Protocol         uint8          // Proto supported (proto.ProtoTCP, proto.ProtoUDP or proto.ProtoBoth)
	UDPPort, TCPPort uint16         // Controller port listened
	AddrPort         netip.AddrPort // request address and port
}

func (agent AgentInfo) Writer(w io.Writer) error {
	if err := bigendian.WriteUint8(w, agent.Protocol); err != nil {
		return err
	} else if err := bigendian.WriteUint16(w, agent.UDPPort); err != nil {
		return err
	} else if err := bigendian.WriteUint16(w, agent.TCPPort); err != nil {
		return err
	}
	addr := agent.AddrPort.Addr()
	if addr.Is4() {
		if err := bigendian.WriteUint8(w, 4); err != nil {
			return err
		} else if err := bigendian.WriteBytes(w, addr.As4()); err != nil {
			return err
		}
	} else {
		if err := bigendian.WriteUint8(w, 6); err != nil {
			return err
		} else if err := bigendian.WriteBytes(w, addr.As16()); err != nil {
			return err
		}
	}
	if err := bigendian.WriteUint16(w, agent.AddrPort.Port()); err != nil {
		return err
	}
	return nil
}
func (agent *AgentInfo) Reader(r io.Reader) (err error) {
	if agent.Protocol, err = bigendian.ReadUint8(r); err != nil {
		return
	} else if agent.UDPPort, err = bigendian.ReadUint16(r); err != nil {
		return
	} else if agent.TCPPort, err = bigendian.ReadUint16(r); err != nil {
		return
	}
	var addrFamily uint8
	var addrPort uint16
	var ipBytes []byte
	if addrFamily, err = bigendian.ReadUint8(r); err != nil {
		return
	} else if addrFamily == 4 {
		if ipBytes, err = bigendian.ReadBytesN(r, 4); err != nil {
			return
		}
	} else if addrFamily == 6 {
		if ipBytes, err = bigendian.ReadBytesN(r, 16); err != nil {
			return
		}
	}
	if addrPort, err = bigendian.ReadUint16(r); err != nil {
		return
	} else if len(ipBytes) == 16 {
		agent.AddrPort = netip.AddrPortFrom(netip.AddrFrom16([16]byte(ipBytes)), addrPort)
	} else {
		agent.AddrPort = netip.AddrPortFrom(netip.AddrFrom4([4]byte(ipBytes)), addrPort)
	}
	return
}

// Reader data from Controller and process in agent
type Response struct {
	Unauthorized bool `json:",omitempty"` // Controller reject connection
	BadRequest   bool `json:",omitempty"` // Controller accepted packet so cannot process Request
	SendAuth     bool `json:",omitempty"` // Send Agent token
	NotListened  bool `json:",omitempty"` // Controller cannot Listen port

	AgentInfo *AgentInfo `json:",omitempty"` // Agent Info
	Pong      *time.Time `json:",omitempty"` // ping response

	CloseClient *Client     `json:",omitempty"` // Controller end client
	DataRX      *ClientData `json:",omitempty"` // Controller recive data from client
}

func ReaderResponse(r io.Reader) (*Response, error) {
	res := &Response{}
	if err := res.Reader(r); err != nil {
		return nil, err
	}
	return res, nil
}

func WriteResponse(w io.Writer, res Response) error {
	buff, err := res.Wbytes()
	if err != nil {
		return err
	} else if _, err := w.Write(buff); err != nil {
		return err
	}
	return nil
}

// Get Bytes from Response
func (req Response) Wbytes() ([]byte, error) {
	buff := new(bytes.Buffer)
	if err := req.Writer(buff); err != nil {
		return nil, err
	}
	return buff.Bytes(), nil
}

func (res Response) Writer(w io.Writer) error {
	if res.Unauthorized {
		return bigendian.WriteUint64(w, ResUnauthorized)
	} else if res.BadRequest {
		return bigendian.WriteUint64(w, ResBadRequest)
	} else if res.SendAuth {
		return bigendian.WriteUint64(w, ResSendAuth)
	} else if res.NotListened {
		return bigendian.WriteUint64(w, ResNotListening)
	} else if pong := res.Pong; pong != nil {
		if err := bigendian.WriteUint64(w, ResPong); err != nil {
			return err
		}
		return bigendian.WriteInt64(w, pong.UnixMilli())
	} else if closeClient := res.CloseClient; closeClient != nil {
		if err := bigendian.WriteUint64(w, ResCloseClient); err != nil {
			return err
		}
		return closeClient.Writer(w)
	} else if rx := res.DataRX; rx != nil {
		if err := bigendian.WriteUint64(w, ResClientData); err != nil {
			return err
		}
		return rx.Writer(w)
	} else if info := res.AgentInfo; info != nil {
		if err := bigendian.WriteUint64(w, ResAgentInfo); err != nil {
			return err
		}
		return info.Writer(w)
	}
	return ErrInvalidBody
}
func (res *Response) Reader(r io.Reader) error {
	resID, err := bigendian.ReadUint64(r)
	if err != nil {
		return err
	}

	if resID == ResBadRequest {
		res.BadRequest = true
		return nil
	} else if resID == ResUnauthorized {
		res.Unauthorized = true
		return nil
	} else if resID == ResNotListening {
		res.NotListened = true
		return nil
	} else if resID == ResSendAuth {
		res.SendAuth = true
		return nil
	} else if resID == ResCloseClient {
		res.CloseClient = new(Client)
		return res.CloseClient.Reader(r)
	} else if resID == ResClientData {
		res.DataRX = new(ClientData)
		return res.DataRX.Reader(r)
	} else if resID == ResAgentInfo {
		res.AgentInfo = new(AgentInfo)
		return res.AgentInfo.Reader(r)
	} else if resID == ResPong {
		unixMil, err := bigendian.ReadInt64(r)
		if err != nil {
			return err
		}
		res.Pong = new(time.Time)
		*res.Pong = time.UnixMilli(unixMil)
		return nil
	}
	return ErrInvalidBody
}
