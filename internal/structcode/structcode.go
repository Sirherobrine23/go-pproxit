/*
Este bloco de codigo server apenas para facilitar minha vida no mundo do go
Definindo a estrutura de seguinte maneira, os dados estão fortemente ligado as structs go então qualquer merda aqui pode ferra com qualquer versão anterior, então isso não será recomendado para algumas coisa
Os dados serão escritos em BigEndian, então tenha cuidado com os dados inseridos casos sejam inportantes
Os pointers serão verificados se são nil's para idicar para a sereliazação e desereliazação

*any               -> int8(0|1) + data...
[]any              -> int64 (Size) + data...
[]bytes	Or String  -> Int64 (Size) + []bytes
map[any]any        -> int64 (Size) + (key + data)...
int64, uint64      -> int64
int32, uint32      -> int32
int16, uint16      -> int16
int8, uint8        -> int8
bool					     -> int8(0|1)
*/
package structcode

import (
	"encoding"
	"errors"
	"fmt"
	"io"
	"reflect"
)

const selectorTagName = "ser"

var (
	typeofError = reflect.TypeFor[error]()
	typeofBytes = reflect.TypeFor[[]byte]()

	typeofTextUnmarshal = reflect.TypeFor[encoding.TextUnmarshaler]()
	typeofBinUnmarshal  = reflect.TypeFor[encoding.BinaryUnmarshaler]()
	typeofTextMarshal   = reflect.TypeFor[encoding.TextMarshaler]()
	typeofBinMarshal    = reflect.TypeFor[encoding.BinaryMarshaler]()
)

func NewEncode(w io.Writer, target any) (err error) {
	if target == nil {
		return nil
	}
	defer func() {
		if ierr := recover(); ierr != nil {
			switch v := ierr.(type) {
			case error:
				err = v
			case string:
				err = errors.New(v)
			}
		}
	}()
	reflectValue := reflect.ValueOf(target)
	if reflectValue.Type().Kind() == reflect.Pointer {
		reflectValue = reflectValue.Elem()
	}
	return encodeRecursive(w, reflectValue)
}

func NewDecode(r io.Reader, target any) (err error) {
	defer func() {
		if ierr := recover(); ierr != nil {
			switch v := ierr.(type) {
			case error:
				err = v
			case string:
				err = errors.New(v)
			}
		}
	}()
	if target == nil {
		return fmt.Errorf("set target, not nil")
	} else if reflect.TypeOf(target).Kind() != reflect.Pointer {
		return fmt.Errorf("set pointer to struct")
	}
	return decodeRecursive(r, reflect.ValueOf(target).Elem())
}
