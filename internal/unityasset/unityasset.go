package unityasset

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
)

const (
	SupportedVersion  = 22
	ClassMonoBehavour = 114
	headerSize        = 48
	maxMetadata       = 64 << 20
	maxCount          = 1 << 20
)

var (
	ErrUnsupported = errors.New("unsupported unity asset file")
	ErrCorrupt     = errors.New("corrupt unity asset file")
)

type Object struct {
	PathID  int64
	Offset  int64
	Size    uint32
	ClassID int32
}

type File struct {
	r            io.ReaderAt
	size         int64
	UnityVersion string
	Objects      []Object
}

type cursor struct {
	b     []byte
	pos   int
	order binary.ByteOrder
	base  int
}

func (c *cursor) need(n int) error {
	if n < 0 || n > len(c.b)-c.pos {
		return fmt.Errorf("%w: metadata ends at %d", ErrCorrupt, c.base+c.pos)
	}
	return nil
}

func (c *cursor) u8() (byte, error) {
	if err := c.need(1); err != nil {
		return 0, err
	}
	c.pos++
	return c.b[c.pos-1], nil
}

func (c *cursor) i16() (int16, error) {
	if err := c.need(2); err != nil {
		return 0, err
	}
	c.pos += 2
	return int16(c.order.Uint16(c.b[c.pos-2:])), nil
}

func (c *cursor) i32() (int32, error) {
	if err := c.need(4); err != nil {
		return 0, err
	}
	c.pos += 4
	return int32(c.order.Uint32(c.b[c.pos-4:])), nil
}

func (c *cursor) i64() (int64, error) {
	if err := c.need(8); err != nil {
		return 0, err
	}
	c.pos += 8
	return int64(c.order.Uint64(c.b[c.pos-8:])), nil
}

func (c *cursor) skip(n int) error {
	if err := c.need(n); err != nil {
		return err
	}
	c.pos += n
	return nil
}

func (c *cursor) cstring() (string, error) {
	i := bytes.IndexByte(c.b[c.pos:], 0)
	if i < 0 {
		return "", fmt.Errorf("%w: unterminated string", ErrCorrupt)
	}
	s := string(c.b[c.pos : c.pos+i])
	c.pos += i + 1
	return s, nil
}

func (c *cursor) align4() {
	abs := c.base + c.pos
	c.pos += (4 - abs%4) % 4
}

func Open(path string) (*File, *os.File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("open assets: %w", err)
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, nil, fmt.Errorf("stat assets: %w", err)
	}
	af, err := Parse(f, st.Size())
	if err != nil {
		f.Close()
		return nil, nil, err
	}
	return af, f, nil
}

func Parse(r io.ReaderAt, size int64) (*File, error) {
	head := make([]byte, headerSize)
	if _, err := r.ReadAt(head, 0); err != nil {
		return nil, fmt.Errorf("%w: header: %w", ErrCorrupt, err)
	}
	version := binary.BigEndian.Uint32(head[8:])
	if version != SupportedVersion {
		return nil, fmt.Errorf("%w: format version %d, only %d is supported", ErrUnsupported, version, SupportedVersion)
	}
	var order binary.ByteOrder = binary.LittleEndian
	if head[16] != 0 {
		order = binary.BigEndian
	}
	metaSize := int64(binary.BigEndian.Uint32(head[20:]))
	fileSize := int64(binary.BigEndian.Uint64(head[24:]))
	dataOffset := int64(binary.BigEndian.Uint64(head[32:]))
	if fileSize != size || metaSize <= 0 || metaSize > maxMetadata || headerSize+metaSize > size || dataOffset < 0 || dataOffset > size {
		return nil, fmt.Errorf("%w: header sizes do not match the file", ErrCorrupt)
	}
	meta := make([]byte, metaSize)
	if _, err := r.ReadAt(meta, headerSize); err != nil {
		return nil, fmt.Errorf("%w: metadata: %w", ErrCorrupt, err)
	}
	c := &cursor{b: meta, order: order, base: headerSize}
	af := &File{r: r, size: size}
	if err := af.readMetadata(c, dataOffset); err != nil {
		return nil, err
	}
	return af, nil
}

func (af *File) readMetadata(c *cursor, dataOffset int64) error {
	var err error
	if af.UnityVersion, err = c.cstring(); err != nil {
		return err
	}
	if _, err := c.i32(); err != nil {
		return err
	}
	typeTree, err := c.u8()
	if err != nil {
		return err
	}
	if typeTree != 0 {
		return fmt.Errorf("%w: files with type trees are not supported", ErrUnsupported)
	}
	typeCount, err := c.i32()
	if err != nil {
		return err
	}
	if typeCount < 0 || typeCount > maxCount {
		return fmt.Errorf("%w: type count %d", ErrCorrupt, typeCount)
	}
	classes := make([]int32, typeCount)
	for i := range classes {
		if classes[i], err = af.readType(c); err != nil {
			return err
		}
	}
	objCount, err := c.i32()
	if err != nil {
		return err
	}
	if objCount < 0 || objCount > maxCount {
		return fmt.Errorf("%w: object count %d", ErrCorrupt, objCount)
	}
	af.Objects = make([]Object, 0, objCount)
	for range objCount {
		o, err := af.readObject(c, dataOffset, classes)
		if err != nil {
			return err
		}
		af.Objects = append(af.Objects, o)
	}
	return nil
}

func (af *File) readType(c *cursor) (int32, error) {
	class, err := c.i32()
	if err != nil {
		return 0, err
	}
	if _, err := c.u8(); err != nil {
		return 0, err
	}
	if _, err := c.i16(); err != nil {
		return 0, err
	}
	if class == ClassMonoBehavour {
		if err := c.skip(16); err != nil {
			return 0, err
		}
	}
	return class, c.skip(16)
}

func (af *File) readObject(c *cursor, dataOffset int64, classes []int32) (Object, error) {
	c.align4()
	var o Object
	var err error
	if o.PathID, err = c.i64(); err != nil {
		return o, err
	}
	start, err := c.i64()
	if err != nil {
		return o, err
	}
	size, err := c.i32()
	if err != nil {
		return o, err
	}
	typeIdx, err := c.i32()
	if err != nil {
		return o, err
	}
	if typeIdx < 0 || int(typeIdx) >= len(classes) {
		return o, fmt.Errorf("%w: object %d has type index %d", ErrCorrupt, o.PathID, typeIdx)
	}
	room := af.size - dataOffset
	if start < 0 || size < 0 || start > room || int64(size) > room-start {
		return o, fmt.Errorf("%w: object %d lies outside the file", ErrCorrupt, o.PathID)
	}
	o.Offset, o.Size, o.ClassID = dataOffset+start, uint32(size), classes[typeIdx]
	return o, nil
}

func (af *File) Read(o Object) ([]byte, error) {
	buf := make([]byte, o.Size)
	if _, err := af.r.ReadAt(buf, o.Offset); err != nil {
		return nil, fmt.Errorf("read object %d: %w", o.PathID, err)
	}
	return buf, nil
}

const monoHeader = 12 + 4 + 12

func MonoBehaviourName(data []byte) (string, int, error) {
	if len(data) < monoHeader+4 {
		return "", 0, fmt.Errorf("%w: MonoBehaviour too short", ErrCorrupt)
	}
	n := int(binary.LittleEndian.Uint32(data[monoHeader:]))
	start := monoHeader + 4
	if n < 0 || n > len(data)-start {
		return "", 0, fmt.Errorf("%w: MonoBehaviour name length %d", ErrCorrupt, n)
	}
	end := start + n
	end += (4 - end%4) % 4
	return string(data[start : start+n]), end, nil
}

func (af *File) FindMonoBehaviours(match func(name string) bool) (map[string][]byte, error) {
	out := map[string][]byte{}
	head := make([]byte, 256)
	for _, o := range af.Objects {
		if o.ClassID != ClassMonoBehavour || o.Size < monoHeader+4 {
			continue
		}
		h := head[:min(int(o.Size), len(head))]
		if _, err := af.r.ReadAt(h, o.Offset); err != nil {
			return nil, fmt.Errorf("read object %d: %w", o.PathID, err)
		}
		name, _, err := MonoBehaviourName(h)
		if err != nil || !match(name) {
			continue
		}
		if _, dup := out[name]; dup {
			return nil, fmt.Errorf("%w: two MonoBehaviours named %q", ErrCorrupt, name)
		}
		data, err := af.Read(o)
		if err != nil {
			return nil, err
		}
		out[name] = data
	}
	return out, nil
}
