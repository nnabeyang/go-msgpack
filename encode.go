package gomsgpack

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"reflect"
	"sync"
)

const (
	maxUint4                  = 1<<4 - 1
	maxUint5                  = 1<<5 - 1
	fixIntMax                 = 0x7f
	fixIntMin                 = -0x20
	startDetectingCyclesAfter = 1000
)

// Marshaler is the interface implemented by types that
// can marshal themselves int valid MessagePack.
type Marshaler interface {
	MarshalMsgPack() ([]byte, error)
	Type() int8
}

var marshalerType = reflect.TypeOf((*Marshaler)(nil)).Elem()

// An UnsupportedTypeError is returned by Marshal when attempting
// to encode an unsupported value type.
type UnsupportedTypeError struct {
	Type reflect.Type
}

func (e *UnsupportedTypeError) Error() string {
	return "msgpack: unsupported type: " + e.Type.String()
}

// An UnsupportedValueError is returned by Marshal when attempting
// to encode an unsupported value type.
type UnsupportedValueError struct {
	Value reflect.Value
	Str   string
}

func (e *UnsupportedValueError) Error() string {
	return "msgpack: unsupported value: " + e.Str
}

// A MarshalerError represents an error from calling a MarshalMsgPack method.
type MarshalerError struct {
	Type reflect.Type
	Err  error
}

func (e *MarshalerError) Error() string {
	return "msgpack: error calling MarshalMsgPack" +
		" for type " + e.Type.String() +
		": " + e.Err.Error()
}

type encodeState struct {
	bytes.Buffer
	ptrLevel uint
	ptrSeen  map[any]struct{}
}

type msgPackError struct{ error }

func (e *encodeState) marshal(v any, asArray bool) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if me, ok := r.(msgPackError); ok {
				err = me.error
			} else {
				panic(r)
			}
		}
	}()
	e.reflectValue(reflect.ValueOf(v), asArray)
	return nil
}

func (e *encodeState) error(err error) {
	panic(msgPackError{err})
}

func (e *encodeState) reflectValue(v reflect.Value, asArray bool) {
	enc := typeEncoder(v.Type(), asArray)
	enc(e, v)
}

// Marshal returns the MessagePack encoding of v.
func Marshal(v any, asArray bool) ([]byte, error) {
	e := new(encodeState)
	e.ptrSeen = make(map[any]struct{})
	err := e.marshal(v, asArray)
	if err != nil {
		return nil, err
	}
	return e.Bytes(), nil
}

type encoderFunc func(e *encodeState, v reflect.Value)

type encoderFuncKey struct {
	Type    reflect.Type
	AsArray bool
}

var encoderCache sync.Map // map[reflect.Type]encoderFunc

func typeEncoder(t reflect.Type, asArray bool) encoderFunc {
	key := encoderFuncKey{
		Type:    t,
		AsArray: asArray,
	}
	if fi, ok := encoderCache.Load(key); ok {
		return fi.(encoderFunc)
	}
	var (
		wg sync.WaitGroup
		f  encoderFunc
	)
	wg.Add(1)
	fi, loaded := encoderCache.LoadOrStore(key, encoderFunc(func(e *encodeState, v reflect.Value) {
		wg.Wait()
		f(e, v)
	}))
	if loaded {
		return fi.(encoderFunc)
	}
	f = newTypeEncoder(t, asArray, true)
	wg.Done()
	encoderCache.Store(key, f)
	return f
}
func newTypeEncoder(t reflect.Type, asArray bool, allowAddr bool) encoderFunc {
	if t.Kind() != reflect.Pointer && allowAddr && reflect.PointerTo(t).Implements(marshalerType) {
		return newCondAddrEncoder(addrMarshalerEncoder, newTypeEncoder(t, asArray, false))
	}
	if t.Implements(marshalerType) {
		return marshalerEncoder
	}

	switch t.Kind() {
	case reflect.Bool:
		return boolEncoder
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return uintEncoder
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return intEncoder
	case reflect.Float32:
		return float32Encoder
	case reflect.Float64:
		return float64Encoder
	case reflect.String:
		return stringEncoder
	case reflect.Interface:
		return newInterfaceEncoder(asArray)
	case reflect.Slice:
		return newSliceEncoder(t)
	case reflect.Array:
		return newArrayEncoder(t)
	case reflect.Struct:
		return newStructEncoder(t, asArray)
	case reflect.Map:
		return newMapEncoder(t)
	case reflect.Pointer:
		return newPtrEncoder(t, asArray)
	default:
		return unsupportedTypeEncoder
	}
}

func boolEncoder(e *encodeState, v reflect.Value) {
	if v.Bool() {
		e.WriteByte(0xc3)
	} else {
		e.WriteByte(0xc2)
	}
}

func uintEncoder(e *encodeState, v reflect.Value) {
	value := v.Uint()
	if value <= fixIntMax {
		e.WriteByte(byte(value)) //positive fixint
		return
	}
	if value <= math.MaxUint8 {
		e.WriteByte(0xcc)
		e.WriteByte(byte(value))
		return
	}
	if value <= math.MaxUint16 {
		e.WriteByte(0xcd)
		bb := make([]byte, 2)
		binary.BigEndian.PutUint16(bb, uint16(value))
		e.Write(bb)
		return
	}
	if value <= math.MaxUint32 {
		e.WriteByte(0xce)
		bb := make([]byte, 4)
		binary.BigEndian.PutUint32(bb, uint32(value))
		e.Write(bb)
		return
	}
	e.WriteByte(0xcf)
	bb := make([]byte, 8)
	binary.BigEndian.PutUint64(bb, value)
	e.Write(bb)
}

func intEncoder(e *encodeState, v reflect.Value) {
	value := v.Int()
	if fixIntMin <= value && value <= fixIntMax { // fixint
		e.WriteByte(byte(value))
		return
	}
	if math.MinInt8 <= value && value <= math.MaxInt8 {
		e.WriteByte(0xd0)
		e.WriteByte(byte(value))
		return
	}
	if math.MinInt16 <= value && value <= math.MaxInt16 {
		bb := make([]byte, 2)
		binary.BigEndian.PutUint16(bb, uint16(value))
		e.WriteByte(0xd1)
		e.Write(bb)
		return
	}
	if math.MinInt32 <= value && value <= math.MaxInt32 {
		bb := make([]byte, 4)
		binary.BigEndian.PutUint32(bb, uint32(value))
		e.WriteByte(0xd2)
		e.Write(bb)
		return
	}
	bb := make([]byte, 8)
	binary.BigEndian.PutUint64(bb, uint64(value))
	e.WriteByte(0xd3)
	e.Write(bb)
}

func float32Encoder(e *encodeState, v reflect.Value) {
	value := v.Float()
	e.WriteByte(0xca)
	bb := make([]byte, 4)
	binary.BigEndian.PutUint32(bb, math.Float32bits(float32(value)))
	e.Write(bb)
}

func float64Encoder(e *encodeState, v reflect.Value) {
	value := v.Float()
	e.WriteByte(0xcb)
	bb := make([]byte, 8)
	binary.BigEndian.PutUint64(bb, math.Float64bits(value))
	e.Write(bb)
}

func stringEncoder(e *encodeState, v reflect.Value) {
	value := v.String()
	n := len(value)
	if n <= maxUint5 {
		e.WriteByte(byte(0xa0 + n))
		e.WriteString(value)
		return
	}
	if n <= math.MaxUint8 {
		e.WriteByte(0xd9)
		e.WriteByte(byte(n))
		e.WriteString(value)
		return
	}
	if n <= math.MaxUint16 {
		e.WriteByte(0xda)
		bb := make([]byte, 2)
		binary.BigEndian.PutUint16(bb, uint16(n))
		e.Write(bb)
		e.WriteString(value)
		return
	}
	if n <= math.MaxUint32 {
		e.WriteByte(0xdb)
		bb := make([]byte, 4)
		binary.BigEndian.PutUint32(bb, uint32(n))
		e.Write(bb)
		e.WriteString(value)
		return
	}
}

func newInterfaceEncoder(asArray bool) encoderFunc {
	return encoderFunc(func(e *encodeState, v reflect.Value) {
		if v.IsNil() {
			e.WriteByte(0xc0)
			return
		}
		e.reflectValue(v.Elem(), asArray)
	})
}

func encodeByteSlice(e *encodeState, v reflect.Value) {
	value := v.Bytes()
	n := len(value)
	if n <= math.MaxUint8 {
		e.WriteByte(0xc4)
		e.WriteByte(byte(n))
		e.Write(value)
		return
	}
	if n <= math.MaxUint16 {
		e.WriteByte(0xc5)
		bb := make([]byte, 2)
		binary.BigEndian.PutUint16(bb, uint16(n))
		e.Write(bb)
		e.Write(value)
		return
	}
	if n <= math.MaxUint32 {
		e.WriteByte(0xc6)
		bb := make([]byte, 4)
		binary.BigEndian.PutUint32(bb, uint32(n))
		e.Write(bb)
		e.Write(value)
		return
	}
}

type sliceEncoder struct {
	arrayEnc encoderFunc
}

func (se sliceEncoder) encode(e *encodeState, v reflect.Value) {
	if v.IsNil() {
		e.WriteByte(0xc0)
		return
	}
	if e.ptrLevel++; e.ptrLevel > startDetectingCyclesAfter {
		ptr := struct {
			ptr uintptr
			len int
		}{v.Pointer(), v.Len()}
		if _, ok := e.ptrSeen[ptr]; ok {
			e.error(&UnsupportedValueError{v, fmt.Sprintf("encounterred a cycle via %s", v.Type())})
		}
		e.ptrSeen[ptr] = struct{}{}
		defer delete(e.ptrSeen, ptr)
	}
	se.arrayEnc(e, v)
	e.ptrLevel--
}

func newSliceEncoder(t reflect.Type) encoderFunc {
	if t.Elem().Kind() == reflect.Uint8 {
		return encodeByteSlice
	}
	enc := sliceEncoder{newArrayEncoder(t)}
	return enc.encode
}

func newArrayEncoder(t reflect.Type) encoderFunc {
	enc := arrayEncoder{typeEncoder(t.Elem(), false)}
	return enc.encode
}

type arrayEncoder struct {
	elemEnc encoderFunc
}

func (ae arrayEncoder) encode(e *encodeState, v reflect.Value) {
	n := v.Len()
	if n <= maxUint4 {
		e.WriteByte(byte(0x90 + n))
	} else if n <= math.MaxUint16 {
		e.WriteByte(0xdc)
		bb := make([]byte, 2)
		binary.BigEndian.PutUint16(bb, uint16(n))
		e.Write(bb)
	} else if n <= math.MaxUint32 {
		e.WriteByte(0xdd)
		bb := make([]byte, 4)
		binary.BigEndian.PutUint32(bb, uint32(n))
		e.Write(bb)
	}
	for i := 0; i < n; i++ {
		ae.elemEnc(e, v.Index(i))
	}
}

type ptrEncoder struct {
	elemEnc encoderFunc
}

func (pe ptrEncoder) encode(e *encodeState, v reflect.Value) {
	if v.IsNil() {
		e.WriteByte(0xc0)
		return
	}

	if e.ptrLevel++; e.ptrLevel > startDetectingCyclesAfter {
		ptr := v.Interface()
		if _, ok := e.ptrSeen[ptr]; ok {
			e.error(&UnsupportedValueError{v, fmt.Sprintf("encounterred a cycle via %s", v.Type())})
		}
		e.ptrSeen[ptr] = struct{}{}
		defer delete(e.ptrSeen, ptr)
	}
	pe.elemEnc(e, v.Elem())
	e.ptrLevel--
}

func newPtrEncoder(t reflect.Type, asArray bool) encoderFunc {
	enc := ptrEncoder{typeEncoder(t.Elem(), asArray)}
	return enc.encode
}

func marshalerEncoder(e *encodeState, v reflect.Value) {
	m, ok := v.Interface().(Marshaler)
	if !ok {
		return
	}
	b, err := m.MarshalMsgPack()
	if err != nil {
		e.error(&MarshalerError{v.Type(), err})
	}
	switch n := len(b); n {
	case 1: // fixext 1
		e.WriteByte(0xd4)
		e.WriteByte(byte(m.Type()))
		e.Write(b)
	case 2: // fixext 2
		e.WriteByte(0xd5)
		e.WriteByte(byte(m.Type()))
		e.Write(b)
	case 4: // fixext 4
		e.WriteByte(0xd6)
		e.WriteByte(byte(m.Type()))
		e.Write(b)
	case 8: // fixext 8
		e.WriteByte(0xd7)
		e.WriteByte(byte(m.Type()))
		e.Write(b)
	case 16: // fixext 16
		e.WriteByte(0xd8)
		e.WriteByte(byte(m.Type()))
		e.Write(b)
	default:
		if n <= math.MaxUint8 { // ext 8
			e.WriteByte(0xc7)
			e.WriteByte(byte(n))
			e.WriteByte(byte(m.Type()))
			e.Write(b)
		} else if n <= math.MaxUint16 { // ext 16
			e.WriteByte(0xc8)
			bb := make([]byte, 2)
			binary.BigEndian.PutUint16(bb, uint16(n))
			e.Write(bb)
			e.WriteByte(byte(m.Type()))
			e.Write(b)
		} else if n <= math.MaxUint32 { // ext 32
			e.WriteByte(0xc9)
			bb := make([]byte, 4)
			binary.BigEndian.PutUint32(bb, uint32(n))
			e.Write(bb)
			e.WriteByte(byte(m.Type()))
			e.Write(b)
		}
	}
}

func addrMarshalerEncoder(e *encodeState, v reflect.Value) {
	va := v.Addr()
	if va.IsNil() {
		e.WriteByte(0xc0)
		return
	}
	marshalerEncoder(e, va)
}

type condAddrEncoder struct {
	canAddrEnc, elseEnc encoderFunc
}

func (ce condAddrEncoder) encode(e *encodeState, v reflect.Value) {
	if v.CanAddr() {
		ce.canAddrEnc(e, v)
	} else {
		ce.elseEnc(e, v)
	}
}

func newCondAddrEncoder(canAddrEnc, elseEnc encoderFunc) encoderFunc {
	enc := condAddrEncoder{canAddrEnc, elseEnc}
	return enc.encode
}

func newStructEncoder(t reflect.Type, asArray bool) encoderFunc {
	se := structEncoder{fields: cachedTypeFields(t), asArray: asArray}
	return se.encode
}

type structEncoder struct {
	fields  structFields
	asArray bool
}

func (se structEncoder) encode(e *encodeState, v reflect.Value) {
	n := len(se.fields.list)
	if se.asArray {
		if n <= maxUint4 {
			e.WriteByte(byte(0x90 + n))
		} else if n <= math.MaxUint16 {
			e.WriteByte(0xdc)
			bb := make([]byte, 2)
			binary.BigEndian.PutUint16(bb, uint16(n))
			e.Write(bb)
		} else if n <= math.MaxUint32 {
			e.WriteByte(0xdd)
			bb := make([]byte, 4)
			binary.BigEndian.PutUint32(bb, uint32(n))
			e.Write(bb)
		}
	} else {
		if n <= maxUint4 {
			e.WriteByte(byte(0x80 + n))
		} else if n <= math.MaxUint16 {
			e.WriteByte(0xde)
			bb := make([]byte, 2)
			binary.BigEndian.PutUint16(bb, uint16(n))
			e.Write(bb)
		} else if n <= math.MaxUint32 {
			e.WriteByte(0xdf)
			bb := make([]byte, 4)
			binary.BigEndian.PutUint32(bb, uint32(n))
			e.Write(bb)
		}
	}
	for i := range se.fields.list {
		f := &se.fields.list[i]
		fv := v
		for _, j := range f.index {
			if fv.Kind() == reflect.Pointer {
				fv = fv.Elem()
			}
			fv = fv.Field(j)
		}
		if !se.asArray {
			stringEncoder(e, reflect.ValueOf(f.name))
		}
		f.encoder(e, fv)
	}
}

type field struct {
	name    string
	typ     reflect.Type
	encoder encoderFunc
	index   []int
}

type structFields struct {
	list      []field
	nameIndex map[string]int
}

func typeFields(t reflect.Type) structFields {
	var fields []field
	current, next := []field{}, []field{{typ: t}}
	for len(next) > 0 {
		current, next = next, current[:0]
		for _, f := range current {
			for i := 0; i < f.typ.NumField(); i++ {
				sf := f.typ.Field(i)
				if sf.Anonymous {
					t := sf.Type
					if t.Kind() == reflect.Pointer {
						t = t.Elem()
					}
					if !sf.IsExported() && t.Kind() != reflect.Struct {
						continue
					}
				} else if !sf.IsExported() {
					continue
				}
				tag := sf.Tag.Get("msgpack")

				name, opts := parseTag(tag)
				if tag == "-" {
					continue
				}
				index := make([]int, len(f.index)+1)
				copy(index, f.index)
				index[len(f.index)] = i
				ft := sf.Type
				if ft.Name() == "" && ft.Kind() == reflect.Pointer {
					ft = ft.Elem()
				}

				if name != "" || !sf.Anonymous || sf.Type.Kind() != reflect.Struct {
					if !isValidTag(name) || name == "" {
						name = sf.Name
					}
					field := field{
						name:    name,
						typ:     ft,
						encoder: typeEncoder(typeByIndex(t, index), opts.Contains("array")),
						index:   index,
					}
					fields = append(fields, field)
					continue
				}

				next = append(next, field{name: sf.Type.Name(), index: index, typ: sf.Type})
			}
		}
	}
	nameIndex := make(map[string]int, len(fields))
	for i, field := range fields {
		nameIndex[field.name] = i
	}

	return structFields{list: fields, nameIndex: nameIndex}
}

func typeByIndex(t reflect.Type, index []int) reflect.Type {
	for _, i := range index {
		if t.Kind() == reflect.Pointer {
			t = t.Elem()
		}
		t = t.Field(i).Type
	}
	return t
}

func isValidTag(s string) bool {
	if s == "" {
		return false
	}
	return true
}

func newMapEncoder(t reflect.Type) encoderFunc {
	me := mapEncoder{
		keyEnc:   typeEncoder(t.Key(), false),
		valueEnc: typeEncoder(t.Elem(), false),
	}
	return me.encode
}

type mapEncoder struct {
	keyEnc   encoderFunc
	valueEnc encoderFunc
}

func (me mapEncoder) encode(e *encodeState, v reflect.Value) {
	if v.IsNil() {
		e.WriteByte(0xc0)
		return
	}
	if e.ptrLevel++; e.ptrLevel > startDetectingCyclesAfter {
		ptr := v.Pointer()
		if _, ok := e.ptrSeen[ptr]; ok {
			e.error(&UnsupportedValueError{v, fmt.Sprintf("encounterred a cycle via %s", v.Type())})
		}
		e.ptrSeen[ptr] = struct{}{}
		defer delete(e.ptrSeen, ptr)
	}
	n := v.Len()
	if n <= maxUint4 {
		e.WriteByte(byte(0x80 + n))
	} else if n <= math.MaxUint16 {
		e.WriteByte(0xde)
		bb := make([]byte, 2)
		binary.BigEndian.PutUint16(bb, uint16(n))
		e.Write(bb)
	} else if n <= math.MaxUint32 {
		e.WriteByte(0xdf)
		bb := make([]byte, 4)
		binary.BigEndian.PutUint32(bb, uint32(n))
		e.Write(bb)
	}
	mi := v.MapRange()
	for i := 0; mi.Next(); i++ {
		me.keyEnc(e, mi.Key())
		me.valueEnc(e, mi.Value())
	}
	e.ptrLevel--
}

func unsupportedTypeEncoder(e *encodeState, v reflect.Value) {
	e.error(&UnsupportedTypeError{v.Type()})
}

var fieldCache sync.Map // map[reflect.Type]structFields

func cachedTypeFields(t reflect.Type) structFields {
	if f, ok := fieldCache.Load(t); ok {
		return f.(structFields)
	}
	f, _ := fieldCache.LoadOrStore(t, typeFields(t))
	return f.(structFields)
}
