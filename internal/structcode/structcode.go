package structcode

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"
)

/*
Este bloco de codigo server apenas para facilitar minha vida no mundo do go
Definindo a estrutura de seguinte maneira, os dados estão fortemente ligado as structs go então qualquer merda aqui pode ferra com qualquer versão anterior, então isso não será recomendado para algumas coisa
Os dados serão escritos em BigEndian, então tenha cuidado com os dados inseridos casos sejam inportantes

time.Time              -> time.Time.UnixMilli()
[]bytes								 =  Int (Size) + []bytes
String								 -> Int (Size) + []bytes
bool									 -> 0/1
int, uint, int8, uint8 =  int
int32, uint32          =  int32
int64, uint64          =  int64
*/

var (
	typeofByte = reflect.TypeOf(([]byte{0})[0])
)

func NewEncode(w io.Writer, body any) error {
	if body == nil {
		return nil
	}

	elem := reflect.TypeOf(body).Elem()
	valuePtr := reflect.ValueOf(body).Elem()

	switch elem.Kind() {
	case reflect.Struct:
		for i := range valuePtr.NumField() {
			field := elem.Field(i)
			fieldPointer := valuePtr.FieldByName(field.Name)
			var err error
			switch v := fieldPointer.Interface().(type) {
			case bool, int8, uint8, int16, uint16, int32, uint32, int64, uint64, float32, float64:
				err = binary.Write(w, binary.BigEndian, v)
			case string:
				if err = binary.Write(w, binary.BigEndian, uint32(fieldPointer.Len())); err == nil {
					_, err = w.Write([]byte(v))
				}
			case time.Time:
				err = binary.Write(w, binary.BigEndian, v.UnixMicro())
			case []byte:
				if err = binary.Write(w, binary.BigEndian, uint32(len(v))); err == nil {
					_, err = w.Write(v)
				}
			default:
				if fieldPointer.IsNil() {
					if err = binary.Write(w, binary.BigEndian, uint8(0)); err != nil {
						return err
					}
					continue
				}
				if err = binary.Write(w, binary.BigEndian, uint8(1)); err != nil {
					return err
				}
				err = NewEncode(w, v)
			}
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func NewDecode(r io.Reader, body any) error {
	if body == nil {
		return nil
	}

	elem := reflect.TypeOf(body).Elem()
	valuePtr := reflect.ValueOf(body).Elem()
	fmt.Println(elem.String())

	switch elem.Kind() {
	case reflect.Struct:
		for i := range valuePtr.NumField() {
			field := elem.Field(i)
			fieldPointer := valuePtr.FieldByName(field.Name)
			fmt.Printf("Decode: %s: %s\n", field.Name, field.Type.String())

			newValue := reflect.New(fieldPointer.Type()).Interface()
			var err error
			switch fieldPointer.Kind() {
			case reflect.Float32, reflect.Float64, reflect.Int8, reflect.Uint8, reflect.Int16, reflect.Uint16, reflect.Int32, reflect.Uint32, reflect.Int64, reflect.Uint64:
				err = binary.Read(r, binary.BigEndian, newValue)
			case reflect.String:
				var size uint32
				if err = binary.Read(r, binary.BigEndian, &size); err == nil {
					buff := make([]byte, size)
					if _, err = r.Read(buff); err == nil {
						fieldPointer.SetString(string(buff))
						continue
					}
				}
			case reflect.Array: // [2]any
				if fieldPointer.Field(0).Type().String() == typeofByte.String() {
					if _, err = r.Read(newValue.([]byte)); err != nil {
						return err
					}
					fieldPointer.Set(reflect.Append(fieldPointer.Elem(), reflect.ValueOf(newValue)))
					continue
				}
				for i := range fieldPointer.Len() {
					newValue := reflect.New(fieldPointer.Field(i).Type()).Interface()
					if err = NewDecode(r, newValue); err != nil {
						return err
					}
					fieldPointer.Field(i).Set(reflect.ValueOf(newValue))
				}
				continue
			case reflect.Slice: // []any
				var size uint32
				if err = binary.Read(r, binary.BigEndian, &size); err != nil {
					return err
				} else if fieldPointer.Type().String() == "[]uint8" || fieldPointer.Type().String() == "*[]uint8" {
					buff := make([]byte, size)
					if _, err = r.Read(buff); err != nil {
						return err
					}
					fieldPointer.Set(reflect.ValueOf(buff))
					continue
				}
			default:
				switch fieldPointer.Type().String() {
				case "time.Time":
					var timestap int64
					if err = binary.Read(r, binary.BigEndian, &timestap); err == nil {
						fieldPointer.Set(reflect.ValueOf(time.UnixMicro(timestap)))
						continue
					}
				default:
					if strings.HasPrefix(fieldPointer.Type().String(), "*") {
						var ok uint8
						if err = binary.Read(r, binary.BigEndian, &ok); err == nil {
							if ok == 0 {
								continue
							}
						}
					}
					if err == nil {
						err = NewDecode(r, newValue)
					}
				}
			}

			if err != nil {
				return err
			}
			fieldPointer.Set(reflect.ValueOf(newValue).Elem())
		}
	}

	return nil
}

func Unmashall(b []byte, target any) error {
	return NewDecode(bytes.NewReader(b), target)
}

func Marshall(target any) ([]byte, error) {
	buff := new(bytes.Buffer)
	err := NewEncode(buff, target)
	return buff.Bytes(), err
}
