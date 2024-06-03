package proto

import (
	"errors"
	"io"
	"time"

	"sirherobrine23.org/Minecraft-Server/go-pproxit/internal/mut"
)

var (
	ErrNoRequestOption error = errors.New("set valid option to request to controller server")
)

type Request struct {
	Authentication *Agent     // Send Authencation to Control server
	Ping           *time.Time // Send Ping to controller
}

func (req Request) Writer(w io.Writer) error {
	if req.Authentication != nil {
		if err := mut.WriteUint32(w, Authentication); err != nil {
			return err // Return error in write request option to auth
		}
		return req.Authentication.Writer(w)
	} else if req.Ping != nil {
		if err := mut.WriteUint32(w, PingPong); err != nil {
			return err // Return error in write request option to send ping to controller
		}
		return mut.WriteInt64(w, req.Ping.UnixMilli())
	}
	return ErrNoRequestOption
}

func (req *Request) Reader(r io.Reader) error {
	requestID, err := mut.ReadUint32(r)
	if err != nil {
		return err
	}
	switch requestID {
	case Authentication:
		req.Authentication = new(Agent)
		return req.Authentication.Reader(r)
	case PingPong:
		req.Ping = new(time.Time)
		unixMili, err := mut.ReadInt64(r)
		if err != nil {
			return err
		}
		*req.Ping = time.UnixMilli(unixMili)
		return nil
	}
	return ErrInvalidBody
}