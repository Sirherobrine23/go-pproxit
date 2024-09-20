package structcode

import (
	"encoding"
	"encoding/binary"
	"io"
	"reflect"
)

func writeBuff(w io.Writer, buff []byte) error {
	if err := binary.Write(w, binary.BigEndian, uint32(len(buff))); err != nil {
		return err
	}
	_, err := w.Write(buff)
	return err
}

func encodeTypeof(w io.Writer, reflectValue reflect.Value) (bool, error) {
	var err error = nil
	var data []byte
	switch {
	default:
		return false, nil
	case reflectValue.Type().Implements(typeofBinMarshal), reflectValue.Type().ConvertibleTo(typeofBinMarshal):
		data, err = reflectValue.Interface().(encoding.BinaryMarshaler).MarshalBinary()
	case reflectValue.Type().Implements(typeofTextMarshal), reflectValue.Type().ConvertibleTo(typeofTextMarshal):
		data, err = reflectValue.Interface().(encoding.TextMarshaler).MarshalText()
	}
	if err == nil {
		err = writeBuff(w, data)
	}
	return true, err
}

func encodeRecursive(w io.Writer, reflectValue reflect.Value) error {
	switch reflectValue.Type().Kind() {
	case reflect.String:
		return writeBuff(w, []byte(reflectValue.String()))
	case reflect.Bool, reflect.Float32, reflect.Float64, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return binary.Write(w, binary.BigEndian, reflectValue.Interface())
	case reflect.Interface:
	case reflect.Map:
		if err := binary.Write(w, binary.BigEndian, uint64(reflectValue.Len())); err != nil {
			return err
		}
		inter := reflectValue.MapRange()
		for inter.Next() {
			if err := encodeRecursive(w, inter.Key()); err != nil {
				return err
			} else if err := encodeRecursive(w, inter.Value()); err != nil {
				return err
			}
		}
	case reflect.Struct:
		if ok, err := encodeTypeof(w, reflectValue); ok {
			return err
		}
		typeof := reflectValue.Type()
		for fieldIndex := range typeof.NumField() {
			fieldType := typeof.Field(fieldIndex)
			if fieldType.Tag.Get(selectorTagName) == "-" || !fieldType.IsExported() {
				continue
			} else if err := encodeRecursive(w, reflectValue.Field(fieldIndex)); err != nil {
				return err
			}
		}
	case reflect.Pointer:
		if reflectValue.IsNil() || reflectValue.IsZero() {
			return binary.Write(w, binary.BigEndian, false)
		} else if err := binary.Write(w, binary.BigEndian, true); err != nil {
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
		if reflectValue.Type().ConvertibleTo(typeofBytes) {
			return writeBuff(w, reflectValue.Bytes())
		} else if err := binary.Write(w, binary.BigEndian, int64(reflectValue.Len())); err != nil {
			return err
		} else if reflectValue.Type().Elem().Kind() == typeofBytes.Elem().Kind() {
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
