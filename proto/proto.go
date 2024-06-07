package proto

import (
	"errors"
	"fmt"
	"io"
	"net/netip"

	"sirherobrine23.org/Minecraft-Server/go-pproxit/internal/bigendian"
)

const (
	ProtoTCP  uint8 = 1 // TCP Protocol
	ProtoUDP  uint8 = 2 // UDP Protocol
	ProtoBoth uint8 = 3 // TCP+UDP Protocol
)

var (
	ErrInvalidBody error = errors.New("invalid body, check request/response")
)


type Client struct {
	Client netip.AddrPort // Client address and port
	Proto  uint8          // Protocol to close (proto.ProtoTCP, proto.ProtoUDP or proto.ProtoBoth)
}

func (close Client) Writer(w io.Writer) error {
	addr := close.Client.Addr()
	if !addr.IsValid() {
		return fmt.Errorf("invalid ip address")
	}

	var family uint8 = 6
	if addr.Is4() {
		family = 4
	}

	if err := bigendian.WriteUint8(w, close.Proto); err != nil {
		return err
	} else if err := bigendian.WriteUint8(w, family); err != nil {
		return err
	} else if err := bigendian.WriteBytes(w, addr.AsSlice()); err != nil {
		return err
	} else if err := bigendian.WriteUint16(w, close.Client.Port()); err != nil {
		return err
	}
	return nil
}
func (close *Client) Reader(r io.Reader) (err error) {
	if close.Proto, err = bigendian.ReadUint8(r); err != nil {
		return
	}
	family, err := bigendian.ReadUint8(r)
	if err != nil {
		return err
	}

	var addr netip.Addr
	if family == 4 {
		buff, err := bigendian.ReadBytesN(r, 4)
		if err != nil {
			return err
		}
		addr = netip.AddrFrom4([4]byte(buff))
	} else {
		buff, err := bigendian.ReadBytesN(r, 16)
		if err != nil {
			return err
		}
		addr = netip.AddrFrom16([16]byte(buff))
	}

	port, err := bigendian.ReadUint16(r)
	if err != nil {
		return err
	}
	close.Client = netip.AddrPortFrom(addr, port)
	return
}

type ClientData struct {
	Client Client // Client Destination
	Size   uint64 // Data size
	Data   []byte // Bytes to send
}

func (data ClientData) Writer(w io.Writer) error {
	if err := data.Client.Writer(w); err != nil {
		return err
	} else if err := bigendian.WriteUint64(w, data.Size); err != nil {
		return err
	} else if _, err := w.Write(data.Data[:data.Size]); err != nil { // Write data without convert to big-endian
		return err
	}
	return nil
}
func (data *ClientData) Reader(r io.Reader) (err error) {
	if err = data.Client.Reader(r); err != nil {
		return
	} else if data.Size, err = bigendian.ReadUint64(r); err != nil {
		return
	}

	data.Data = make([]byte, data.Size)
	if _, err = r.Read(data.Data); err != nil {
		return
	}
	return
}