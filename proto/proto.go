package proto

import (
	"errors"
	"net/netip"
)

const (
	ProtoBoth uint8 = iota // TCP+UDP Protocol
	ProtoTCP               // TCP Protocol
	ProtoUDP               // UDP Protocol

	DataSize       uint64 = 10_000                // Default listener data recive and send
	PacketSize     uint64 = 800                   // Packet to without data only requests and response headers
	PacketDataSize uint64 = DataSize + PacketSize // Header and Data request/response
)

var (
	ErrInvalidBody error = errors.New("invalid body, check request/response")
)

type Client struct {
	Client netip.AddrPort // Client address and port
	Proto  uint8          // Protocol to close (proto.ProtoTCP, proto.ProtoUDP or proto.ProtoBoth)
}

type ClientData struct {
	Client Client // Client Destination
	Size   uint64 // Data size
	Data   []byte `json:"-"` // Bytes to send
}
