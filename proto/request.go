package proto

import (
	"bytes"
	"errors"
	"io"
	"time"

	"sirherobrine23.org/Minecraft-Server/go-pproxit/internal/bigendian"
)

const (
	ReqAuth        uint64 = 1 // Request Agent Auth
	ReqPing        uint64 = 2 // Time ping
	ReqCloseClient uint64 = 3 // Close client
	ReqClientData  uint64 = 4 // Send data
	ReqResize      uint64 = 5 // Resize request buffer
)

var (
	ErrProtoBothNoSupported error = errors.New("protocol UDP+TCP not supported currently")
)

type AgentAuth [36]byte

func (agent AgentAuth) Writer(w io.Writer) error {
	if err := bigendian.WriteBytes(w, agent[:]); err != nil {
		return err
	}
	return nil
}
func (agent *AgentAuth) Reader(r io.Reader) error {
	if err := bigendian.ReaderBytes(r, agent[:], 36); err != nil {
		return err
	}
	return nil
}

// Send request to agent and wait response
type Request struct {
	AgentAuth    *AgentAuth  // Send agent authentication to controller
	Ping         *time.Time  // Send ping time to controller in unix milliseconds
	ClientClose  *Client     // Close client in controller
	DataTX       *ClientData // Recive data from agent
	ResizeBuffer *uint64     // Resize request buffer
}

// Get Bytes from Request
func (req Request) Wbytes() ([]byte, error) {
	buff := new(bytes.Buffer)
	if err := req.Writer(buff); err != nil {
		return nil, err
	}
	return buff.Bytes(), nil
}

func (req Request) Writer(w io.Writer) error {
	if auth := req.AgentAuth; auth != nil {
		if err := bigendian.WriteUint64(w, ReqAuth); err != nil {
			return err
		}
		return auth.Writer(w)
	} else if ping := req.Ping; ping != nil {
		if err := bigendian.WriteUint64(w, ReqPing); err != nil {
			return err
		}
		return bigendian.WriteInt64(w, ping.UnixMilli())
	} else if close := req.ClientClose; close != nil {
		if err := bigendian.WriteUint64(w, ReqCloseClient); err != nil {
			return err
		}
		return close.Writer(w)
	} else if data := req.DataTX; data != nil {
		if err := bigendian.WriteUint64(w, ReqClientData); err != nil {
			return err
		}
		return data.Writer(w)
	} else if req.ResizeBuffer != nil {
		if err := bigendian.WriteUint64(w, ReqResize); err != nil {
			return err
		}
		return bigendian.WriteUint64(w, *req.ResizeBuffer)
	}
	return ErrInvalidBody
}
func (req *Request) Reader(r io.Reader) (err error) {
	var reqID uint64
	if reqID, err = bigendian.ReadUint64(r); err != nil {
		return
	}
	if reqID == ReqAuth {
		req.AgentAuth = new(AgentAuth)
		return req.AgentAuth.Reader(r)
	} else if reqID == ReqPing {
		var timeUnix int64
		if timeUnix, err = bigendian.ReadInt64(r); err != nil {
			return
		}
		req.Ping = new(time.Time)
		*req.Ping = time.UnixMilli(timeUnix)
		return
	} else if reqID == ReqCloseClient {
		req.ClientClose = new(Client)
		return req.ClientClose.Reader(r)
	} else if reqID == ReqClientData {
		req.DataTX = new(ClientData)
		return req.DataTX.Reader(r)
	} else if reqID == ReqResize {
		req.ResizeBuffer = new(uint64)
		*req.ResizeBuffer, err = bigendian.ReadUint64(r)
		return
	}
	return ErrInvalidBody
}
