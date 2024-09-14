package structcode

import (
	"encoding/binary"
	"fmt"
	"io"
	"reflect"
)

func NewDecode(r io.Reader, body any) error {
	if body == nil {
		return nil
	}

	structType := reflect.TypeOf(body)
	if structType.Kind() != reflect.Ptr {
		return fmt.Errorf("must pass a pointer")
	}
	elem := structType.Elem()
	kind := elem.Kind()
	value := reflect.ValueOf(body)
	if kind == reflect.Struct {
		structurePointer := value.Elem()
		for i := range structurePointer.NumField() {
			field := elem.Field(i)
			fieldPointer := structurePointer.FieldByName(field.Name)
			kind := field.Type.Kind()
			switch kind {
			default:
				newValue := reflect.New(fieldPointer.Type()).Interface()
				if err := NewDecode(r, newValue); err != nil {
					fmt.Printf("unable to convert value to [%s][%s]\n", fieldPointer.Type().Kind(), field.Name)
					break
				}
				fieldPointer.Set(reflect.ValueOf(newValue).Elem())
			case reflect.Struct:
				nestedStruct := reflect.New(fieldPointer.Type()).Interface()
				if err := NewDecode(r, nestedStruct); err != nil {
					return err
				}
				fieldPointer.Set(reflect.ValueOf(nestedStruct).Elem())
			}
		}
	}

	valuePtr := value.Elem()
	switch kind {
	default:
		break
	case reflect.String:
		var size int64
		if err := binary.Read(r, binary.BigEndian, &size); err != nil {
			return err
		}
		buff := make([]byte, size)
		if err := binary.Read(r, binary.BigEndian, buff); err != nil {
			return err
		}
		valuePtr.Set(reflect.ValueOf(string(buff)).Elem())
	case reflect.Slice:
		if data, ok := reflect.New(valuePtr.Type()).Interface().([]byte); ok {
			size := int64(len(data))
			if size == 0 {
				if err := binary.Read(r, binary.BigEndian, &size); err != nil {
					return err
				}
			}
			if _, err := r.Read(data); err != nil {
				return err
			}
			valuePtr.Set(reflect.ValueOf(data).Elem())
			return nil
		}
	case
		reflect.Float32, reflect.Float64,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		newValue := reflect.New(valuePtr.Type()).Interface()
		if err := binary.Read(r, binary.BigEndian, newValue); err != nil {
			return err
		}
		valuePtr.Set(reflect.ValueOf(newValue).Elem())
	}

	return nil
}

func NewEncode(w io.Writer, body any) error {
	if body == nil {
		return nil
	}

	structType := reflect.TypeOf(body)
	if structType.Kind() != reflect.Ptr {
		return fmt.Errorf("must pass a pointer")
	}

	elem := structType.Elem()
	kind := elem.Kind()
	value := reflect.ValueOf(body)
	if kind == reflect.Struct {
		structurePointer := value.Elem()
		for i := range structurePointer.NumField() {
			field := elem.Field(i)
			fieldPointer := structurePointer.FieldByName(field.Name)
			kind := field.Type.Kind()
			switch kind {
			default:
				if err := NewEncode(w, fieldPointer.Interface()); err != nil {
					fmt.Printf("unable to convert value to [%s][%s]\n", fieldPointer.Type().Kind(), field.Name)
				}
			case reflect.Struct:
				if err := NewEncode(w, fieldPointer.Interface()); err != nil {
					return err
				}
			}
		}
	}

	valuePtr := value.Elem()
	switch kind {
	default:
		break
	case reflect.String, reflect.Slice:
		content := valuePtr.Bytes()
		if err := binary.Write(w, binary.BigEndian, int64(len(content))); err != nil {
			return err
		} else if err := binary.Write(w, binary.BigEndian, content); err != nil {
			return err
		}
	case
		reflect.Float32, reflect.Float64,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if err := binary.Write(w, binary.BigEndian, valuePtr.Interface()); err != nil {
			return err
		}
	}

	return nil
}
