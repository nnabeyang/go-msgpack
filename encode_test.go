package gomsgpack

import (
	"encoding/hex"
	"fmt"
	"reflect"
	"testing"
)

type opacity uint8

func (o *opacity) Type() int8 {
	return 1
}

func (*opacity) MarshalMsgPack() ([]byte, error) {
	return []byte{byte(0x3d)}, nil
}
func (o *opacity) UnmarshalMsgPack(data []byte) error {
	*o = opacity(data[0])
	return nil
}

type position struct {
	X int8
	Y int8
}

func (o *position) Type() int8 {
	return 2
}

func (p *position) MarshalMsgPack() ([]byte, error) {
	return []byte{byte(0x12), byte(0x34)}, nil
}
func (o *position) UnmarshalMsgPack(data []byte) error {
	*o = position{
		X: int8(data[0]),
		Y: int8(data[1]),
	}
	return nil
}

type marshalerErr struct {
	X int8
	Y int8
}

func (o *marshalerErr) Type() int8 {
	return 3
}

func (p *marshalerErr) MarshalMsgPack() ([]byte, error) {
	return nil, fmt.Errorf("marshaler error")
}
func (p *marshalerErr) UnmarshalMsgPack(data []byte) error {
	return fmt.Errorf("unmarshaler error")
}

type xy struct {
	X int8
	Y int8
}

func (o xy) Type() int8 {
	return 4
}

func (p xy) MarshalMsgPack() ([]byte, error) {
	return []byte{0xff}, nil
}

type Optionals struct {
	Str Pair `msgpack:"pair"`
	Sta Pair `msgpack:",array"`
	Stw Pair `msgpack:"-"`
}

func TestEncodeOptionals(t *testing.T) {
	o := Optionals{
		Str: Pair{X: 0x12, Y: 0x34},
		Sta: Pair{X: 0x56, Y: 0x78},
		Stw: Pair{X: 0x9a, Y: 0xbc},
	}
	got, err := Marshal(o, false)
	if err != nil {
		t.Fatal(err)
	}
	actual := hex.EncodeToString(got)
	expect := "82a47061697282a15812a15934a3537461925678"
	if actual != expect {
		t.Errorf("got : %s\nwant: %s\n", actual, expect)
	}
	var p, expect2 Optionals
	err = Unmarshal(got, &p)
	expect2.Str = o.Str
	expect2.Sta = o.Sta
	if reflect.DeepEqual(o, expect2) {
		t.Errorf("got : %v\nwant: %v %T\n", p, expect2, p.Stw.X)
	}
}

type SamePointerNoCycle struct {
	Ptr1, Ptr2 *SamePointerNoCycle
}

type PointerCycle struct {
	Ptr *PointerCycle
}

var pointerCycle = &PointerCycle{}

type PointerCycleIndirect struct {
	Ptrs []any
}

type RecursiveSlice []RecursiveSlice

func TestSamePointerNoCycle(t *testing.T) {
	var samePointerNoCycle = &SamePointerNoCycle{}
	ptr := &SamePointerNoCycle{}
	samePointerNoCycle.Ptr1 = ptr
	samePointerNoCycle.Ptr2 = ptr
	if _, err := Marshal(samePointerNoCycle, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSliceNoCycle(t *testing.T) {
	var sliceNoCycle = []any{nil, nil}
	sliceNoCycle[1] = sliceNoCycle[:1]
	for i := startDetectingCyclesAfter; i > 0; i-- {
		sliceNoCycle = []any{sliceNoCycle}
	}
	if _, err := Marshal(sliceNoCycle, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUnsupportedValues(t *testing.T) {
	pointerCycleIndirect := &PointerCycleIndirect{}
	mapCycle := make(map[string]any)
	sliceCycle := []any{nil}
	recursiveSliceCycle := []RecursiveSlice{nil}
	pointerCycle.Ptr = pointerCycle
	pointerCycleIndirect.Ptrs = []any{pointerCycleIndirect}
	sliceCycle[0] = sliceCycle
	recursiveSliceCycle[0] = recursiveSliceCycle
	mapCycle["x"] = mapCycle
	unsupportedValues := []any{
		pointerCycle,
		pointerCycleIndirect,
		mapCycle,
		sliceCycle,
		recursiveSliceCycle,
	}
	for i, v := range unsupportedValues {
		if _, err := Marshal(v, false); err != nil {
			if _, ok := err.(*UnsupportedValueError); !ok {
				t.Errorf("for %v, got %T want UnsupportedValueError", v, err)
			}
		} else {
			t.Errorf("#%d:for %v, expected error", i, v)
		}
	}
}

type marshalPanic struct{}

func (marshalPanic) MarshalMsgPack() ([]byte, error) { panic(0xdead) }
func (marshalPanic) Type() int8                      { return 0x44 }
func TestMarshalPanic(t *testing.T) {
	defer func() {
		if got := recover(); !reflect.DeepEqual(got, 0xdead) {
			t.Errorf("panic() = (%T)(%v), want 0xdead", got, got)
		}
	}()
	Marshal(&marshalPanic{}, false)
	t.Error("Marshal should have panicked")
}

func TestEncodeMarshalerError(t *testing.T) {
	v := new(marshalerErr)
	_, err := Marshal(v, false)
	rv := reflect.ValueOf(v)
	expect := &MarshalerError{rv.Type(), fmt.Errorf("marshaler error")}
	if !equalError(err, expect) {
		t.Errorf("expect '%v' but, got '%v'", expect, err)
	}
}

func TestEncodeFixExt(t *testing.T) {
	{ // fixext 1
		v := new(opacity)
		data, _ := Marshal(v, false)
		expect := "d4013d"
		actual := hex.EncodeToString(data)
		if actual != expect {
			t.Errorf("expect '%s' but, got '%s'", expect, actual)
		}
	}
	{ // fixext 2
		v := new(position)
		data, _ := Marshal(v, false)
		expect := "d5021234"
		actual := hex.EncodeToString(data)
		if actual != expect {
			t.Errorf("expect '%s' but, got '%s'", expect, actual)
		}
	}
}
