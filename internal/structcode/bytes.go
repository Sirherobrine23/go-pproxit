package structcode

import (
	"bytes"
)

func Marshal(target any) ([]byte, error) {
	buff := new(bytes.Buffer)
	err := NewEncode(buff, target)
	return buff.Bytes(), err
}

func Unmarshal(b []byte, target any) error {
	return NewDecode(bytes.NewReader(b), target)
}
