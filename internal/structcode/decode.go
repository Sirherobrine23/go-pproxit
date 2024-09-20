package structcode

import (
	"encoding"
	"encoding/binary"
	"io"
	"reflect"
)

func readBuff(r io.Reader) ([]byte, error) {
	size := uint32(0)
	if err := binary.Read(r, binary.BigEndian, &size); err != nil {
		return nil, err
	}
	buff := make([]byte, size)
	_, err := r.Read(buff)
	return buff, err
}

func decodeTypeof(r io.Reader, reflectValue reflect.Value) (bool, error) {
	var data any
	npoint := reflect.New(reflectValue.Type())
	typeof := npoint.Type()
	switch {
	default:
		return false, nil
	case typeof.Implements(typeofBinUnmarshal), typeof.ConvertibleTo(typeofBinUnmarshal):
		buff, err := readBuff(r)
		if err != nil {
			return true, err
		}
		t := npoint.Interface()
		if err := t.(encoding.BinaryUnmarshaler).UnmarshalBinary(buff); err != nil {
			return true, err
		}
		data = t
	case typeof.Implements(typeofTextUnmarshal), typeof.ConvertibleTo(typeofTextUnmarshal):
		buff, err := readBuff(r)
		if err != nil {
			return true, err
		}
		t := npoint.Interface()
		if err := t.(encoding.TextUnmarshaler).UnmarshalText(buff); err != nil {
			return true, err
		}
		data = t
	}
	reflectValue.Set(reflect.ValueOf(data).Elem())
	return true, nil
}

func decodeRecursive(r io.Reader, reflectValue reflect.Value) error {
	switch reflectValue.Type().Kind() {
	case reflect.String:
		buff, err := readBuff(r)
		if err != nil {
			return err
		}
		reflectValue.SetString(string(buff))
	case reflect.Bool, reflect.Float32, reflect.Float64, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		data := reflect.New(reflectValue.Type()).Interface()
		if err := binary.Read(r, binary.BigEndian, data); err != nil {
			return err
		}
		reflectValue.Set(reflect.ValueOf(data).Elem())
	case reflect.Interface:
	case reflect.Map:
		mapTypeof := reflectValue.Type()
		reflectValue.Set(reflect.MakeMap(mapTypeof))
		var size uint64
		if err := binary.Read(r, binary.BigEndian, &size); err != nil {
			return err
		}
		for range size {
			key, value := reflect.New(mapTypeof.Key()).Elem(), reflect.New(mapTypeof.Elem()).Elem()
			if err := decodeRecursive(r, key); err != nil {
				return err
			} else if err := decodeRecursive(r, value); err != nil {
				return err
			}
			reflectValue.SetMapIndex(key, value)
		}
	case reflect.Struct:
		if ok, err := decodeTypeof(r, reflectValue); ok {
			return err
		}
		typeof := reflectValue.Type()
		for fieldIndex := range typeof.NumField() {
			fieldType := typeof.Field(fieldIndex)
			if fieldType.Tag.Get(selectorTagName) == "-" || !fieldType.IsExported() {
				continue
			} else if err := decodeRecursive(r, reflectValue.Field(fieldIndex)); err != nil {
				return err
			}
		}
	case reflect.Pointer:
		var read bool
		if err := binary.Read(r, binary.BigEndian, &read); err != nil {
			return err
		} else if read {
			reflectValue.Set(reflect.New(reflectValue.Type().Elem()))
			return decodeRecursive(r, reflectValue.Elem())
		}
	case reflect.Array:
		for arrIndex := range reflectValue.Len() {
			if err := decodeRecursive(r, reflectValue.Index(arrIndex)); err != nil {
				return err
			}
		}
	case reflect.Slice:
		if reflectValue.Type().ConvertibleTo(typeofBytes) {
			buff, err := readBuff(r)
			if err != nil {
				return err
			}
			reflectValue.SetBytes(buff)
			return nil
		}
		size := int64(0)
		if err := binary.Read(r, binary.BigEndian, &size); err != nil {
			return err
		}
		typeof := reflectValue.Type().Elem()
		for range size {
			newData := reflect.New(typeof)
			if err := decodeRecursive(r, newData); err != nil {
				return err
			}
			reflectValue.Set(reflect.AppendSlice(reflectValue, newData.Elem()))
		}
	}
	return nil
}
