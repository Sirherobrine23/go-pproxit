package proto

import (
	"io"
	"time"

	"sirherobrine23.org/Minecraft-Server/go-pproxit/internal/mut"
)

type Response struct {
	Unauthorized bool        // if Request is not authenticated
	Pong         *time.Time  // Ping request
	NewClient    *Client     // Add new client to clients list
	CloseClient  *Client     // Close client from clients list
	ClientData   *DataClient // Write data to specific client
}

func (res Response) Writer(w io.Writer) error {
	if res.Unauthorized {
		return mut.WriteUint32(w, Unauthorized)
	} else if res.Pong != nil {
		if err := mut.WriteUint32(w, PingPong); err != nil {
			return err
		}
		return mut.WriteUint32(w, Unauthorized)
	} else if res.NewClient != nil {
		if err := mut.WriteUint32(w, ClientNew); err != nil {
			return err
		}
		return res.CloseClient.Writer(w)
	} else if res.CloseClient != nil {
		if err := mut.WriteUint32(w, ClientClose); err != nil {
			return err
		}
		return res.CloseClient.Writer(w)
	} else if res.ClientData != nil {
		if err := mut.WriteUint32(w, ClientData); err != nil {
			return err
		}
		return res.ClientData.Writer(w)
	}
	return ErrNoRequestOption
}

func (res *Response) Reader(r io.Reader) error {
	id, err := mut.ReadUint32(r)
	if err != nil {
		return err
	}

	switch id {
	case Unauthorized:
		res.Unauthorized = true // Request not processed
		return nil
	case PingPong:
		res.Pong = new(time.Time)
		unixMil, err := mut.ReadInt64(r)
		if err != nil {
			return err
		}
		*res.Pong = time.UnixMilli(unixMil)
		return nil
	case ClientNew:
		res.NewClient = new(Client)
		return res.NewClient.Reader(r)
	case ClientClose:
		res.CloseClient = new(Client)
		return res.CloseClient.Reader(r)
	case ClientData:
		res.ClientData = new(DataClient)
		res.ClientData.Client = Client{}
		return res.ClientData.Reader(r)
	}

	return ErrInvalidBody
}
