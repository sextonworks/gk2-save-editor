package unitytest

import (
	"bytes"
	"encoding/binary"
	"slices"
)

func put(buf *bytes.Buffer, v any) {
	if err := binary.Write(buf, binary.LittleEndian, v); err != nil {
		panic(err)
	}
}

type Object struct {
	PathID int64
	Class  int32
	Data   []byte
}

func Mono(name string, payload []byte) []byte {
	b := make([]byte, 28)
	b = binary.LittleEndian.AppendUint32(b, uint32(len(name)))
	b = append(b, name...)
	for len(b)%4 != 0 {
		b = append(b, 0)
	}
	return append(b, payload...)
}

type Builder struct {
	Version  uint32
	TypeTree byte
	Objects  []Object
	BadType  bool
}

func (bl Builder) Build() []byte {
	var meta bytes.Buffer
	meta.WriteString("6000.3.9f1\x00")
	put(&meta, int32(19))
	meta.WriteByte(bl.TypeTree)
	classes := []int32{114, 49}
	put(&meta, int32(len(classes)))
	for _, c := range classes {
		put(&meta, c)
		meta.WriteByte(0)
		put(&meta, int16(-1))
		if c == 114 {
			meta.Write(make([]byte, 16))
		}
		meta.Write(make([]byte, 16))
	}
	put(&meta, int32(len(bl.Objects)))
	var data bytes.Buffer
	for _, o := range bl.Objects {
		for (48+meta.Len())%4 != 0 {
			meta.WriteByte(0)
		}
		put(&meta, o.PathID)
		put(&meta, int64(data.Len()))
		put(&meta, uint32(len(o.Data)))
		idx := int32(0)
		if o.Class != 114 {
			idx = 1
		}
		if bl.BadType {
			idx = 7
		}
		put(&meta, idx)
		data.Write(o.Data)
		for data.Len()%8 != 0 {
			data.WriteByte(0)
		}
	}
	dataOffset := int64(48 + meta.Len())
	total := dataOffset + int64(data.Len())
	head := make([]byte, 48)
	be := binary.BigEndian
	be.PutUint32(head[8:], bl.Version)
	be.PutUint32(head[20:], uint32(meta.Len()))
	be.PutUint64(head[24:], uint64(total))
	be.PutUint64(head[32:], uint64(dataOffset))
	return slices.Concat(head, meta.Bytes(), data.Bytes())
}
