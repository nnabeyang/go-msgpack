package gomsgpack

import (
	"encoding/hex"
	"fmt"
	"math"
	"reflect"
	"testing"
	"time"
)

type SS string

func (s *SS) Type() int8 {
	return 3
}

func (s *SS) MarshalMsgPack() ([]byte, error) {
	return []byte(*s), nil
}
func (s *SS) UnmarshalMsgPack(data []byte) error {
	*s = SS(data)
	return nil
}

type Pair struct {
	X uint8
	Y uint8
}

type WithTag struct {
	A uint8  `msgpack:"a"`
	B string `msgpack:"b"`
}

type T = struct {
	X int `msgpack:"-"`
	Y int
}

type All struct {
	Bool        bool
	Int         int
	Int8        int8
	Int16       int16
	Int32       int32
	Int64       int64
	Uint        uint
	Uint8       uint8
	Uint16      uint16
	Uint32      uint32
	Uint64      uint64
	Uintptr     uintptr
	Float32     float32
	Float64     float64
	Foo         string `msgpack:"bar"`
	Foo2        string `msgpack:"bar2,dummyopt"`
	String      string
	Map         map[string]Small
	MapP        map[string]*Small
	EmptyMap    map[string]Small
	Slice       []Small
	SliceP      []*Small
	EmptySlice  []Small
	NilSlice    []Small
	StringSlice []string
	ByteSlice   []byte
	Small       Small
	Small2      Small `msgpack:"small,array"`
	PSmall      *Small
	Interface   any
	Time32      *Timestamp
	Time64      *Timestamp
	Time96      *Timestamp
	unexported  int
}

type Small struct {
	Tag string
}

var allValue = All{
	Bool:    true,
	Int:     int(-0x4b6b34abcc),
	Int8:    int8(-0x35),
	Int16:   int16(-0x4b6b),
	Int32:   int32(-0x4b6b34),
	Int64:   int64(-0x4b6b34abcc),
	Uint:    uint(0x4b6b34abcc),
	Uint8:   uint8(0x8),
	Uint16:  uint16(0x4b6b),
	Uint32:  uint32(0x004b6b34),
	Uint64:  uint64(0x4b6b34abcc),
	Uintptr: uintptr(0x4b6b34abcc),
	Float32: 14.1,
	Float64: float64(math.MaxFloat32) * 2,
	Foo:     "foo",
	Foo2:    "foo2",
	String:  "16",
	Map: map[string]Small{
		"17": {Tag: "tag17"},
		"18": {Tag: "tag18"},
	},
	MapP: map[string]*Small{
		"19": {Tag: "tag19"},
		"20": nil,
	},
	EmptyMap:    map[string]Small{},
	NilSlice:    nil,
	Slice:       []Small{{Tag: "tag20"}, {Tag: "tag21"}},
	SliceP:      []*Small{{Tag: "tag22"}, nil, {Tag: "tag23"}},
	EmptySlice:  []Small{},
	StringSlice: []string{"str24", "str25", "str26"},
	ByteSlice:   []byte{27, 28, 29},
	Small:       Small{Tag: "tag30"},
	Small2:      Small{Tag: "tag30"},
	PSmall:      &Small{Tag: "tag31"},
	Interface:   5.2,
	Time32:      NewTimestamp(time.Date(2022, 6, 22, 8, 56, 32, 0, time.UTC)),
	Time64:      NewTimestamp(time.Date(2022, 6, 22, 8, 56, 32, 999999999, time.UTC)),
	Time96:      NewTimestamp(time.Date(292277026596, 12, 4, 15, 30, 7, 999999999, time.UTC)),
}

type PAll struct {
	PBool      *bool
	PInt       *int
	PInt8      *int8
	PInt16     *int16
	PInt32     *int32
	PInt64     *int64
	PUint      *uint
	PUint8     *uint8
	PUint16    *uint16
	PUint32    *uint32
	PUint64    *uint64
	PUintptr   *uintptr
	PFloat32   *float32
	PFloat64   *float64
	PString    *string
	PMap       *map[string]Small
	PMapP      *map[string]*Small
	PSlice     *[]Small
	PSliceP    *[]*Small
	PPSmall    **Small
	PInterface *any
	unexported int
}

var pallValue = PAll{
	PBool:      &allValue.Bool,
	PInt:       &allValue.Int,
	PInt8:      &allValue.Int8,
	PInt16:     &allValue.Int16,
	PInt32:     &allValue.Int32,
	PInt64:     &allValue.Int64,
	PUint:      &allValue.Uint,
	PUint8:     &allValue.Uint8,
	PUint16:    &allValue.Uint16,
	PUint32:    &allValue.Uint32,
	PUint64:    &allValue.Uint64,
	PUintptr:   &allValue.Uintptr,
	PFloat32:   &allValue.Float32,
	PFloat64:   &allValue.Float64,
	PString:    &allValue.String,
	PMap:       &allValue.Map,
	PMapP:      &allValue.MapP,
	PSlice:     &allValue.Slice,
	PSliceP:    &allValue.SliceP,
	PPSmall:    &allValue.PSmall,
	PInterface: &allValue.Interface,
}

func equalError(a, b error) bool {
	if a == nil {
		return b == nil
	}
	if b == nil {
		return a == nil
	}
	return a.Error() == b.Error()
}

type unmarshalTest struct {
	in  string
	ptr any
	out any
	err error
}

func TestMarshal(t *testing.T) {
	data, err := Marshal(allValue, false)
	if err != nil {
		t.Fatalf("Marshal allValue: %v", err)
	}
	var v All
	if err := Unmarshal(data, &v); err != nil {
		t.Errorf(err.Error())
	}
	if !reflect.DeepEqual(v, allValue) {
		t.Error("Unmarshal allValue")
	}

	data, err = Marshal(pallValue, false)
	if err != nil {
		t.Fatalf("Marshal pallValue: %v", err)
	}

	var pv PAll
	if err := Unmarshal(data, &pv); err != nil {
		t.Fatalf("Marshal pallValue: %v", err)
	}
	if !reflect.DeepEqual(pv, pallValue) {
		t.Error("Unmarshal pallValue")
	}
}

var unmarshalTests = []unmarshalTest{
	// basic types
	{in: "01", ptr: new(uint8), out: uint8(0x01)},
	{in: "59", ptr: new(uint16), out: uint16(0x59)},
	{in: "7f", ptr: new(uint32), out: uint32(0x7f)},
	{in: "7f", ptr: new(uint64), out: uint64(0x7f)},
	{in: "7f", ptr: new(uint), out: uint(0x7f)},
	{in: "7f", ptr: new(uintptr), out: uintptr(0x7f)},
	{in: "01", ptr: new(int8), out: int8(0x01)},
	{in: "59", ptr: new(int16), out: int16(0x59)},
	{in: "7f", ptr: new(int32), out: int32(0x7f)},
	{in: "7f", ptr: new(int64), out: int64(0x7f)},
	{in: "7f", ptr: new(int), out: int(0x7f)},
	{in: "82a15812a15934", ptr: new(Pair), out: Pair{X: uint8(0x12), Y: uint8(0x34)}},
	{in: "82a15812a15934", ptr: new(map[string]uint8), out: map[string]uint8{"X": uint8(0x12), "Y": uint8(0x34)}},
	{in: "82a161ccafa162a378797a", ptr: new(WithTag), out: WithTag{A: 0xaf, B: "xyz"}},
	{in: "921234", ptr: new(Pair), out: Pair{X: uint8(0x12), Y: uint8(0x34)}},
	{in: "93a3616263a378797aa3646464", ptr: new([3]string), out: [3]string{"abc", "xyz", "ddd"}},
	{in: "93a3616263a378797aa3646464", ptr: new([]string), out: []string{"abc", "xyz", "ddd"}},
	{in: "90", ptr: new([]string), out: []string{}},
	{in: "c0", ptr: new(*int), out: (*int)(nil)},
	{in: "c0", ptr: new(*SS), out: (*SS)(nil)},
	{in: "c2", ptr: new(bool), out: false},
	{in: "c3", ptr: new(bool), out: true},
	{in: "c403123456", ptr: new([]byte), out: []byte{0x12, 0x34, 0x56}},
	{in: "c50003123456", ptr: new([]byte), out: []byte{0x12, 0x34, 0x56}},
	{in: "c600000003123456", ptr: new([]byte), out: []byte{0x12, 0x34, 0x56}},
	{in: "c7050348656c6c6f", ptr: new(SS), out: SS("Hello")},
	{in: "c70cff3b9ac9ff7fffffffffffffff", ptr: new(Timestamp), out: Timestamp(time.Date(292277026596, 12, 4, 15, 30, 7, 999999999, time.UTC))},
	{in: "c800050348656c6c6f", ptr: new(SS), out: SS("Hello")},
	{in: "c9000000050348656c6c6f", ptr: new(SS), out: SS("Hello")},
	{in: "ca4015c28f", ptr: new(float32), out: float32(2.34)},
	{in: "cb4002b851eb851eb8", ptr: new(float64), out: float64(2.34)},
	{in: "cc80", ptr: new(uint8), out: uint8(0x80)},
	{in: "cd4b6b", ptr: new(uint16), out: uint16(0x4b6b)},
	{in: "ce004b6b34", ptr: new(uint32), out: uint32(0x4b6b34)},
	{in: "cf0000004b6b34abcc", ptr: new(uint64), out: uint64(0x4b6b34abcc)},
	{in: "cf0000004b6b34abcc", ptr: new(uint), out: uint(0x4b6b34abcc)},
	{in: "cf0000004b6b34abcc", ptr: new(uintptr), out: uintptr(0x4b6b34abcc)},
	{in: "d081", ptr: new(int8), out: int8(-0x7f)},
	{in: "d1b495", ptr: new(int16), out: int16(-0x4b6b)},
	{in: "d2ffb494cc", ptr: new(int32), out: int32(-0x4b6b34)},
	{in: "d3ffffffb494cb5434", ptr: new(int64), out: int64(-0x4b6b34abcc)},
	{in: "d3ffffffb494cb5434", ptr: new(int), out: int(-0x4b6b34abcc)},
	{in: "d4013b", ptr: new(opacity), out: opacity(0x3b)},
	{in: "d5021234", ptr: new(position), out: position{X: 0x12, Y: 0x34}},
	{in: "d6ff62b2d940", ptr: new(Timestamp), out: Timestamp(time.Date(2022, 6, 22, 8, 56, 32, 0, time.UTC))},
	{in: "d7ffee6b27fc62b2d940", ptr: new(Timestamp), out: Timestamp(time.Date(2022, 6, 22, 8, 56, 32, 999999999, time.UTC))},
	{in: "d90548656c6c6f", ptr: new(string), out: "Hello"},
	{in: "da000548656c6c6f", ptr: new(string), out: "Hello"},
	{in: "db0000000548656c6c6f", ptr: new(string), out: "Hello"},
	{in: "a548656c6c6f", ptr: new(string), out: "Hello"},
	{in: "a0", ptr: new(string), out: ""},
	{in: "dc00021234", ptr: new(Pair), out: Pair{X: uint8(0x12), Y: uint8(0x34)}},
	{in: "dc0003a3616263a378797aa3646464", ptr: new([3]string), out: [3]string{"abc", "xyz", "ddd"}},
	{in: "dc0003a3616263a378797aa3646464", ptr: new([]string), out: []string{"abc", "xyz", "ddd"}},
	{in: "dc0000", ptr: new([]string), out: []string{}},
	{in: "dd000000021234", ptr: new(Pair), out: Pair{X: uint8(0x12), Y: uint8(0x34)}},
	{in: "dd00000003a3616263a378797aa3646464", ptr: new([3]string), out: [3]string{"abc", "xyz", "ddd"}},
	{in: "dd00000003a3616263a378797aa3646464", ptr: new([]string), out: []string{"abc", "xyz", "ddd"}},
	{in: "dd00000000", ptr: new([]string), out: []string{}},
	{in: "de0002a15812a15934", ptr: new(Pair), out: Pair{X: uint8(0x12), Y: uint8(0x34)}},
	{in: "de0002a15812a15934", ptr: new(map[string]uint8), out: map[string]uint8{"X": uint8(0x12), "Y": uint8(0x34)}},
	{in: "de0002a161ccafa162a378797a", ptr: new(WithTag), out: WithTag{A: 0xaf, B: "xyz"}},
	{in: "df00000002a15812a15934", ptr: new(Pair), out: Pair{X: uint8(0x12), Y: uint8(0x34)}},
	{in: "df00000002a15812a15934", ptr: new(map[string]uint8), out: map[string]uint8{"X": uint8(0x12), "Y": uint8(0x34)}},
	{in: "df00000002a161ccafa162a378797a", ptr: new(WithTag), out: WithTag{A: 0xaf, B: "xyz"}},
	{in: "e0", ptr: new(int8), out: int8(-0x20)},
	{in: "e0", ptr: new(int16), out: int16(-0x20)},
	{in: "e0", ptr: new(int32), out: int32(-0x20)},
	{in: "e0", ptr: new(int64), out: int64(-0x20)},
	{in: "e0", ptr: new(int), out: int(-0x20)},

	// Z has a "-" tag.
	{in: "81a15934", ptr: new(T), out: T{X: 0, Y: 0x34}},

	// UnmarshalTypeError
	{in: "c0", ptr: new(int), out: []byte{}, err: &UnmarshalTypeError{"null", reflect.TypeOf(1), 1, "", ""}},
	{in: "c2", ptr: new(int), out: []byte{}, err: &UnmarshalTypeError{"bool", reflect.TypeOf(1), 1, "", ""}},
	{in: "c403123456", ptr: new(int), out: []byte{}, err: &UnmarshalTypeError{"binary", reflect.TypeOf(1), 1, "", ""}},
	{in: "cb4002b851eb851eb8", ptr: new(int), out: float64(0), err: &UnmarshalTypeError{"float", reflect.TypeOf(1), 1, "", ""}},
	{in: "cc80", ptr: new(int), out: []byte{}, err: &UnmarshalTypeError{"uint", reflect.TypeOf(1), 1, "", ""}},
	{in: "d1b495", ptr: new(uint), out: uint(0), err: &UnmarshalTypeError{"int", reflect.TypeOf(uint(1)), 1, "", ""}},
	{in: "d90548656c6c6f", ptr: new(int), out: "", err: &UnmarshalTypeError{"string", reflect.TypeOf(1), 1, "", ""}},
	{in: "e0", ptr: new(string), out: int(0), err: &UnmarshalTypeError{"fixint", reflect.TypeOf(""), 1, "", ""}},
	{in: "6b", ptr: new(string), out: int(0), err: &UnmarshalTypeError{"fixint", reflect.TypeOf(""), 1, "", ""}},
	{in: "a548656c6c6f", ptr: new(int), out: "", err: &UnmarshalTypeError{"fixstr", reflect.TypeOf(1), 1, "", ""}},
	{in: "93a3616263a378797aa3646464", ptr: new(int), err: &UnmarshalTypeError{"array", reflect.TypeOf(1), 1, "", ""}},
	{in: "82a161ccafa162a378797a", ptr: new(int), err: &UnmarshalTypeError{"map", reflect.TypeOf(1), 1, "", ""}},
}

func TestUnmarshal(t *testing.T) {
	for i, tt := range unmarshalTests {
		in, _ := hex.DecodeString(tt.in)
		typ := reflect.TypeOf(tt.ptr)
		typ = typ.Elem()
		v := reflect.New(typ)
		if !reflect.DeepEqual(tt.ptr, v.Interface()) {
			t.Errorf("#%d: unmarshalTest.ptr %#v is not a pointer to a zero value", i, tt.ptr)
			continue
		}
		if err := Unmarshal(in, v.Interface()); !equalError(err, tt.err) {
			t.Errorf("#%d: %v, want %v", i, err, tt.err)
			continue
		} else if err != nil {
			continue
		}
		if !reflect.DeepEqual(v.Elem().Interface(), tt.out) {
			t.Errorf("#%d: mismatch\n have %#+v\nwant: %#+v", i, v.Elem().Interface(), tt.out)
			continue
		}
	}
}

var interfaceSetTests = []struct {
	pre  any
	src  string
	post any
}{
	{nil, "c0", nil},
	{new(bool), "c2", false},
	{new(int), "d012", int64(0x12)},
	{new(uint8), "12", int64(0x12)},
	{new(int8), "e0", int64(-0x20)},
	{new(uint), "cc12", uint64(0x12)},
	{new(float32), "cb4002b851eb851eb8", float64(2.34)},
	{new(string), "a548656c6c6f", "Hello"},
	{new(string), "d90548656c6c6f", "Hello"},
	{new([]byte), "c40548656c6c6f", []byte("Hello")},
	{
		new(map[string]any),
		"82a161ccafa162a378797a",
		map[any]any{
			"a": uint64(0xaf),
			"b": "xyz",
		},
	},
	{
		new([]string),
		"93a3616263a378797aa3646464",
		[]any{"abc", "xyz", "ddd"},
	},
}

func TestInterfaceSetTests(t *testing.T) {
	for _, tt := range interfaceSetTests {
		b := struct{ X any }{tt.pre}
		blob := "81a158" + tt.src
		data, _ := hex.DecodeString(blob)
		if err := Unmarshal(data, &b); err != nil {
			t.Errorf("Unmarshal %#q: %v", blob, err)
			continue
		}
		if !reflect.DeepEqual(b.X, tt.post) {
			t.Errorf("Unmarshal %#q into %#v: X=%#v %T, want %#v %T", blob, tt.pre, b.X, b.X, tt.post, tt.post)
		}
	}
}

var invalidUnmarshalTests = []struct {
	v    any
	want string
}{
	{nil, "msgpack: Unmarshal(nil)"},
	{struct{}{}, "msgpack: Unmarshal(non-pointer struct {})"},
	{(*int)(nil), "msgpack: Unmarshal(nil *int)"},
}

func TestInvalidUnmarshalError(t *testing.T) {
	data, _ := hex.DecodeString("c403123456")
	for _, tt := range invalidUnmarshalTests {
		err := Unmarshal(data, tt.v)
		if err == nil {
			t.Errorf("Unmarshal expecting error, got nil")
			continue
		}
		if got := err.Error(); got != tt.want {
			t.Errorf("Unmarshal = %q; want %q", got, tt.want)
		}
	}
}

func TestUnmarshalerError(t *testing.T) {
	var v marshalerErr
	data, _ := hex.DecodeString("d60362b2d940")
	err := Unmarshal(data, &v)
	expect := fmt.Errorf("unmarshaler error")
	if !equalError(err, expect) {
		t.Errorf("expect '%v' but, got '%v'", expect, err)
	}
}
