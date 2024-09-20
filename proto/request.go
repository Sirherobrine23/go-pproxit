package proto

import (
	"errors"
	"time"
)

var ErrProtoBothNoSupported error = errors.New("protocol UDP+TCP not supported currently")

// Send request to agent and wait response
type Request struct {
	Ping        *time.Time  `json:",omitempty"` // Send ping time to controller in unix milliseconds
	AgentAuth   *[]byte     `json:",omitempty"` // Send agent authentication to controller
	ClientClose *Client     `json:",omitempty"` // Close client in controller
	DataTX      *ClientData `json:",omitempty"` // Recive data from agent
}
