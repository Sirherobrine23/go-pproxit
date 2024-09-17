package proto

import (
	"errors"
	"time"
)

const (
	ReqAuth        uint64 = iota // Request Agent Auth
	ReqPing        uint64 = iota // Time ping
	ReqCloseClient uint64 = iota // Close client
	ReqClientData  uint64 = iota // Send data
)

var (
	ErrProtoBothNoSupported error = errors.New("protocol UDP+TCP not supported currently")
)

type AgentAuth [36]byte

// Send request to agent and wait response
type Request struct {
	AgentAuth   *AgentAuth  `json:",omitempty"` // Send agent authentication to controller
	Ping        *time.Time  `json:",omitempty"` // Send ping time to controller in unix milliseconds
	ClientClose *Client     `json:",omitempty"` // Close client in controller
	DataTX      *ClientData `json:",omitempty"` // Recive data from agent
}
