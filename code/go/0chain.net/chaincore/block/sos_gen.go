package block

// Code generated by github.com/tinylib/msgp DO NOT EDIT.

import (
	"0chain.net/chaincore/threshold/bls"
	"github.com/tinylib/msgp/msgp"
)

// MarshalMsg implements msgp.Marshaler
func (z *ShareOrSigns) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 2
	// string "ID"
	o = append(o, 0x82, 0xa2, 0x49, 0x44)
	o = msgp.AppendString(o, z.ID)
	// string "ShareOrSigns"
	o = append(o, 0xac, 0x53, 0x68, 0x61, 0x72, 0x65, 0x4f, 0x72, 0x53, 0x69, 0x67, 0x6e, 0x73)
	o = msgp.AppendMapHeader(o, uint32(len(z.ShareOrSigns)))
	keys_za0001 := make([]string, 0, len(z.ShareOrSigns))
	for k := range z.ShareOrSigns {
		keys_za0001 = append(keys_za0001, k)
	}
	msgp.Sort(keys_za0001)
	for _, k := range keys_za0001 {
		za0002 := z.ShareOrSigns[k]
		o = msgp.AppendString(o, k)
		if za0002 == nil {
			o = msgp.AppendNil(o)
		} else {
			o, err = za0002.MarshalMsg(o)
			if err != nil {
				err = msgp.WrapError(err, "ShareOrSigns", k)
				return
			}
		}
	}
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *ShareOrSigns) UnmarshalMsg(bts []byte) (o []byte, err error) {
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
		case "ID":
			z.ID, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "ID")
				return
			}
		case "ShareOrSigns":
			var zb0002 uint32
			zb0002, bts, err = msgp.ReadMapHeaderBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "ShareOrSigns")
				return
			}
			if z.ShareOrSigns == nil {
				z.ShareOrSigns = make(map[string]*bls.DKGKeyShare, zb0002)
			} else if len(z.ShareOrSigns) > 0 {
				for key := range z.ShareOrSigns {
					delete(z.ShareOrSigns, key)
				}
			}
			for zb0002 > 0 {
				var za0001 string
				var za0002 *bls.DKGKeyShare
				zb0002--
				za0001, bts, err = msgp.ReadStringBytes(bts)
				if err != nil {
					err = msgp.WrapError(err, "ShareOrSigns")
					return
				}
				if msgp.IsNil(bts) {
					bts, err = msgp.ReadNilBytes(bts)
					if err != nil {
						return
					}
					za0002 = nil
				} else {
					if za0002 == nil {
						za0002 = new(bls.DKGKeyShare)
					}
					bts, err = za0002.UnmarshalMsg(bts)
					if err != nil {
						err = msgp.WrapError(err, "ShareOrSigns", za0001)
						return
					}
				}
				z.ShareOrSigns[za0001] = za0002
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
func (z *ShareOrSigns) Msgsize() (s int) {
	s = 1 + 3 + msgp.StringPrefixSize + len(z.ID) + 13 + msgp.MapHeaderSize
	if z.ShareOrSigns != nil {
		for za0001, za0002 := range z.ShareOrSigns {
			_ = za0002
			s += msgp.StringPrefixSize + len(za0001)
			if za0002 == nil {
				s += msgp.NilSize
			} else {
				s += za0002.Msgsize()
			}
		}
	}
	return
}
