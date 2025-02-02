package stakepool

// Code generated by github.com/tinylib/msgp DO NOT EDIT.

import (
	"github.com/tinylib/msgp/msgp"
)

// MarshalMsg implements msgp.Marshaler
func (z *UserStakePools) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 1
	// string "Pools"
	o = append(o, 0x81, 0xa5, 0x50, 0x6f, 0x6f, 0x6c, 0x73)
	o = msgp.AppendMapHeader(o, uint32(len(z.Pools)))
	keys_za0001 := make([]string, 0, len(z.Pools))
	for k := range z.Pools {
		keys_za0001 = append(keys_za0001, k)
	}
	msgp.Sort(keys_za0001)
	for _, k := range keys_za0001 {
		za0002 := z.Pools[k]
		o = msgp.AppendString(o, k)
		o = msgp.AppendArrayHeader(o, uint32(len(za0002)))
		for za0003 := range za0002 {
			o = msgp.AppendString(o, za0002[za0003])
		}
	}
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *UserStakePools) UnmarshalMsg(bts []byte) (o []byte, err error) {
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
		case "Pools":
			var zb0002 uint32
			zb0002, bts, err = msgp.ReadMapHeaderBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Pools")
				return
			}
			if z.Pools == nil {
				z.Pools = make(map[string][]string, zb0002)
			} else if len(z.Pools) > 0 {
				for key := range z.Pools {
					delete(z.Pools, key)
				}
			}
			for zb0002 > 0 {
				var za0001 string
				var za0002 []string
				zb0002--
				za0001, bts, err = msgp.ReadStringBytes(bts)
				if err != nil {
					err = msgp.WrapError(err, "Pools")
					return
				}
				var zb0003 uint32
				zb0003, bts, err = msgp.ReadArrayHeaderBytes(bts)
				if err != nil {
					err = msgp.WrapError(err, "Pools", za0001)
					return
				}
				if cap(za0002) >= int(zb0003) {
					za0002 = (za0002)[:zb0003]
				} else {
					za0002 = make([]string, zb0003)
				}
				for za0003 := range za0002 {
					za0002[za0003], bts, err = msgp.ReadStringBytes(bts)
					if err != nil {
						err = msgp.WrapError(err, "Pools", za0001, za0003)
						return
					}
				}
				z.Pools[za0001] = za0002
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
func (z *UserStakePools) Msgsize() (s int) {
	s = 1 + 6 + msgp.MapHeaderSize
	if z.Pools != nil {
		for za0001, za0002 := range z.Pools {
			_ = za0002
			s += msgp.StringPrefixSize + len(za0001) + msgp.ArrayHeaderSize
			for za0003 := range za0002 {
				s += msgp.StringPrefixSize + len(za0002[za0003])
			}
		}
	}
	return
}
