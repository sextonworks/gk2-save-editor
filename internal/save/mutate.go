package save

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/sextonworks/gk2-save-editor/internal/odin"
)

var (
	ErrFull     = errors.New("container is full")
	ErrTemplate = errors.New("no plain item to copy the layout from")
	ErrNoZombie = errors.New("zombie not found")
)

func NewUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(fmt.Errorf("crypto/rand: %w", err))
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func (s *Save) clone() *Save {
	data := make([]byte, len(s.data))
	copy(data, s.data)
	c, err := Load(s.path, data)
	if err != nil {
		panic(fmt.Errorf("reparse a valid save: %w", err))
	}
	return c
}

func (s *Save) reload(data []byte) error {
	root, err := odin.Validate(data)
	if err != nil {
		return fmt.Errorf("validate edited save: %w", err)
	}
	s.data, s.root, s.game = data, root, root.Children[0]
	return nil
}

func (s *Save) Bytes() []byte {
	out := make([]byte, len(s.data))
	copy(out, s.data)
	return out
}

func (s *Save) itemNode(c Container, uniqueID string) (*odin.Node, error) {
	_, arr, err := s.containerArray(c)
	if err != nil {
		return nil, err
	}
	for _, it := range arr.Children {
		if stackOf(it).UniqueID == uniqueID {
			return it, nil
		}
	}
	return nil, fmt.Errorf("%s %s: %w", c, uniqueID, ErrNoItem)
}

func (s *Save) setNumber(n *odin.Node, v float64) error {
	if n.Wire == odin.WireFloat32 || n.Wire == odin.WireFloat64 {
		return odin.SetFloat(s.data, n, v)
	}
	return odin.SetInt(s.data, n, int64(v))
}

func (s *Save) setField(parent *odin.Node, name string, v float64) error {
	n, err := child(parent, name)
	if err != nil {
		return err
	}
	return s.setNumber(n, v)
}

func (s *Save) setString(n *odin.Node, v string) error {
	data, err := odin.SetString(s.data, n, v)
	if err != nil {
		return err
	}
	return s.reload(data)
}

func plainItem(it *odin.Node) bool {
	inner, err := array(it, "inventory")
	if err != nil || len(inner.Children) > 0 {
		return false
	}
	props, err := array(it, "properties")
	return err == nil && len(props.Children) == 0
}

func (s *Save) template(arr *odin.Node, before int) (*odin.Node, error) {
	for _, it := range arr.Children {
		if plainItem(it) {
			return it, nil
		}
	}
	var found *odin.Node
	odin.Walk(s.root, func(n *odin.Node) bool {
		if found != nil || n.Off >= before {
			return false
		}
		if n.Name != "inventory" {
			return true
		}
		if items, ok := n.Array(); ok {
			for _, it := range items.Children {
				if it.End <= before && plainItem(it) {
					found = it
					return false
				}
			}
		}
		return true
	})
	if found == nil {
		return nil, ErrTemplate
	}
	return found, nil
}

func (s *Save) insertItem(c Container, itemID string, count int64, uniqueID string) error {
	holder, arr, err := s.containerArray(c)
	if err != nil {
		return err
	}
	if size := integer(holder, "inventorySize"); int64(len(arr.Children)) >= size {
		return fmt.Errorf("%s %d/%d: %w", c, len(arr.Children), size, ErrFull)
	}
	at := arr.End - 1
	tpl, err := s.template(arr, at)
	if err != nil {
		return fmt.Errorf("%s: %w", c, err)
	}
	blob := make([]byte, tpl.End-tpl.Off)
	copy(blob, s.data[tpl.Off:tpl.End])
	next := odin.MaxRefID(s.root)
	odin.Walk(tpl, func(n *odin.Node) bool {
		if n.HasRefID {
			next++
			binary.LittleEndian.PutUint32(blob[n.RefIDOff-tpl.Off:], uint32(next))
		}
		return true
	})
	countNode, err := child(tpl, "count")
	if err != nil {
		return err
	}
	local := &odin.Node{Wire: countNode.Wire, ValueOff: countNode.ValueOff - tpl.Off}
	if err := odin.SetInt(blob, local, count); err != nil {
		return fmt.Errorf("count: %w", err)
	}
	uidNode, err := child(tpl, "uniqueId", "id")
	if err != nil {
		return err
	}
	idNode, err := child(tpl, "id")
	if err != nil {
		return err
	}
	for _, e := range []struct {
		node  *odin.Node
		value string
	}{{uidNode, uniqueID}, {idNode, itemID}} {
		start, end, err := odin.StringSpan(s.data, e.node)
		if err != nil {
			return err
		}
		blob = odin.Splice(blob, start-tpl.Off, end-tpl.Off, odin.EncodeString(s.data[start], e.value))
	}
	data := make([]byte, len(s.data))
	copy(data, s.data)
	if err := odin.SetArrayLen(data, arr, int64(len(arr.Children)+1)); err != nil {
		return err
	}
	if fill, err := child(holder, "inventoryFillSize"); err == nil && fill.Int >= 0 {
		if err := odin.SetInt(data, fill, fill.Int+1); err != nil {
			return fmt.Errorf("fill size: %w", err)
		}
	}
	return s.reload(odin.Splice(data, at, at, blob))
}

func (s *Save) deleteItem(c Container, uniqueID string) error {
	holder, arr, err := s.containerArray(c)
	if err != nil {
		return err
	}
	it, err := s.itemNode(c, uniqueID)
	if err != nil {
		return err
	}
	data := make([]byte, len(s.data))
	copy(data, s.data)
	if err := odin.SetArrayLen(data, arr, int64(len(arr.Children)-1)); err != nil {
		return err
	}
	if fill, err := child(holder, "inventoryFillSize"); err == nil && fill.Int > 0 {
		if err := odin.SetInt(data, fill, fill.Int-1); err != nil {
			return fmt.Errorf("fill size: %w", err)
		}
	}
	return s.reload(odin.Splice(data, it.Off, it.End, nil))
}

func (s *Save) resourceNode(resType string) (*odin.Node, error) {
	arr, err := array(s.game, "playerData", "res", "resValues")
	if err != nil {
		return nil, err
	}
	for _, a := range arr.Children {
		if str(a, "type") == resType {
			return child(a, "value")
		}
	}
	return nil, fmt.Errorf("resource %q: %w", resType, ErrField)
}

func (s *Save) talentNode(id string) (*odin.Node, error) {
	arr, err := array(s.game, "talentSystemData", "talentData")
	if err != nil {
		return nil, err
	}
	for _, t := range arr.Children {
		if str(t, "id") == id {
			return t, nil
		}
	}
	return nil, fmt.Errorf("talent %q: %w", id, ErrField)
}

func (s *Save) zombieNode(name string) (*odin.Node, error) {
	nodes, err := s.zombieNodes()
	if err != nil {
		return nil, err
	}
	for _, z := range nodes {
		if str(z, "name") == name {
			return z, nil
		}
	}
	return nil, fmt.Errorf("%q: %w", name, ErrNoZombie)
}
