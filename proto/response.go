package proto

import (
	"net/netip"
	"time"
)

type AgentInfo struct {
	Protocol         Protoc         // Proto supported (proto.ProtoTCP, proto.ProtoUDP or proto.ProtoBoth)
	UDPPort, TCPPort uint16         // Controller port listened
	AddrPort         netip.AddrPort // Address and port
	ConnectToken     []byte         // Bytes to conenct
}

// Reader data from Controller and process in agent
type Response struct {
	Unauthorized bool            `json:",omitempty"` // Controller reject connection
	SendAuth     bool            `json:",omitempty"` // Send Agent token
	BadRequest   bool            `json:",omitempty"` // Controller accepted packet so cannot process Request
	NotListened  bool            `json:",omitempty"` // Controller cannot Listen port
	AgentInfo    *AgentInfo      `json:",omitempty"` // Agent Info
	Pong         *time.Time      `json:",omitempty"` // ping response
	ConnectTo    *netip.AddrPort `json:",omitempty"` // Connect to dial and auth
	CloseClient  *Client         `json:",omitempty"` // Controller end client
	DataRX       *ClientData     `json:",omitempty"` // Controller recive data from client
	Error        error           `json:",omitempty"` // Error
}
