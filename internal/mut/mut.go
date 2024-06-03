package mut

import (
	"encoding/binary"
	"io"
)

func readStream[T any](r io.Reader) (data T, err error) {
	err = binary.Read(r, binary.BigEndian, &data)
	return
}

func ReadInt8(r io.Reader) (int8, error) {
	return readStream[int8](r)
}
func ReadInt16(r io.Reader) (int16, error) {
	return readStream[int16](r)
}
func ReadInt32(r io.Reader) (int32, error) {
	return readStream[int32](r)
}
func ReadInt64(r io.Reader) (int64, error) {
	return readStream[int64](r)
}

func ReadUint8(r io.Reader) (uint8, error) {
	return readStream[uint8](r)
}
func ReadUint16(r io.Reader) (uint16, error) {
	return readStream[uint16](r)
}
func ReadUint32(r io.Reader) (uint32, error) {
	return readStream[uint32](r)
}
func ReadUint64(r io.Reader) (uint64, error) {
	return readStream[uint64](r)
}