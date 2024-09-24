package proto

import (
	"errors"
	"net/netip"
)

const (
	ProtoBoth Protoc = iota // TCP+UDP Protocol
	ProtoTCP                // TCP Protocol
	ProtoUDP                // UDP Protocol
)

var ErrInvalidBody error = errors.New("invalid body, check request/response")

type Protoc uint8 // Net protocol support

func (pr Protoc) MarshalText() ([]byte, error) { return []byte(pr.String()), nil }
func (pr Protoc) String() string {
	switch pr {
	case ProtoBoth:
		return "TCP + UDP"
	case ProtoTCP:
		return "TCP"
	case ProtoUDP:
		return "UDP"
	default:
		return "Invalid proto"
	}
}

type Client struct {
	Proto  Protoc         // Protocol to close (proto.ProtoTCP, proto.ProtoUDP or proto.ProtoBoth)
	Client netip.AddrPort // Client address and port
}

type ClientData struct {
	Client Client // Client Destination
	Data   []byte `json:"-"` // Bytes to send
}

// Return pointer to value
func Point[T any](val T) *T { return &val }
