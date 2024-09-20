package structcode

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	mathrand "math/rand/v2"
	"net/netip"
	"sync"
	"testing"
	"time"

	"sirherobrine23.org/Minecraft-Server/go-pproxit/proto"
)

type mapTest map[string]mapStr
type mapStr struct {
	Text string
	Blob []byte
}

func randomBuff(size int) []byte {
	buff := make([]byte, size)
	_, err := rand.Read(buff)
	if err != nil {
		panic(err)
	}
	return buff
}

func TestSerelelize(t *testing.T) {
	t.Run("Map", func(t *testing.T) {
		var err error
		var waiter sync.WaitGroup
		var enc, dec mapTest
		enc = mapTest{}
		enc["Test"] = mapStr{"Golang is best", []byte{5, 14, 22, 13}}
		for i := range mathrand.IntN(20) {
			enc["Rand"+fmt.Sprint(i)] = mapStr{string(randomBuff(14)), randomBuff(64)}
		}

		waiter.Add(2)
		r, w := io.Pipe()
		go func() {
			defer waiter.Done()
			if err = NewDecode(r, &dec); err != nil {
				t.Error(err)
				return
			}
		}()
		go func() {
			defer waiter.Done()
			if err = NewEncode(w, enc); err != nil {
				t.Error(err)
				return
			}
		}()
		waiter.Wait()
		if err != nil {
			return
		}
		for k, v := range enc {
			if d, ok := dec[k]; ok {
				if v.Text != d.Text {
					t.Errorf("text from decode not exists or mismatch (%q), Encode %q, Decode %q", k, v.Text, d.Text)
					return
				} else if !bytes.Equal(v.Blob, d.Blob) {
					t.Errorf("blob from decode not exists or mismatch (%q), Encode %s, Decode %s", k, hex.EncodeToString(v.Blob), hex.EncodeToString(d.Blob))
					return
				}
				continue
			}
			t.Errorf("key not exists in decode (%q)", k)
			return
		}
	})
	t.Run("Response", func(t *testing.T) {
		var err error
		var encodeRes, decodeRes proto.Response
		encodeRes.BadRequest = true
		encodeRes.NotListened = true
		encodeRes.SendAuth = true

		encodeRes.AgentInfo = &proto.AgentInfo{
			Protocol: 1,
			UDPPort:  2555,
			TCPPort:  3000,
			AddrPort: netip.MustParseAddrPort("[::]:10000"),
		}

		var waiter sync.WaitGroup
		waiter.Add(2)
		r, w := io.Pipe()
		go func() {
			defer waiter.Done()
			if err = NewDecode(r, &decodeRes); err != nil {
				t.Error(err)
				return
			}
		}()
		go func() {
			defer waiter.Done()
			if err = NewEncode(w, encodeRes); err != nil {
				t.Error(err)
				return
			}
		}()
		waiter.Wait()
		if err != nil {
			return
		} else if decodeRes.BadRequest != encodeRes.BadRequest {
			t.Errorf("invalid decode/encode, Current values to BadRequest, Decode %v, Encode %v", decodeRes.BadRequest, encodeRes.BadRequest)
			return
		} else if decodeRes.NotListened != encodeRes.NotListened {
			t.Errorf("invalid decode/encode, Current values to NotListened, Decode %v, Encode %v", decodeRes.NotListened, encodeRes.NotListened)
			return
		} else if decodeRes.SendAuth != encodeRes.SendAuth {
			t.Errorf("invalid decode/encode, Current values to SendAuth, Decode %v, Encode %v", decodeRes.SendAuth, encodeRes.SendAuth)
			return
		} else if decodeRes.AgentInfo == nil {
			t.Errorf("invalid decode, Current values to AgentInfo, Decode %+v, Encode %+v", decodeRes.AgentInfo, encodeRes.AgentInfo)
			return
		} else if decodeRes.AgentInfo.Protocol != encodeRes.AgentInfo.Protocol {
			t.Errorf("invalid decode/encode, Current values to AgentInfo.Protocol, Decode %d, Encode %d", decodeRes.AgentInfo.Protocol, encodeRes.AgentInfo.Protocol)
			return
		} else if decodeRes.AgentInfo.TCPPort != encodeRes.AgentInfo.TCPPort {
			t.Errorf("invalid decode/encode, Current values to AgentInfo.TCPPort, Decode %d, Encode %d", decodeRes.AgentInfo.TCPPort, encodeRes.AgentInfo.TCPPort)
		} else if decodeRes.AgentInfo.UDPPort != encodeRes.AgentInfo.UDPPort {
			t.Errorf("invalid decode/encode, Current values to AgentInfo.UDPPort, Decode %d, Encode %d", decodeRes.AgentInfo.UDPPort, encodeRes.AgentInfo.UDPPort)
			return
		} else if decodeRes.AgentInfo.AddrPort.Compare(encodeRes.AgentInfo.AddrPort) != 0 {
			t.Errorf("invalid decode/encode, Current values to AgentInfo.AddrPort, Decode %s, Encode %s", decodeRes.AgentInfo.AddrPort, encodeRes.AgentInfo.AddrPort)
			return
		}
	})
	t.Run("Request", func(t *testing.T) {
		var err error
		var encodeRequest, decodeRequest proto.Request
		encodeRequest.AgentAuth = &[]byte{0, 0, 1, 1, 1, 1, 1, 0, 255}
		encodeRequest.Ping = new(time.Time)
		*encodeRequest.Ping = time.Now()

		var waiter sync.WaitGroup
		waiter.Add(2)
		r, w := io.Pipe()
		go func() {
			defer waiter.Done()
			if err = NewEncode(w, encodeRequest); err != nil {
				t.Error(err)
				return
			}
		}()
		go func() {
			defer waiter.Done()
			if err = NewDecode(r, &decodeRequest); err != nil {
				t.Error(err)
				return
			}
		}()
		waiter.Wait()
		if err != nil {
			return
		} else if decodeRequest.Ping.Unix() != encodeRequest.Ping.Unix() {
			t.Errorf("cannot decode/encode Ping date, Decode %d, Encode: %d", decodeRequest.Ping.Unix(), encodeRequest.Ping.Unix())
			return
		} else if !bytes.Equal(*decodeRequest.AgentAuth, *encodeRequest.AgentAuth) {
			t.Errorf("cannot decode/encode auth data, Decode %q, Encode: %q", hex.EncodeToString(*decodeRequest.AgentAuth), hex.EncodeToString(*encodeRequest.AgentAuth))
			return
		}
	})
}
