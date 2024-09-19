package structcode

import (
	"encoding"
	"encoding/binary"
	"io"
	"reflect"
)

func encodeRecursive(w io.Writer, reflectValue reflect.Value) error {
	switch reflectValue.Type().Kind() {
	case reflect.Interface:
	case reflect.String:
		str := reflectValue.String()
		if err := binary.Write(w, binary.BigEndian, int64(len(str))); err != nil {
			return err
		} else if _, err := w.Write([]byte(str)); err != nil {
			return err
		}
	case reflect.Bool, reflect.Float32, reflect.Float64, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return binary.Write(w, binary.BigEndian, reflectValue.Interface())
	case reflect.Struct:
		if reflectValue.Type().Implements(typeofBinMarshal) || reflectValue.Type().Implements(typeofTextMarshal) {
			var err error
			var data []byte
			if reflectValue.Type().Implements(typeofBinMarshal) {
				data, err = reflectValue.Interface().(encoding.BinaryMarshaler).MarshalBinary()
			} else {
				data, err = reflectValue.Interface().(encoding.TextMarshaler).MarshalText()
			}
			if err == nil {
				if err = binary.Write(w, binary.BigEndian, int64(len(data))); err == nil {
					_, err = w.Write(data)
				}
			}
			return err
		}
		typeof := reflectValue.Type()
		for fieldIndex := range typeof.NumField() {
			if typeof.Field(fieldIndex).Tag.Get(selectorTagName) == "-" || !typeof.Field(fieldIndex).IsExported() {
				continue
			} else if err := encodeRecursive(w, reflectValue.Field(fieldIndex)); err != nil {
				return err
			}
		}
	case reflect.Pointer:
		if reflectValue.IsNil() || reflectValue.IsZero() {
			return binary.Write(w, binary.BigEndian, int8(0))
		} else if err := binary.Write(w, binary.BigEndian, int8(1)); err != nil {
			return err
		}
		return encodeRecursive(w, reflectValue.Elem())
	case reflect.Array:
		for arrIndex := range reflectValue.Len() {
			if err := encodeRecursive(w, reflectValue.Index(arrIndex)); err != nil {
				return err
			}
		}
	case reflect.Slice:
		if err := binary.Write(w, binary.BigEndian, int64(reflectValue.Len())); err != nil {
			return err
		} else if reflectValue.Type().Elem().Kind() == typeofByte.Kind() {
			_, err = w.Write(reflectValue.Bytes())
			return err
		}
		for sliceIndex := range reflectValue.Len() {
			if err := encodeRecursive(w, reflectValue.Index(sliceIndex)); err != nil {
				return err
			}
		}
	}
	return nil
}
