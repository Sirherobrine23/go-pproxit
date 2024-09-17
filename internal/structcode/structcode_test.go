package structcode

import (
	"bytes"
	"testing"
	"time"
)

type structTest struct {
	Text  string
	Int8  int8
	Int16 int16
	Int32 int32
	Int64 int64

	Bytes      []byte
	ArrayBytes [4]byte

	Date     time.Time
	Pointer  *structTest
	Pointer2 any
}

func TestSerelelize(t *testing.T) {
	var testData structTest
	testData.Text = "Golang is best"
	testData.Int8 = 2
	testData.Int16 = 200
	testData.Int32 = 1_000_000
	testData.Int64 = 1024 * 12 * 12
	testData.Bytes = []byte("google maintener go")
	testData.ArrayBytes = [4]byte{0, 1, 1, 1}
	testData.Date = time.Now()
	testData.Pointer = &structTest{
		Text: "Golang",
	}

	buff := new(bytes.Buffer)
	if err := NewEncode(buff, &testData); err != nil {
		t.Error(err)
		return
	}
}

func TestDeserelelize(t *testing.T) {
	var testData structTest
	testData.Text = "Golang is best"
	testData.Int8 = 2
	testData.Int16 = 200
	testData.Int32 = 1_000_000
	testData.Int64 = 1024 * 12 * 12
	testData.Bytes = []byte("google maintener go")
	testData.ArrayBytes = [4]byte{0, 1, 1, 1}
	testData.Date = time.Now()
	testData.Pointer = &structTest{
		Text: "Golang",
	}

	buff := new(bytes.Buffer)
	if err := NewEncode(buff, &testData); err != nil {
		t.Error(err)
		return
	}
	var decodeTest structTest
	if err := NewDecode(buff, &decodeTest); err != nil {
		t.Error(err)
		return
	}
}
