package bigendian

import (
	"encoding/binary"
	"io"
)

func readStream[T any](r io.Reader) (data T, err error) {
	err = binary.Read(r, binary.BigEndian, &data)
	return
}
func writeStream[T any](w io.Writer, data T) error {
	return binary.Write(w, binary.BigEndian, data)
}

func WriteBytes[T any](w io.Writer, value T) error {
	return binary.Write(w, binary.BigEndian, value)
}
func ReaderBytes[T any](r io.Reader, value T, Size uint64) error {
	return binary.Read(r, binary.BigEndian, value)
}

func ReadBytesN(r io.Reader, Size uint64) ([]byte, error) {
	buff := make([]byte, Size)
	if err := binary.Read(r, binary.BigEndian, buff); err != nil {
		return nil, err
	}
	return buff, nil
}

func ReadInt8(r io.Reader) (int8, error) {
	return readStream[int8](r)
}
func WriteInt8(w io.Writer, value int8) error {
	return writeStream[int8](w, value)
}

func ReadInt16(r io.Reader) (int16, error) {
	return readStream[int16](r)
}
func WriteInt16(w io.Writer, value int16) error {
	return writeStream[int16](w, value)
}

func ReadInt32(r io.Reader) (int32, error) {
	return readStream[int32](r)
}
func WriteInt32(w io.Writer, value int32) error {
	return writeStream[int32](w, value)
}

func ReadInt64(r io.Reader) (int64, error) {
	return readStream[int64](r)
}
func WriteInt64(w io.Writer, value int64) error {
	return writeStream[int64](w, value)
}

func ReadUint8(r io.Reader) (uint8, error) {
	return readStream[uint8](r)
}
func WriteUint8(w io.Writer, value uint8) error {
	return writeStream[uint8](w, value)
}

func ReadUint16(r io.Reader) (uint16, error) {
	return readStream[uint16](r)
}
func WriteUint16(w io.Writer, value uint16) error {
	return writeStream[uint16](w, value)
}

func ReadUint32(r io.Reader) (uint32, error) {
	return readStream[uint32](r)
}
func WriteUint32(w io.Writer, value uint32) error {
	return writeStream[uint32](w, value)
}

func ReadUint64(r io.Reader) (uint64, error) {
	return readStream[uint64](r)
}
func WriteUint64(w io.Writer, value uint64) error {
	return writeStream[uint64](w, value)
}
