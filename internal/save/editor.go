package save

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
)

var ErrNothingToUndo = errors.New("nothing to undo")

type Editor struct {
	path     string
	original []byte
	origHash [32]byte
	ops      []Op
	cursor   int
	saved    int
	cur      *Save
}

func OpenEditor(path string) (*Editor, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read save: %w", err)
	}
	return NewEditor(path, data)
}

func NewEditor(path string, data []byte) (*Editor, error) {
	s, err := Load(path, data)
	if err != nil {
		return nil, err
	}
	orig := make([]byte, len(data))
	copy(orig, data)
	return &Editor{path: path, original: orig, origHash: sha256.Sum256(orig), cur: s}, nil
}

func (e *Editor) Save() *Save {
	return e.cur
}

func (e *Editor) OriginalHash() [32]byte {
	return e.origHash
}

func (e *Editor) Apply(op Op) error {
	next := e.cur.clone()
	if err := op.Apply(next); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	if err := next.reload(next.data); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	if bytes.Equal(next.data, e.cur.data) {
		return nil
	}
	e.ops = append(e.ops[:e.cursor], op)
	e.cursor++
	e.cur = next
	return nil
}

func (e *Editor) replay(n int) (*Save, error) {
	s, err := Load(e.path, append([]byte(nil), e.original...))
	if err != nil {
		return nil, err
	}
	for _, op := range e.ops[:n] {
		if err := op.Apply(s); err != nil {
			return nil, fmt.Errorf("replay %s: %w", op, err)
		}
	}
	return s, nil
}

func (e *Editor) Undo() error {
	if e.cursor == 0 {
		return ErrNothingToUndo
	}
	s, err := e.replay(e.cursor - 1)
	if err != nil {
		return err
	}
	e.cursor--
	e.cur = s
	return nil
}

func (e *Editor) Redo() error {
	if e.cursor == len(e.ops) {
		return ErrNothingToUndo
	}
	s, err := e.replay(e.cursor + 1)
	if err != nil {
		return err
	}
	e.cursor++
	e.cur = s
	return nil
}

func (e *Editor) CanUndo() bool {
	return e.cursor > 0
}

func (e *Editor) CanRedo() bool {
	return e.cursor < len(e.ops)
}

func (e *Editor) Dirty() bool {
	return e.cursor != e.saved
}

func (e *Editor) Ops() []Op {
	out := make([]Op, e.cursor)
	copy(out, e.ops[:e.cursor])
	return out
}

func (e *Editor) Original() (*Save, error) {
	return Load(e.path, append([]byte(nil), e.original...))
}

func (e *Editor) Changes() []string {
	out := make([]string, 0, e.cursor)
	for _, op := range e.ops[:e.cursor] {
		out = append(out, op.String())
	}
	return out
}

func (e *Editor) MarkSaved(data []byte) {
	e.original = append([]byte(nil), data...)
	e.origHash = sha256.Sum256(e.original)
	e.ops = e.ops[e.cursor:e.cursor]
	e.cursor, e.saved = 0, 0
}
