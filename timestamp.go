package gomsgpack

import (
	"encoding/binary"
	"time"
)

// Timestamp represents an instantaneous point on the time-line in the world that
// is independent from time zones or calendars. Maximum precision is nanoseconds.
// Timestamp extension type is assigned to extension type -1.
// It defines 3 formats: 32-bit format, 64-bit format, and 96-bit format.
type Timestamp time.Time

func (t *Timestamp) MarshalMsgPack() ([]byte, error) {
	tt := time.Time(*t)
	var bb []byte
	sec := uint64(tt.Unix())
	if sec>>34 == 0 {
		nsec := uint64(tt.Nanosecond())
		if nsec == 0 { // timestamp 32 bit
			bb = make([]byte, 4)
			binary.BigEndian.PutUint32(bb, uint32(sec))
			return bb, nil
		}
		// timestamp 64 bit
		bb = make([]byte, 8)
		data := nsec<<34 | sec
		binary.BigEndian.PutUint64(bb, data)
		return bb, nil
	}
	// timestamp 96 bit
	bb = make([]byte, 12)
	binary.BigEndian.PutUint32(bb, uint32(tt.Nanosecond()))
	binary.BigEndian.PutUint64(bb[4:], sec)
	return bb, nil
}

func (t *Timestamp) UnmarshalMsgPack(data []byte) error {
	n := len(data)
	switch n {
	case 4: // timestamp 32 bit
		sec := binary.BigEndian.Uint32(data)
		tt := time.Unix(int64(sec), 0).UTC()
		*t = Timestamp(tt)
	case 8: // timestamp 64 bit
		data := binary.BigEndian.Uint64(data)
		nsec := int64(data >> 34)
		sec := int64(data & (1<<34 - 1))
		tt := time.Unix(sec, nsec).UTC()
		*t = Timestamp(tt)
	case 12: // timestamp 96 bit
		nsec := int64(binary.BigEndian.Uint32(data[0:4]))
		sec := int64(binary.BigEndian.Uint64(data[4:]))
		tt := time.Unix(sec, nsec).UTC()
		*t = Timestamp(tt)

	}
	return nil
}

func (t *Timestamp) Type() int8 {
	return int8(-1)
}

func NewTimestamp(t time.Time) *Timestamp {
	v := new(Timestamp)
	*v = Timestamp(t)
	return v
}
