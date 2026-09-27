package unityasset

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"strings"

	"github.com/pierrec/lz4/v4"
)

const (
	bundleSignature    = "UnityFS"
	compressionMask    = 0x3f
	compressNone       = 0
	compressLZ4        = 2
	compressLZ4HC      = 3
	flagInfoAtEnd      = 0x80
	flagPaddingAtStart = 0x200
	maxBlockInfo       = 16 << 20
	maxBundleData      = 1 << 30
)

type bundleBlock struct {
	usize, csize uint32
	flags        uint16
}

type bundleNode struct {
	offset, size int64
	path         string
}

type Bundle struct {
	files map[string][]byte
	names []string
}

func (b *Bundle) Names() []string { return b.names }

func (b *Bundle) File(name string) ([]byte, bool) {
	data, ok := b.files[name]
	return data, ok
}

func (b *Bundle) Assets() (*File, []byte, error) {
	for _, n := range b.names {
		if strings.HasSuffix(n, ".resS") || strings.HasSuffix(n, ".resource") {
			continue
		}
		data := b.files[n]
		af, err := Parse(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return nil, nil, fmt.Errorf("bundle file %s: %w", n, err)
		}
		return af, data, nil
	}
	return nil, nil, fmt.Errorf("%w: bundle has no serialized file", ErrCorrupt)
}

func (b *Bundle) Stream(path string) ([]byte, bool) {
	return b.File(path[strings.LastIndex(path, "/")+1:])
}

func OpenBundle(path string) (*Bundle, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read bundle: %w", err)
	}
	return ParseBundle(raw)
}

func PeekBundle(path string, limit int) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read bundle: %w", err)
	}
	blocks, nodes, dataStart, err := bundleLayout(raw)
	if err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		return nil, nil
	}
	var out []byte
	pos := dataStart
	for _, blk := range blocks {
		if len(out) >= limit {
			break
		}
		chunk, err := inflate(raw, pos, blk)
		if err != nil {
			return nil, err
		}
		out = append(out, chunk...)
		pos += int(blk.csize)
	}
	start := nodes[0].offset
	if start > int64(len(out)) {
		return nil, nil
	}
	return out[start:], nil
}

func ParseBundle(raw []byte) (*Bundle, error) {
	blocks, nodes, dataStart, err := bundleLayout(raw)
	if err != nil {
		return nil, err
	}
	var total int64
	for _, blk := range blocks {
		total += int64(blk.usize)
	}
	if total > maxBundleData {
		return nil, fmt.Errorf("%w: bundle data too large", ErrUnsupported)
	}
	data := make([]byte, 0, total)
	pos := dataStart
	for _, blk := range blocks {
		chunk, err := inflate(raw, pos, blk)
		if err != nil {
			return nil, err
		}
		data = append(data, chunk...)
		pos += int(blk.csize)
	}
	b := &Bundle{files: map[string][]byte{}}
	for _, n := range nodes {
		if n.offset < 0 || n.size < 0 || n.offset > int64(len(data)) || n.size > int64(len(data))-n.offset {
			return nil, fmt.Errorf("%w: bundle entry %s lies outside the data", ErrCorrupt, n.path)
		}
		b.files[n.path] = data[n.offset : n.offset+n.size]
		b.names = append(b.names, n.path)
	}
	return b, nil
}

type bundleHeader struct {
	version, csize, usize, flags int32
}

func readBundleHeader(c *cursor) (bundleHeader, error) {
	var h bundleHeader
	sig, err := c.cstring()
	if err != nil {
		return h, err
	}
	if sig != bundleSignature {
		return h, fmt.Errorf("%w: not a UnityFS bundle", ErrUnsupported)
	}
	if h.version, err = c.i32(); err != nil {
		return h, err
	}
	if h.version < 6 || h.version > 8 {
		return h, fmt.Errorf("%w: bundle version %d", ErrUnsupported, h.version)
	}
	for range 2 {
		if _, err := c.cstring(); err != nil {
			return h, err
		}
	}
	if _, err := c.i64(); err != nil {
		return h, err
	}
	for _, dst := range []*int32{&h.csize, &h.usize, &h.flags} {
		if *dst, err = c.i32(); err != nil {
			return h, err
		}
	}
	if h.csize < 0 || h.usize < 0 || h.usize > maxBlockInfo {
		return h, fmt.Errorf("%w: block info size", ErrCorrupt)
	}
	if h.version >= 7 {
		c.pos += (16 - c.pos%16) % 16
	}
	return h, nil
}

func bundleLayout(raw []byte) ([]bundleBlock, []bundleNode, int, error) {
	c := &cursor{b: raw, order: binary.BigEndian}
	h, err := readBundleHeader(c)
	if err != nil {
		return nil, nil, 0, err
	}
	infoAt, dataStart := c.pos, c.pos
	if h.flags&flagInfoAtEnd != 0 {
		infoAt = len(raw) - int(h.csize)
	} else {
		dataStart = infoAt + int(h.csize)
	}
	if infoAt < 0 || int(h.csize) > len(raw)-infoAt {
		return nil, nil, 0, fmt.Errorf("%w: block info outside the file", ErrCorrupt)
	}
	info, err := decompress(raw[infoAt:infoAt+int(h.csize)], int(h.usize), int(h.flags)&compressionMask)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("block info: %w", err)
	}
	if h.flags&flagPaddingAtStart != 0 {
		dataStart += (16 - dataStart%16) % 16
	}
	blocks, nodes, err := readBlockInfo(info)
	if err != nil {
		return nil, nil, 0, err
	}
	return blocks, nodes, dataStart, nil
}

func readBlockInfo(info []byte) ([]bundleBlock, []bundleNode, error) {
	c := &cursor{b: info, order: binary.BigEndian}
	if err := c.skip(16); err != nil {
		return nil, nil, err
	}
	nb, err := c.i32()
	if err != nil {
		return nil, nil, err
	}
	if nb < 0 || nb > maxCount {
		return nil, nil, fmt.Errorf("%w: block count %d", ErrCorrupt, nb)
	}
	blocks := make([]bundleBlock, nb)
	for i := range blocks {
		u, err := c.i32()
		if err != nil {
			return nil, nil, err
		}
		cs, err := c.i32()
		if err != nil {
			return nil, nil, err
		}
		f, err := c.i16()
		if err != nil {
			return nil, nil, err
		}
		blocks[i] = bundleBlock{uint32(u), uint32(cs), uint16(f)}
	}
	nn, err := c.i32()
	if err != nil {
		return nil, nil, err
	}
	if nn < 0 || nn > maxCount {
		return nil, nil, fmt.Errorf("%w: node count %d", ErrCorrupt, nn)
	}
	nodes := make([]bundleNode, nn)
	for i := range nodes {
		off, err := c.i64()
		if err != nil {
			return nil, nil, err
		}
		size, err := c.i64()
		if err != nil {
			return nil, nil, err
		}
		if _, err := c.i32(); err != nil {
			return nil, nil, err
		}
		p, err := c.cstring()
		if err != nil {
			return nil, nil, err
		}
		nodes[i] = bundleNode{off, size, p}
	}
	return blocks, nodes, nil
}

func inflate(raw []byte, pos int, blk bundleBlock) ([]byte, error) {
	if pos < 0 || int64(blk.csize) > int64(len(raw)-pos) {
		return nil, fmt.Errorf("%w: block outside the file", ErrCorrupt)
	}
	return decompress(raw[pos:pos+int(blk.csize)], int(blk.usize), int(blk.flags)&compressionMask)
}

func decompress(src []byte, usize, method int) ([]byte, error) {
	switch method {
	case compressNone:
		if len(src) != usize {
			return nil, fmt.Errorf("%w: stored block size", ErrCorrupt)
		}
		return src, nil
	case compressLZ4, compressLZ4HC:
		dst := make([]byte, usize)
		n, err := lz4.UncompressBlock(src, dst)
		if err != nil {
			return nil, fmt.Errorf("%w: lz4: %w", ErrCorrupt, err)
		}
		if n != usize {
			return nil, fmt.Errorf("%w: lz4 size %d, want %d", ErrCorrupt, n, usize)
		}
		return dst, nil
	default:
		return nil, fmt.Errorf("%w: compression method %d", ErrUnsupported, method)
	}
}
