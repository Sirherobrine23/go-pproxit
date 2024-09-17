package structcode

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"testing"
	"time"
)

type testBase struct {
	Text string
	Date time.Time
	Body []byte
	T1 float32
	T2 int8
	T3 bool
	T4 *testBase
}

func TestDecodeEncode(t *testing.T) {
	var v1, v2 testBase
	v1.Text = "Golang is best programer language"
	v1.Body = make([]byte, 20)
	rand.Read(v1.Body)
	v1.Date = time.Now()
	v1.T1 = 0.2
	v1.T2 = 1
	v1.T4 = &testBase{
		Text: "Test t4",
	}

	v1Body, err := Marshall(&v1)
	if err != nil {
		t.Error(err)
		return
	}
	t.Log("Unsmarhall")
	t.Logf("Data: %q", base64.StdEncoding.EncodeToString(v1Body))

	if err := Unmashall(v1Body, &v2); err != nil {
		t.Error(err)
		return
	} else if v2.Text != v1.Text {
		t.Errorf("invalid unmarshall data, current %q, accept %q", v2.Text, v1.Text)
		return
	} else if !bytes.Equal(v2.Body, v1.Body) {
		t.Errorf("invalid unmarshall data, current %q, accept %q", hex.EncodeToString(v2.Body), hex.EncodeToString(v1.Body))
		return
	} else if v2.T1 != v1.T1 {
		t.Errorf("invalid unmarshall data, current %f, accept %f", v2.T1, v1.T1)
		return
	} else if v2.T2 != v1.T2 {
		t.Errorf("invalid unmarshall data, current %d, accept %d", v2.T2, v1.T2)
		return
	} else if v2.T3 != v1.T3 {
		t.Errorf("invalid unmarshall data, current %v, accept %v", v2.T3, v1.T3)
		return
	} else if v2.Date.UnixMilli() != v1.Date.UnixMilli() {
		t.Errorf("invalid unmarshall data, current %d, accept %d", v2.Date.UnixMilli(), v1.Date.UnixMilli())
		return
	} else if v2.T4.Text != v1.T4.Text {
		t.Errorf("invalid unmarshall data, current %s, accept %s", v2.T4.Text, v1.T4.Text)
		return
	}
}
