package proto

import (
	"net/netip"
	"time"
)

const (
	ResUnauthorized uint64 = iota // Request not processed and ignored
	ResBadRequest   uint64 = iota // Request cannot process and ignored
	ResCloseClient  uint64 = iota // Controller closed connection
	ResClientData   uint64 = iota // Controller accepted data
	ResSendAuth     uint64 = iota // Send token to controller
	ResAgentInfo    uint64 = iota // Agent info
	ResPong         uint64 = iota // Ping response
	ResNotListening uint64 = iota // Resize buffer size
)

type AgentInfo struct {
	Protocol         uint8          // Proto supported (proto.ProtoTCP, proto.ProtoUDP or proto.ProtoBoth)
	UDPPort, TCPPort uint16         // Controller port listened
	AddrPort         netip.AddrPort // request address and port
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
