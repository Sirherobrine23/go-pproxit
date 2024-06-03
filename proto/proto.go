package proto

import (
	"encoding/binary"
	"errors"
	"io"
	"net/netip"
	"time"

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
	ClientClose    uint32 = 4 // Client disconnected
	ClientData     uint32 = 5 // Client with data
)

// Writer agent UUID
type Agent uuid.UUID

func (agent Agent) Writer(w io.Writer) error {
	return binary.Write(w, binary.BigEndian, agent[:]) //
}

// Reader agent UUID from Controller
func (agent Agent) Reader(r io.Reader) error {
	return binary.Read(r, binary.BigEndian, agent[:])
}

// Ping and Pong response
type PingPongTime struct{ time.Time }

// Write unix milliseconds to stream
func (ping *PingPongTime) Writer(w io.Writer) error {
	return binary.Write(w, binary.BigEndian, ping.UnixMilli())
}

// Reader stream and get time from unix milliseconds
func (pong *PingPongTime) Reader(r io.Reader) error {
	timeUnix, err := mut.ReadInt64(r)
	if err == nil {
		pong.Time = time.UnixMilli(timeUnix)
	}
	return err
}

type Client struct {
	Addr        netip.AddrPort // Client address and port
	ConnectType uint8          // Client connection type 1 => TCP or 2 => UDP
}

type DataClient struct {
	Client        // Client info
	Data   []byte // Client data for transport, 1024 is size limit
}
