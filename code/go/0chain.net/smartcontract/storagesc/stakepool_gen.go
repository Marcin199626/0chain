package storagesc

// Code generated by github.com/tinylib/msgp DO NOT EDIT.

import (
	"github.com/tinylib/msgp/msgp"
)

// MarshalMsg implements msgp.Marshaler
func (z *stakePool) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 3
	// string "StakePool"
	o = append(o, 0x83, 0xa9, 0x53, 0x74, 0x61, 0x6b, 0x65, 0x50, 0x6f, 0x6f, 0x6c)
	o, err = z.StakePool.MarshalMsg(o)
	if err != nil {
		err = msgp.WrapError(err, "StakePool")
		return
	}
	// string "TotalOffers"
	o = append(o, 0xab, 0x54, 0x6f, 0x74, 0x61, 0x6c, 0x4f, 0x66, 0x66, 0x65, 0x72, 0x73)
	o, err = z.TotalOffers.MarshalMsg(o)
	if err != nil {
		err = msgp.WrapError(err, "TotalOffers")
		return
	}
	// string "TotalUnStake"
	o = append(o, 0xac, 0x54, 0x6f, 0x74, 0x61, 0x6c, 0x55, 0x6e, 0x53, 0x74, 0x61, 0x6b, 0x65)
	o, err = z.TotalUnStake.MarshalMsg(o)
	if err != nil {
		err = msgp.WrapError(err, "TotalUnStake")
		return
	}
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *stakePool) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var zb0001 uint32
	zb0001, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		err = msgp.WrapError(err)
		return
	}
	for zb0001 > 0 {
		zb0001--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			err = msgp.WrapError(err)
			return
		}
		switch msgp.UnsafeString(field) {
		case "StakePool":
			bts, err = z.StakePool.UnmarshalMsg(bts)
			if err != nil {
				err = msgp.WrapError(err, "StakePool")
				return
			}
		case "TotalOffers":
			bts, err = z.TotalOffers.UnmarshalMsg(bts)
			if err != nil {
				err = msgp.WrapError(err, "TotalOffers")
				return
			}
		case "TotalUnStake":
			bts, err = z.TotalUnStake.UnmarshalMsg(bts)
			if err != nil {
				err = msgp.WrapError(err, "TotalUnStake")
				return
			}
		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				err = msgp.WrapError(err)
				return
			}
		}
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z *stakePool) Msgsize() (s int) {
	s = 1 + 10 + z.StakePool.Msgsize() + 12 + z.TotalOffers.Msgsize() + 13 + z.TotalUnStake.Msgsize()
	return
}
