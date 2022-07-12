package gomsgpack

import (
	"encoding/binary"
	"fmt"
	"math"
	"reflect"
	"strings"
)

// Unmarshal decodes the MessagePack-encoded data and stores the result
// in the value pointed to by v.
func Unmarshal(data []byte, v any) error {
	d := &decodeState{}
	d.init(data)
	return d.unmarshal(v)
}

// Unmarshaler is the interface implemented by types
// that can unmarshal a MessagePack description of themselves.
type Unmarshaler interface {
	UnmarshalMsgPack([]byte) error
	Type() int8
}

// An UnmarshalTypeError describes a MessagePack value that was
// not appropriate for a value of a specific Go type.
type UnmarshalTypeError struct {
	Value  string
	Type   reflect.Type
	Offset int64
	Struct string
	Field  string
}

func (e *UnmarshalTypeError) Error() string {
	if e.Struct != "" || e.Field != "" {
		return "msgpack: cannot unmarshal " + e.Value + " into Go struct field " + e.Struct + "." + e.Field + " of type " + e.Type.String()
	}
	return "msgpack: cannnot unmarshal " + e.Value + " into Go value of type " + e.Type.String()
}

// An InvalidUnmarshalError describes an invalid argument passed to Unmarshal.
type InvalidUnmarshalError struct {
	Type reflect.Type
}

func (e *InvalidUnmarshalError) Error() string {
	if e.Type == nil {
		return "msgpack: Unmarshal(nil)"
	}

	if e.Type.Kind() != reflect.Pointer {
		return "msgpack: Unmarshal(non-pointer " + e.Type.String() + ")"
	}
	return "msgpack: Unmarshal(nil " + e.Type.String() + ")"
}

type InvalidDataError struct {
	msg    string
	Offset int64
}

func (e *InvalidDataError) Error() string { return e.msg }

type errorContext struct {
	Struct     reflect.Type
	FieldStack []string
}

type decodeState struct {
	data         []byte
	off          int
	opcode       int
	errorContext *errorContext
	savedError   error
}

func (d *decodeState) unmarshal(v any) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return &InvalidUnmarshalError{reflect.TypeOf(v)}
	}
	d.scanNext()
	err := d.value(rv)
	if err != nil {
		return d.addErrorContext(err)
	}
	return d.savedError
}

func (d *decodeState) readIndex() int {
	return d.off - 1
}

func (d *decodeState) init(data []byte) {
	d.data = data
	d.off = 0
}

func (d *decodeState) saveError(err error) {
	if d.savedError == nil {
		d.savedError = d.addErrorContext(err)
	}
}

func (d *decodeState) addErrorContext(err error) error {
	if d.errorContext != nil && (d.errorContext.Struct != nil || len(d.errorContext.FieldStack) > 0) {
		switch err := err.(type) {
		case *UnmarshalTypeError:
			err.Struct = d.errorContext.Struct.Name()
			err.Field = strings.Join(d.errorContext.FieldStack, ".")
		}
	}
	return err
}

func (d *decodeState) scanNext() {
	if d.off < len(d.data) {
		d.opcode = step(d.data[d.off])
		d.off++
	} else {
		d.opcode = scanEnd
		d.off = len(d.data) + 1
	}
}

func (d *decodeState) scanCurrent() {
	if d.off > 0 && d.off < len(d.data) {
		d.opcode = step(d.data[d.off-1])
	}
}

const (
	scanBeginLiteral = iota
	scanBeginMap
	scanBeginArray
	scanEnd
	scanNeverUsed
)

func step(c byte) int {
	if 0xbf >= c || c >= 0xe0 {
		if c&0xe0 == 0xe0 { // negative fixint
			return scanBeginLiteral
		}
		if c&0xa0 == 0xa0 { // fixstr
			return scanBeginLiteral
		}
		if c&0x90 == 0x90 { // fixarray
			return scanBeginArray
		}
		if c&0x80 == 0x80 { // fixmap
			return scanBeginMap
		}
		if c&0x80 == 0 { // positive fixint
			return scanBeginLiteral
		}
		return scanNeverUsed
	}
	switch c {
	case 0xc1: // never used
		return scanNeverUsed
	case 0xdc, 0xdd: // array 16, array 32
		return scanBeginArray
	case 0xde, 0xdf: // map 16, map 32
		return scanBeginMap
	default:
		return scanBeginLiteral
	}
}

func (d *decodeState) rescanLiteral() error {
	i := d.off
	if d.off > len(d.data) {
		return &InvalidDataError{"unexpected end of MessagePack input", int64(d.off)}
	}
	c := d.data[i-1]
	switch c {
	case 0xc0, 0xc2, 0xc3: // nil, false, true
	case 0xc4, 0xc5, 0xc6: // bin 8, bin 16, bin 32
		nn := 1 << (c - 0xc4)
		i += int(bigEndianUint(d.data[i:i+nn])) + nn
	case 0xc7, 0xc8, 0xc9: // ext 8, ext 16, ext 32
		nn := 1 << (c - 0xc7)
		i += int(bigEndianUint(d.data[i:i+nn])) + nn + 1
	case 0xca, 0xcb: // float32, float64
		i += 4 << (c - 0xca)
	case 0xcc, 0xcd, 0xce, 0xcf: // uint8, uint16, uint32, uint64
		i += 1 << (c - 0xcc)
	case 0xd0, 0xd1, 0xd2, 0xd3: // int8, int16, int32, int64
		i += 1 << (c - 0xd0)
	case 0xd4, 0xd5, 0xd6, 0xd7, 0xd8: // fixext 1, fixext 4, fixext 8, fixext 16
		i += 1 + (1 << (c - 0xd4))
	case 0xd9, 0xda, 0xdb: // str8, str16, str32
		nn := 1 << (c - 0xd9)
		n := bigEndianUint(d.data[i : i+nn])
		i += int(n) + nn
	default:
		if c&0xe0 == 0xe0 { // negative fixint
			break
		}
		if c&0xa0 == 0xa0 { // fixstr
			i += int(c - 0xa0)
		}
	}
	d.off = i + 1
	return nil
}

func (d *decodeState) value(v reflect.Value) error {
	if d.off > len(d.data) {
		return &InvalidDataError{"unexpected end of MessagePack input", int64(d.off)}
	}
	switch d.opcode {
	default:
		panic("MessagePack decoder out of sync")
	case scanBeginArray:
		if err := d.array(v); err != nil {
			return err
		}
	case scanBeginMap:
		if err := d.object(v); err != nil {
			return err
		}
	case scanBeginLiteral:
		start := d.readIndex()
		if err := d.rescanLiteral(); err != nil {
			return err
		}
		end := d.readIndex()
		if len(d.data) <= end-1 {
			return &InvalidDataError{"unexpected end of MessagePack input", int64(end - 1)}
		}
		if err := d.literalStore(d.data[start:end], v); err != nil {
			return err
		}
	case scanNeverUsed:
		return fmt.Errorf("msgpack: 0xc1 is never used code in MessagePack format.")
	}
	return nil
}

func (d *decodeState) array(v reflect.Value) error {
	var n int
	c := d.data[d.off-1]
	b := 0
	switch c {
	case 0xdc, 0xdd: // array 16, array 32
		nn := 1 << (c - 0xdb)
		n = int(bigEndianUint(d.data[d.off : d.off+nn]))
		b += 1 + nn
		d.off += nn
	default:
		if c&0x90 == 0x90 { // fixarray
			n = int(c - 0x90)
		}
	}
	for {
		if v.Kind() != reflect.Pointer {
			break
		}
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		v = v.Elem()
	}
	d.scanNext()
	t := v.Kind()
	switch v.Kind() {
	case reflect.Slice:
		if n == 0 {
			v.Set(reflect.MakeSlice(v.Type(), 0, 0))
			return nil
		}
		if n > v.Cap() {
			newv := reflect.MakeSlice(v.Type(), v.Len(), n)
			reflect.Copy(newv, v)
			v.Set(newv)
		}
		if n > v.Len() {
			v.SetLen(n)
		}
	case reflect.Interface:
		if v.NumMethod() == 0 {
			ai := make([]any, n, n)
			vv := reflect.ValueOf(ai)
			for i := 0; i < n; i++ {
				if err := d.value(vv.Index(i)); err != nil {
					return err
				}
				d.scanCurrent()
			}
			v.Set(reflect.ValueOf(ai))
			return nil
		}
		fallthrough
	default:
		d.saveError(&UnmarshalTypeError{Value: "array", Type: v.Type(), Offset: int64(d.readIndex())})
	case reflect.Array, reflect.Struct:
		break
	}
	switch t {
	case reflect.Array, reflect.Slice:
		for i := 0; i < n; i++ {
			if err := d.value(v.Index(i)); err != nil {
				return err
			}
			d.scanCurrent()
		}
	case reflect.Struct:
		for i := 0; i < n; i++ {
			if err := d.value(v.Field(i)); err != nil {
				return err
			}
			d.scanCurrent()
		}
	}
	return nil
}

func (d *decodeState) object(v reflect.Value) error {
	var n int
	c := d.data[d.off-1]
	switch c {
	case 0xde, 0xdf: // map 16, map 32
		nn := 1 << (c - 0xdd)
		n = int(bigEndianUint(d.data[d.off : d.off+nn]))
		d.off += nn
	default:
		if c&0x80 == 0x80 { // fixmap
			n = int(c - 0x80)
		}
	}
	for {
		if v.Kind() != reflect.Pointer {
			break
		}
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		v = v.Elem()
	}

	if v.Kind() == reflect.Interface && v.NumMethod() == 0 {
		mi := make(map[any]any)
		vv := reflect.ValueOf(mi)
		v.Set(vv)
		rt := vv.Type()
		keyType := rt.Key()
		valueType := rt.Elem()
		d.scanNext()
		for i := 0; i < n; i++ {
			key := reflect.New(keyType).Elem()
			d.scanCurrent()
			if err := d.value(key); err != nil {
				return err
			}
			elem := reflect.New(valueType).Elem()
			d.scanCurrent()
			if err := d.value(elem); err != nil {
				return err
			}
			vv.SetMapIndex(key, elem)
		}
		return nil
	}
	switch v.Kind() {
	case reflect.Struct:
		fields := cachedTypeFields(v.Type())
		d.scanNext()
		for i := 0; i < n; i++ {
			// Read key
			start := d.readIndex()
			if err := d.rescanLiteral(); err != nil {
				return err
			}
			item := d.data[start:d.readIndex()]
			key := string(item[1:])
			if i, ok := fields.nameIndex[key]; ok {
				f := fields.list[i]
				subv := v
				for _, i := range f.index {
					if subv.Kind() == reflect.Pointer {
						subv = subv.Elem()
					}
					subv = subv.Field(i)
				}
				d.scanCurrent()
				if err := d.value(subv); err != nil {
					return err
				}
			}
		}
	case reflect.Map:
		if v.IsNil() {
			v.Set(reflect.MakeMap(v.Type()))
		}
		rt := v.Type()
		keyType := rt.Key()
		valueType := rt.Elem()
		d.scanNext()
		for i := 0; i < n; i++ {
			key := reflect.New(keyType).Elem()
			d.scanCurrent()
			if err := d.value(key); err != nil {
				return err
			}
			elem := reflect.New(valueType).Elem()
			d.scanCurrent()
			if err := d.value(elem); err != nil {
				return err
			}
			v.SetMapIndex(key, elem)
		}
	default:
		d.saveError(&UnmarshalTypeError{Value: "map", Type: v.Type(), Offset: int64(d.readIndex())})
	}
	return nil
}

func (d *decodeState) literalStore(item []byte, v reflect.Value) error {
	if v.Type().NumMethod() > 0 && v.CanInterface() {
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		if u, ok := v.Interface().(Unmarshaler); ok {
			switch c := item[0]; c {
			case 0xd4, 0xd5, 0xd6, 0xd7, 0xd8: // fixext1, fixext 2, fixext 4, fixext 8
				if int8(item[1]) == u.Type() {
					return u.UnmarshalMsgPack(item[2:])
				}
				return nil
			case 0xc7, 0xc8, 0xc9: // ext 8, ext 16, ext 32
				nn := 1 + 1<<(c-0xc7)
				if int8(item[nn]) == u.Type() {
					return u.UnmarshalMsgPack(item[nn+1:])
				}
				return nil
			}
		}
	}
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		} else {
			v = v.Elem()
		}
	}
	switch c := item[0]; c {
	case 0xc0: // null
		switch v.Kind() {
		case reflect.Interface, reflect.Pointer, reflect.Map, reflect.Slice:
			v.Set(reflect.Zero(v.Type()))
		default:
			d.saveError(&UnmarshalTypeError{Value: "null", Type: v.Type(), Offset: int64(d.readIndex())})
		}
		return nil
	case 0xc2, 0xc3: // false, true
		switch v.Kind() {
		case reflect.Bool:
			v.SetBool(0xc3 == c)
		case reflect.Pointer:
			v = v.Elem()
			v.SetBool(0xc3 == c)
		case reflect.Interface:
			v.Set(reflect.ValueOf(0xc3 == c))
		default:
			d.saveError(&UnmarshalTypeError{Value: "bool", Type: v.Type(), Offset: int64(d.readIndex())})
		}
		return nil
	case 0xc4, 0xc5, 0xc6: // bin 8, bin 16, bin 32
		switch v.Kind() {
		case reflect.Slice:
			if v.Type().Elem().Kind() == reflect.Uint8 {
				v.SetBytes(item[1+(1<<(c-0xc4)):])
			} else {
				d.saveError(&UnmarshalTypeError{Value: "binary", Type: v.Type(), Offset: int64(d.readIndex())})
			}
		case reflect.Interface:
			v.Set(reflect.ValueOf(item[1+(1<<(c-0xc4)):]))
		default:
			d.saveError(&UnmarshalTypeError{Value: "binary", Type: v.Type(), Offset: int64(d.readIndex())})
		}
		return nil
	case 0xca, 0xcb: // float32, float64
		if v.Kind() == reflect.Pointer {
			v = v.Elem()
		}
		switch v.Kind() {
		case reflect.Float32, reflect.Float64:
			v.SetFloat(bigEndianFloat(item[1:]))
		case reflect.Interface:
			if v.NumMethod() == 0 {
				v.Set(reflect.ValueOf(bigEndianFloat(item[1:])))
			} else {
				d.saveError(&UnmarshalTypeError{Value: "float", Type: v.Type(), Offset: int64(d.readIndex())})
			}
		default:
			d.saveError(&UnmarshalTypeError{Value: "float", Type: v.Type(), Offset: int64(d.readIndex())})
		}
		return nil
	case 0xcc, 0xcd, 0xce, 0xcf: // uint8, uint16, uint32, uint64
		if v.Kind() == reflect.Pointer {
			v = v.Elem()
		}
		switch v.Kind() {
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			v.SetUint(bigEndianUint(item[1:]))
		case reflect.Interface:
			if v.NumMethod() == 0 {
				v.Set(reflect.ValueOf(bigEndianUint(item[1:])))
			} else {
				d.saveError(&UnmarshalTypeError{Value: "uint", Type: v.Type(), Offset: int64(d.readIndex())})
			}
		default:
			d.saveError(&UnmarshalTypeError{Value: "uint", Type: v.Type(), Offset: int64(d.readIndex())})
		}
		return nil
	case 0xd0, 0xd1, 0xd2, 0xd3: // int8, int16, int32, int64
		if v.Kind() == reflect.Pointer {
			v = v.Elem()
		}
		switch v.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			v.SetInt(int64(bigEndianUint(item[1:])))
		case reflect.Interface:
			if v.NumMethod() == 0 {
				v.Set(reflect.ValueOf(int64(bigEndianUint(item[1:]))))
			} else {
				d.saveError(&UnmarshalTypeError{Value: "int", Type: v.Type(), Offset: int64(d.readIndex())})
			}
		default:
			d.saveError(&UnmarshalTypeError{Value: "int", Type: v.Type(), Offset: int64(d.readIndex())})
		}
		return nil
	case 0xd9, 0xda, 0xdb: // str8, str16, str32
		if v.Kind() == reflect.Pointer {
			v = v.Elem()
		}
		switch v.Kind() {
		case reflect.String:
			v.SetString(string(item[1+(1<<(c-0xd9)):]))
		case reflect.Interface:
			v.Set(reflect.ValueOf(string(item[1+(1<<(c-0xd9)):])))
		default:
			d.saveError(&UnmarshalTypeError{Value: "string", Type: v.Type(), Offset: int64(d.readIndex())})
		}
		return nil
	}

	if len(item) == 1 && item[0]&0xe0 == 0xe0 { // negative fixint
		if v.Kind() == reflect.Pointer {
			v = v.Elem()
		}
		switch v.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			v.SetInt(int64(int8(item[0])))
		case reflect.Interface:
			v.Set(reflect.ValueOf(int64(int8(item[0]))))
		default:
			d.saveError(&UnmarshalTypeError{Value: "fixint", Type: v.Type(), Offset: int64(d.readIndex())})
		}
		return nil
	}
	if item[0]&0xa0 == 0xa0 { // fixstr
		if v.Kind() == reflect.Pointer {
			v = v.Elem()
		}
		switch v.Kind() {
		case reflect.String:
			v.SetString(string(item[1:]))
		case reflect.Interface:
			v.Set(reflect.ValueOf(string(item[1:])))
		default:
			d.saveError(&UnmarshalTypeError{Value: "fixstr", Type: v.Type(), Offset: int64(d.readIndex())})
		}
		return nil
	}

	if len(item) == 1 && item[0]&0x80 == 0 { // positive fixint
		if v.Kind() == reflect.Pointer {
			v = v.Elem()
		}
		switch v.Kind() {
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			v.SetUint(uint64(item[0]))
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			v.SetInt(int64(item[0]))
		case reflect.Interface:
			v.Set(reflect.ValueOf(int64(item[0])))
		default:
			d.saveError(&UnmarshalTypeError{Value: "fixint", Type: v.Type(), Offset: int64(d.readIndex())})
		}
	}
	return nil
}

func bigEndianUint(data []byte) uint64 {
	switch len(data) {
	case 1:
		return uint64(data[0])
	case 2:
		return uint64(binary.BigEndian.Uint16(data))
	case 4:
		return uint64(binary.BigEndian.Uint32(data))
	case 8:
		return binary.BigEndian.Uint64(data)
	}
	panic("bigEndianInt: invalid data")
}

func bigEndianFloat(data []byte) float64 {
	switch len(data) {
	case 4:
		return float64(math.Float32frombits(binary.BigEndian.Uint32(data)))
	case 8:
		return float64(math.Float64frombits(binary.BigEndian.Uint64(data)))
	}
	panic("bigEndianFloat: invalid data")
}
