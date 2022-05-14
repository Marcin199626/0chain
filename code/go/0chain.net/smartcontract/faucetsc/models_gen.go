package faucetsc

// Code generated by github.com/tinylib/msgp DO NOT EDIT.

import (
	"github.com/tinylib/msgp/msgp"
)

// MarshalMsg implements msgp.Marshaler
func (z *GlobalNode) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 4
	// string "FaucetConfig"
	o = append(o, 0x84, 0xac, 0x46, 0x61, 0x75, 0x63, 0x65, 0x74, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67)
	if z.FaucetConfig == nil {
		o = msgp.AppendNil(o)
	} else {
		o, err = z.FaucetConfig.MarshalMsg(o)
		if err != nil {
			err = msgp.WrapError(err, "FaucetConfig")
			return
		}
	}
	// string "ID"
	o = append(o, 0xa2, 0x49, 0x44)
	o = msgp.AppendString(o, z.ID)
	// string "Used"
	o = append(o, 0xa4, 0x55, 0x73, 0x65, 0x64)
	o = msgp.AppendInt64(o, z.Used)
	// string "StartTime"
	o = append(o, 0xa9, 0x53, 0x74, 0x61, 0x72, 0x74, 0x54, 0x69, 0x6d, 0x65)
	o = msgp.AppendTime(o, z.StartTime)
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *GlobalNode) UnmarshalMsg(bts []byte) (o []byte, err error) {
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
		case "FaucetConfig":
			if msgp.IsNil(bts) {
				bts, err = msgp.ReadNilBytes(bts)
				if err != nil {
					return
				}
				z.FaucetConfig = nil
			} else {
				if z.FaucetConfig == nil {
					z.FaucetConfig = new(FaucetConfig)
				}
				bts, err = z.FaucetConfig.UnmarshalMsg(bts)
				if err != nil {
					err = msgp.WrapError(err, "FaucetConfig")
					return
				}
			}
		case "ID":
			z.ID, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "ID")
				return
			}
		case "Used":
			z.Used, bts, err = msgp.ReadInt64Bytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Used")
				return
			}
		case "StartTime":
			z.StartTime, bts, err = msgp.ReadTimeBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "StartTime")
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
func (z *GlobalNode) Msgsize() (s int) {
	s = 1 + 13
	if z.FaucetConfig == nil {
		s += msgp.NilSize
	} else {
		s += z.FaucetConfig.Msgsize()
	}
	s += 3 + msgp.StringPrefixSize + len(z.ID) + 5 + msgp.Int64Size + 10 + msgp.TimeSize
	return
}

// MarshalMsg implements msgp.Marshaler
func (z UserNode) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 3
	// string "ID"
	o = append(o, 0x83, 0xa2, 0x49, 0x44)
	o = msgp.AppendString(o, z.ID)
	// string "StartTime"
	o = append(o, 0xa9, 0x53, 0x74, 0x61, 0x72, 0x74, 0x54, 0x69, 0x6d, 0x65)
	o = msgp.AppendTime(o, z.StartTime)
	// string "Used"
	o = append(o, 0xa4, 0x55, 0x73, 0x65, 0x64)
	o = msgp.AppendInt64(o, z.Used)
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *UserNode) UnmarshalMsg(bts []byte) (o []byte, err error) {
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
		case "StartTime":
			z.StartTime, bts, err = msgp.ReadTimeBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "StartTime")
				return
			}
		case "Used":
			z.Used, bts, err = msgp.ReadInt64Bytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Used")
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
func (z UserNode) Msgsize() (s int) {
	s = 1 + 3 + msgp.StringPrefixSize + len(z.ID) + 10 + msgp.TimeSize + 5 + msgp.Int64Size
	return
}
