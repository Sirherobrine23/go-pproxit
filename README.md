# Golang pproxit to TCP and UDP Connections

Same to playit.gg, to make more simples to connections and implementations and host self server proxy

## Implementation

- [ ] API
  - [ ] Create tunnel (`/tunnel`)
  - [ ] Delete tunnel (`/tunnel/:id`)
  - [ ] Get server address to connect (`/tunnel/:id`)
  - [ ] Allocate tunnel ports (`/tunnel/:id/port`)
  - [ ] Remote tunnel ports (`/tunnel/:id/port`)
- [ ] Control server
  - [ ] Connect and Authentication
  - [ ] UDP Client
  - [ ] TCP Client
  - [ ] Client
    - [ ] Message with to new client
    - [ ] Re-transmit packet to client
    - [ ] Send packet to control

### Packets

All packets send in UDP Socket with `2048` size with `big-endian` encoding.