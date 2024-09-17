package structcode

import (
	"encoding"
	"encoding/binary"
	"fmt"
	"io"
	"reflect"
)

/*
Este bloco de codigo server apenas para facilitar minha vida no mundo do go
Definindo a estrutura de seguinte maneira, os dados estão fortemente ligado as structs go então qualquer merda aqui pode ferra com qualquer versão anterior, então isso não será recomendado para algumas coisa
Os dados serão escritos em BigEndian, então tenha cuidado com os dados inseridos casos sejam inportantes
Os pointers serão verificados se são nil's para idicar para a sereliazação e desereliazação

*any               -> int8(0|1) + data...
[]any              -> int64 (Size) + data...
map[any]any        -> int64 (Size) + (key + data)...
[]bytes	Or String  -> Int64 (Size) + []bytes
int64, uint64      -> int64
int32, uint32      -> int32
int16, uint16      -> int16
int8, uint8        -> int8
bool					     -> int8(0|1)
*/

const selectorTagName = "ser"

var typeofBytes = reflect.TypeOf([]byte{})

func NewEncode(w io.Writer, target any) error {
	if target == nil {
		return binary.Write(w, binary.BigEndian, int8(0))
	}
	targetReflect := reflect.ValueOf(target)
	switch targetReflect.Type().Kind() {
	case reflect.Array, reflect.Slice:
		if err := binary.Write(w, binary.BigEndian, int64(targetReflect.Len())); err != nil {
			return err
		}

		switch targetReflect.Type().Elem().Kind() {
		case typeofBytes.Elem().Kind(): // Check if the element is a byte type
			buff := make([]byte, targetReflect.Len())
			for i := range targetReflect.Len() {
				buff[i] = targetReflect.Index(i).Interface().(byte)
			}
			_, err := w.Write(buff)
			return err
		default:
			for i := range targetReflect.Len() {
				if err := NewEncode(w, targetReflect.Index(i).Interface()); err != nil {
					return err
				}
			}
		}
	case reflect.Pointer:
		return NewEncode(w, targetReflect.Elem().Interface()) // Ignore point and reencode
	case reflect.Struct:
		BinaryMarshaler, isBinaryMarshaler := targetReflect.Interface().(encoding.BinaryMarshaler)
		TextMarshaler, isTextMarshaler := targetReflect.Interface().(encoding.TextMarshaler)
		if isBinaryMarshaler || isTextMarshaler {
			var data []byte
			var err error
			if isBinaryMarshaler {
				data, err = BinaryMarshaler.MarshalBinary()
			} else {
				data, err = TextMarshaler.MarshalText()
			}
			if err != nil {
				return err
			}
			return NewEncode(w, data)
		}

		typeof := targetReflect.Type()
		for i := range targetReflect.NumField() {
			if tag := typeof.Field(i).Tag.Get(selectorTagName); tag == "-" || !targetReflect.Field(i).IsValid() {
				continue
			} else if targetReflect.Field(i).Type().Kind() == reflect.Pointer {
				if targetReflect.IsZero() || !targetReflect.CanInterface() || targetReflect.Field(i).IsNil() {
					return binary.Write(w, binary.BigEndian, int8(0))
				} else if err := binary.Write(w, binary.BigEndian, int8(1)); err != nil {
					return err
				} else if err := NewEncode(w, targetReflect.Field(i).Elem().Interface()); err != nil {
					return err
				}
				continue
			}
			if err := NewEncode(w, targetReflect.Field(i).Interface()); err != nil {
				return err
			}
		}
	case reflect.String:
		if err := binary.Write(w, binary.BigEndian, int64(targetReflect.Len())); err != nil {
			return err
		}
		_, err := w.Write([]byte(targetReflect.String()))
		return err
	case reflect.Bool:
		if targetReflect.Bool() {
			return binary.Write(w, binary.BigEndian, int8(1))
		}
		return binary.Write(w, binary.BigEndian, int8(0))
	case reflect.Uint8, reflect.Int8, reflect.Int16, reflect.Uint16, reflect.Int32, reflect.Uint32, reflect.Int64, reflect.Uint64, reflect.Float32, reflect.Float64:
		return binary.Write(w, binary.BigEndian, target)
	default:
		return fmt.Errorf("set valid struct or array/slice, or any primary values")
	}

	return nil
}

func NewDecode(r io.Reader, target any) error {
	if target == nil {
		return binary.Read(r, binary.BigEndian, int8(0))
	}
	targetReflect := reflect.ValueOf(target).Elem()
	switch targetReflect.Type().Kind() {
	case reflect.Slice:
		var size int64
		if err := binary.Read(r, binary.BigEndian, &size); err != nil {
			return err
		}

		switch targetReflect.Type().Elem().Kind() {
		case typeofBytes.Elem().Kind():
			buff := make([]byte, size)
			if _, err := r.Read(buff); err != nil {
				return err
			}
			targetReflect.SetBytes(buff)
		default:
			for i := range int(size) {
				data := reflect.New(targetReflect.Field(i).Type().Elem()).Interface()
				if err := NewDecode(r, data); err != nil {
					return err
				}
				targetReflect.Set(reflect.AppendSlice(targetReflect, reflect.ValueOf(data)))
			}
		}
		return nil
	case reflect.Array:
		var size int64
		if err := binary.Read(r, binary.BigEndian, &size); err != nil {
			return err
		} else if size != int64(targetReflect.Len()) {
			return fmt.Errorf("size mismatch: expected %d, got %d", targetReflect.Len(), size)
		}
		switch targetReflect.Type().Elem().Kind() {
		case typeofBytes.Elem().Kind(): // Check if the element is a byte type
			buff := make([]byte, size)
			if _, err := r.Read(buff); err != nil {
				return err
			}
			for i, data := range buff {
				targetReflect.Index(i).Set(reflect.ValueOf(data))
			}
		default:
			for i := 0; i < int(size); i++ {
				elem := reflect.New(targetReflect.Type().Elem()).Interface()
				if err := NewDecode(r, elem); err != nil {
					return err
				}
				targetReflect.Index(i).Set(reflect.ValueOf(elem).Elem())
			}
		}
		return nil
	case reflect.String:
		var err error
		var size int64
		if err = binary.Read(r, binary.BigEndian, &size); err == nil {
			buff := make([]byte, size)
			if _, err = r.Read(buff); err == nil {
				targetReflect.SetString(string(buff))
			}
		}
		return err
	case reflect.Bool:
		var boolInt int8
		if err := binary.Read(r, binary.BigEndian, &boolInt); err != nil {
			return err
		}
		targetReflect.SetBool(boolInt == 1)
	case reflect.Uint8, reflect.Int8, reflect.Int16, reflect.Uint16, reflect.Int32, reflect.Uint32, reflect.Int64, reflect.Uint64, reflect.Float32, reflect.Float64:
		return binary.Read(r, binary.BigEndian, target)
	case reflect.Struct:
		BinaryMarshaler, isBinaryMarshaler := target.(encoding.BinaryUnmarshaler)
		TextMarshaler, isTextMarshaler := target.(encoding.TextUnmarshaler)
		if isBinaryMarshaler || isTextMarshaler {
			var data []byte
			var err error
			if err = NewDecode(r, &data); err != nil {
				return err
			} else if isBinaryMarshaler {
				return BinaryMarshaler.UnmarshalBinary(data)
			}
			return TextMarshaler.UnmarshalText(data)
		}
		for i := range targetReflect.NumField() {
			if tag := targetReflect.Type().Field(i).Tag.Get(selectorTagName); tag == "-" || !targetReflect.Field(i).CanSet() {
				continue
			}

			if targetReflect.Field(i).Type().Kind() == reflect.Pointer {
				var ok bool
				if err := NewDecode(r, &ok); err != nil {
					return err
				} else if ok {
					data := reflect.New(targetReflect.Field(i).Type().Elem()).Interface()
					if err := NewDecode(r, data); err != nil {
						return err
					}
					targetReflect.Field(i).Set(reflect.ValueOf(data))
				}
				continue
			}
			data := reflect.New(targetReflect.Field(i).Type()).Interface()
			if err := NewDecode(r, data); err != nil {
				return err
			}
			targetReflect.Field(i).Set(reflect.ValueOf(data).Elem())
		}
	case reflect.Interface:
		BinaryMarshaler, isBinaryMarshaler := target.(encoding.BinaryUnmarshaler)
		TextMarshaler, isTextMarshaler := target.(encoding.TextUnmarshaler)
		if isBinaryMarshaler || isTextMarshaler {
			var data []byte
			var err error
			if err = NewDecode(r, &data); err != nil {
				return err
			} else if isBinaryMarshaler {
				return BinaryMarshaler.UnmarshalBinary(data)
			}
			return TextMarshaler.UnmarshalText(data)
		}
	default:
		return fmt.Errorf("set valid struct or array/slice, or any primary values, kind %s", targetReflect.Type().Kind())
	}
	return nil
}
