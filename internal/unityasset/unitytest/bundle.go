package unitytest

import (
	"bytes"
	"encoding/binary"

	"github.com/pierrec/lz4/v4"
)

type BundleFile struct {
	Name string
	Data []byte
}

type BundleSpec struct {
	Files     []BundleFile
	BlockSize int
	Stored    bool
	InfoAtEnd bool
}

func block(src []byte, stored bool) ([]byte, uint16) {
	if stored {
		return src, 0
	}
	dst := make([]byte, lz4.CompressBlockBound(len(src)))
	n, err := lz4.CompressBlock(src, dst, nil)
	if err != nil || n == 0 {
		return src, 0
	}
	return dst[:n], 2
}

func (spec BundleSpec) Build() []byte {
	be := binary.BigEndian
	var data []byte
	type node struct {
		off, size int64
		name      string
	}
	nodes := make([]node, 0, len(spec.Files))
	for _, f := range spec.Files {
		nodes = append(nodes, node{int64(len(data)), int64(len(f.Data)), f.Name})
		data = append(data, f.Data...)
	}
	size := spec.BlockSize
	if size <= 0 {
		size = 1 << 17
	}
	var blocks bytes.Buffer
	var info bytes.Buffer
	info.Write(make([]byte, 16))
	count := (len(data) + size - 1) / size
	_ = binary.Write(&info, be, int32(count))
	for i := 0; i < len(data); i += size {
		chunk := data[i:min(i+size, len(data))]
		c, flag := block(chunk, spec.Stored)
		blocks.Write(c)
		_ = binary.Write(&info, be, uint32(len(chunk)))
		_ = binary.Write(&info, be, uint32(len(c)))
		_ = binary.Write(&info, be, flag)
	}
	_ = binary.Write(&info, be, int32(len(nodes)))
	for _, n := range nodes {
		_ = binary.Write(&info, be, n.off)
		_ = binary.Write(&info, be, n.size)
		_ = binary.Write(&info, be, uint32(0))
		info.WriteString(n.name + "\x00")
	}
	cinfo, method := block(info.Bytes(), spec.Stored)
	flags := uint32(method) | 0x200
	if spec.InfoAtEnd {
		flags |= 0x80
	}
	var out bytes.Buffer
	out.WriteString("UnityFS\x00")
	_ = binary.Write(&out, be, int32(8))
	out.WriteString("5.x.x\x000.0.0\x00")
	_ = binary.Write(&out, be, int64(0))
	_ = binary.Write(&out, be, uint32(len(cinfo)))
	_ = binary.Write(&out, be, uint32(info.Len()))
	_ = binary.Write(&out, be, flags)
	for out.Len()%16 != 0 {
		out.WriteByte(0)
	}
	if !spec.InfoAtEnd {
		out.Write(cinfo)
	}
	for out.Len()%16 != 0 {
		out.WriteByte(0)
	}
	out.Write(blocks.Bytes())
	if spec.InfoAtEnd {
		out.Write(cinfo)
	}
	return out.Bytes()
}
