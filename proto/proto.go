package proto

import (
	"errors"
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
	if err := bigendian.WriteUint8(w, close.Proto); err != nil {
		return err
	} else if addr.Is4() {
		if err := bigendian.WriteUint8(w, 4); err != nil {
			return err
		} else if err := bigendian.WriteBytes(w, addr.As4()); err != nil {
			return err
		}
	} else {
		if err := bigendian.WriteUint8(w, 6); err != nil {
			return err
		} else if err := bigendian.WriteBytes(w, addr.As16()); err != nil {
			return err
		}
	}
	if err := bigendian.WriteUint16(w, close.Client.Port()); err != nil {
		return err
	}
	return nil
}
func (close *Client) Reader(r io.Reader) (err error) {
	if close.Proto, err = bigendian.ReadUint8(r); err != nil {
		return
	} else if close.Proto, err = bigendian.ReadUint8(r); err != nil {
		return
	} else if close.Proto == ProtoBoth {
		return ErrProtoBothNoSupported
	}
	var addrFamily uint8
	var addrPort uint16
	var ipBytes []byte
	if addrFamily, err = bigendian.ReadUint8(r); err != nil {
		return
	} else if addrFamily == 4 {
		if ipBytes, err = bigendian.ReadBytesN(r, 4); err != nil {
			return
		}
	} else if addrFamily == 6 {
		if ipBytes, err = bigendian.ReadBytesN(r, 16); err != nil {
			return
		}
	}
	if addrPort, err = bigendian.ReadUint16(r); err != nil {
		return
	} else if len(ipBytes) == 16 {
		close.Client = netip.AddrPortFrom(netip.AddrFrom16([16]byte(ipBytes)), addrPort)
	} else {
		close.Client = netip.AddrPortFrom(netip.AddrFrom4([4]byte(ipBytes)), addrPort)
	}
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
	} else if _, err = r.Read(data.Data[0:data.Size]); err != nil {
		return
	}
	return
}