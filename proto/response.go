package proto

import (
	"io"

	"sirherobrine23.org/Minecraft-Server/go-pproxit/internal/mut"
)

type Response struct {
	Unauthorized bool          // if Request is not authenticated
	Pong         *PingPongTime // Ping request
}

func ResponseBytes(r io.Reader) (res Response, err error) {
	id, err := mut.ReadUint32(r)
	if err != nil {
		return res, err
	}

	switch id {
	case Unauthorized:
		res.Unauthorized = true // Request not processed
	case PingPong:
		res.Pong = new(PingPongTime)
		res.Pong.Reader(r)
	default:
		err = ErrInvalidBody
	}

	return
}
