# Golang pproxit to TCP and UDP Connections

Same to playit.gg, to make more simples to connections and implementations and host self server proxy, this implements only Server, for client Wrapper to same `net.Listener`

## Server

Controls all connections and connected clients, any request within the accepted posts will be forwarded to its current clients without any data filtering, only users that can be blocked

## Client

Connect to the controller and accept and manage local connections, quickly and dynamically, without having to specify the values

### Auth

As part of being simple, the controller can enable simple or even more complex authentication, and some data can also be recorded if there are some functions enabled by the Controller.