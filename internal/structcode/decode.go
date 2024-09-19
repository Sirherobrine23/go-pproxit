package structcode

import (
	"encoding"
	"encoding/binary"
	"io"
	"reflect"
	"time"
)

func decodeRecursive(r io.Reader, reflectValue reflect.Value) error {
	switch reflectValue.Type().Kind() {
	case reflect.Interface:
	case reflect.String:
		size := int64(0)
		if err := binary.Read(r, binary.BigEndian, &size); err != nil {
			return err
		}
		buff := make([]byte, size)
		if _, err := r.Read(buff); err != nil {
			return err
		}
		reflectValue.SetString(string(buff))
	case reflect.Bool, reflect.Float32, reflect.Float64, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		data := reflect.New(reflectValue.Type()).Interface()
		if err := binary.Read(r, binary.BigEndian, data); err != nil {
			return err
		}
		reflectValue.Set(reflect.ValueOf(data).Elem())
	case reflect.Struct:
		if reflectValue.Type().ConvertibleTo(typeofTimer) || reflectValue.Type().Implements(typeofBinUnmarshal) || reflectValue.Type().Implements(typeofTextUnmarshal) {
			size := int64(0)
			if err := binary.Read(r, binary.BigEndian, &size); err != nil {
				return err
			}
			buff := make([]byte, size)
			if _, err := r.Read(buff); err != nil {
				return err
			} else if reflectValue.Type().ConvertibleTo(typeofTimer) {
				ttime := reflectValue.Interface().(time.Time)
				if err := ttime.UnmarshalBinary(buff); err != nil {
					return err
				}
				reflectValue.Set(reflect.ValueOf(ttime))
				return nil
			} else if reflectValue.Type().Implements(typeofBinUnmarshal) {
				data := reflectValue.Interface().(encoding.BinaryUnmarshaler)
				if err := data.UnmarshalBinary(buff); err != nil {
					return err
				}
				reflectValue.Set(reflect.ValueOf(data))
				return nil
			}
			data := reflectValue.Interface().(encoding.TextUnmarshaler)
			if err := data.UnmarshalText(buff); err != nil {
				return err
			}
			reflectValue.Set(reflect.ValueOf(data))
			return nil
		}

		typeof := reflectValue.Type()
		for fieldIndex := range typeof.NumField() {
			if typeof.Field(fieldIndex).Tag.Get(selectorTagName) == "-" || !typeof.Field(fieldIndex).IsExported() {
				continue
			} else if err := decodeRecursive(r, reflectValue.Field(fieldIndex)); err != nil {
				return err
			}
		}
	case reflect.Pointer:
		read := int8(0)
		if err := binary.Read(r, binary.BigEndian, &read); err != nil {
			return err
		} else if read == 0 {
			return nil
		}
		reflectValue.Set(reflect.New(reflectValue.Type().Elem()))
		return decodeRecursive(r, reflectValue.Elem())
	case reflect.Array:
		for arrIndex := range reflectValue.Len() {
			if err := decodeRecursive(r, reflectValue.Index(arrIndex)); err != nil {
				return err
			}
		}
	case reflect.Slice:
		size := int64(0)
		if err := binary.Read(r, binary.BigEndian, &size); err != nil {
			return err
		} else if reflectValue.Type().Elem().Kind() == typeofByte.Kind() {
			buff := make([]byte, size)
			if _, err = r.Read(buff); err != nil {
				return err
			}
			reflectValue.SetBytes(buff)
		} else {
			typeof := reflectValue.Type().Elem()
			for range size {
				newData := reflect.New(typeof)
				if err := decodeRecursive(r, newData); err != nil {
					return err
				}
				reflectValue.Set(reflect.AppendSlice(reflectValue, newData.Elem()))
			}
		}
	}
	return nil
}
