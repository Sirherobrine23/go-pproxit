package api

import (
	"net/netip"

	"github.com/google/uuid"
)

type AgentRouting struct {
	AgentID            uuid.UUID        `json:"agent"`       // Agent UUID
	AccountStatus      string           `json:"status"`      // Account status
	ControllersAddress []netip.AddrPort `json:"controllers"` // Controller address to connect
}

func (api *APIRequest) AgentRouting() (agent *AgentRouting, err error) {
	_, err = api.JSONRequest("/agent", "GET", &agent, nil, nil)
	return
}
