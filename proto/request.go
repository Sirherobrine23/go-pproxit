package proto

import (
	"encoding/binary"
	"errors"
	"io"
)

var (
	ErrNoRequestOption error = errors.New("set valid option to request to controller server")
)

type Request struct {
	Authentication *Agent        // Send Authencation to Control server
	Ping           *PingPongTime // Send Ping to controller
}

func (req Request) Writer(w io.Writer) error {
	if req.Authentication != nil {
		if err := binary.Write(w, binary.BigEndian, Authentication); err != nil {
			return err // Return error in write request option to auth
		}
		return req.Authentication.Writer(w)
	} else if req.Ping != nil {
		if err := binary.Write(w, binary.BigEndian, PingPong); err != nil {
			return err // Return error in write request option to send ping to controller
		}
		return req.Ping.Writer(w)
	}
	return ErrNoRequestOption
}
