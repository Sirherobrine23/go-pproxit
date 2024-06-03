package proto

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/netip"

	"github.com/google/uuid"
	"sirherobrine23.org/Minecraft-Server/go-pproxit/internal/mut"
)

var (
	ErrInvalidBody error = errors.New("invalid body, cannot process")
)

const (
	Unauthorized   uint32 = 1 // Request is Unauthorized and not processed
	Authentication uint32 = 2 // Send authencation agent
	PingPong       uint32 = 3 // Ping Request and Pong Response
	ClientNew      uint32 = 4 // Client disconnected
	ClientClose    uint32 = 5 // Client disconnected
	ClientData     uint32 = 6 // Client with data
)

// Agent authencation
type Agent struct {
	AgentID    uuid.UUID // Agent ID
	AgentToken string    // Agent token hex code 64 string size or 32 []byte size
}

func (agent Agent) Writer(w io.Writer) error {
	if err := binary.Write(w, binary.BigEndian, agent.AgentID[:]); err != nil {
		return err
	} else if tokenBytes, err := hex.DecodeString(agent.AgentToken); err != nil {
		return err
	} else if err := binary.Write(w, binary.BigEndian, tokenBytes[:]); err != nil {
		return err
	}
	return nil
}

// Reader agent UUID from Controller
func (agent *Agent) Reader(r io.Reader) error {
	if err := binary.Read(r, binary.BigEndian, agent.AgentID); err != nil {
		return err
	}
	codeBuff, err := mut.ReadBytesN(r, 32)
	if err != nil {
		return err
	}
	agent.AgentToken = hex.EncodeToString(codeBuff)
	return nil
}

type Client struct {
	ConnectType uint8          // Client connection type 1 => TCP or 2 => UDP
	Addr        netip.AddrPort // Client address and port
}

func (client Client) Writer(w io.Writer) error {
	if err := mut.WriteUint8(w, client.ConnectType); err != nil {
		return err
	}
	if client.Addr.Addr().Is6() {
		if err := mut.WriteUint8(w, 6); err != nil {
			return err
		} else if err := mut.WriteBytes[[16]byte](w, client.Addr.Addr().As16()); err != nil {
			return err
		}
	} else {
		if err := mut.WriteUint8(w, 4); err != nil {
			return err
		} else if err := mut.WriteBytes[[4]byte](w, client.Addr.Addr().As4()); err != nil {
			return err
		}
	}
	return mut.WriteUint16(w, client.Addr.Port())
}

func (client *Client) Reader(r io.Reader) error {
	connType, err := mut.ReadUint8(r)
	if err != nil {
		return err
	}
	client.ConnectType = connType
	addrType, err := mut.ReadUint8(r)
	if err != nil {
		return err
	}
	switch addrType {
	case 4:
		ipAddr, err := mut.ReadBytesN(r, 4)
		if err != nil {
			return err
		}
		port, err := mut.ReadUint16(r)
		if err != nil {
			return err
		}
		client.Addr = netip.AddrPortFrom(netip.AddrFrom4([4]byte(ipAddr)), port)
		return nil
	case 6:
		ipAddr, err := mut.ReadBytesN(r, 16)
		if err != nil {
			return err
		}
		port, err := mut.ReadUint16(r)
		if err != nil {
			return err
		}
		client.Addr = netip.AddrPortFrom(netip.AddrFrom4([4]byte(ipAddr)), port)
		return nil
	}
	return ErrInvalidBody
}

type DataClient struct {
	Client        // Client info
	Size   uint64 // Data Size
	Data   []byte // Client data for transport, 1024 is size limit
}

func (client DataClient) Writer(w io.Writer) error {
	if len(client.Data) > 1024 {
		return fmt.Errorf("data to transmiter is biger")
	} else if err := client.Client.Writer(w); err != nil {
		return err
	} else if err := mut.WriteUint64(w, client.Size); err != nil {
		return err
	} else if err := mut.WriteBytes(w, client.Data[:client.Size]); err != nil {
		return err
	}
	return nil
}
func (client *DataClient) Reader(r io.Reader) error {
	if err := client.Client.Reader(r); err != nil {
		return err
	}
	size, err := mut.ReadUint64(r)
	if err != nil {
		return err
	}
	buff, err := mut.ReadBytesN(r, size)
	if err != nil {
		return err
	}
	client.Size = size
	client.Data = buff[:]
	return nil
}
