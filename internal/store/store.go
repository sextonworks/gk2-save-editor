package store

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"
)

var (
	ErrChangedOnDisk = errors.New("save changed on disk since it was opened")
	ErrNoBackup      = errors.New("backup not found")
	ErrBadBackup     = errors.New("backup is damaged")
)

const manifestName = "manifest.json"

func DataDir(getenv func(string) string, goos, home string) string {
	if v := getenv("GK2_DATA_DIR"); v != "" {
		return v
	}
	switch goos {
	case "windows":
		if v := getenv("LOCALAPPDATA"); v != "" {
			return filepath.Join(v, "gk2")
		}
		return filepath.Join(home, "AppData", "Local", "gk2")
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "gk2")
	}
	if v := getenv("XDG_DATA_HOME"); v != "" {
		return filepath.Join(v, "gk2")
	}
	return filepath.Join(home, ".local", "share", "gk2")
}

func DefaultBackupDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(DataDir(os.Getenv, runtime.GOOS, home), "backups")
}

func Hash(data []byte) [32]byte {
	return sha256.Sum256(data)
}

func WriteAtomic(path string, data []byte, expected [32]byte) error {
	current, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read current save: %w", err)
	}
	if sha256.Sum256(current) != expected {
		return fmt.Errorf("%s: %w", path, ErrChangedOnDisk)
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".gk2-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if st, err := os.Stat(path); err == nil {
		if err := os.Chmod(tmpName, st.Mode().Perm()); err != nil {
			return fmt.Errorf("copy permissions: %w", err)
		}
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace save (is the game or Steam holding the file?): %w", err)
	}
	if err := syncDir(dir); err != nil {
		return fmt.Errorf("save replaced, but flushing the folder failed: %w", err)
	}
	return nil
}

func syncDir(dir string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	d, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("open folder: %w", err)
	}
	defer d.Close()
	if err := d.Sync(); err != nil {
		return fmt.Errorf("sync folder: %w", err)
	}
	return nil
}

type Manifest struct {
	Created time.Time         `json:"created"`
	Source  string            `json:"source"`
	Files   map[string]string `json:"files"`
}

type Backup struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	Created time.Time `json:"created"`
	Files   []string  `json:"files"`
	Legacy  bool      `json:"legacy"`
}

type Store struct {
	Dir  string
	Keep int
	Now  func() time.Time
	Warn func(error)
}

func New(dir string) *Store {
	return &Store{Dir: dir, Keep: 50, Now: time.Now, Warn: func(error) {}}
}

func pairFiles(datPath string) []string {
	files := []string{datPath}
	info := strings.TrimSuffix(datPath, filepath.Ext(datPath)) + ".info"
	if _, err := os.Stat(info); err == nil {
		files = append(files, info)
	}
	return files
}

func (s *Store) Create(datPath string) (Backup, error) {
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return Backup{}, fmt.Errorf("create backup folder: %w", err)
	}
	now := s.Now()
	base := now.Format("20060102_150405")
	name := base + ".zip"
	for i := 1; ; i++ {
		if _, err := os.Stat(filepath.Join(s.Dir, name)); os.IsNotExist(err) {
			break
		}
		name = fmt.Sprintf("%s_%d.zip", base, i)
	}
	contents := map[string][]byte{}
	man := Manifest{Created: now.UTC(), Source: filepath.Dir(datPath), Files: map[string]string{}}
	for _, f := range pairFiles(datPath) {
		data, err := os.ReadFile(f)
		if err != nil {
			return Backup{}, fmt.Errorf("read %s: %w", f, err)
		}
		sum := sha256.Sum256(data)
		contents[filepath.Base(f)] = data
		man.Files[filepath.Base(f)] = hex.EncodeToString(sum[:])
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, fname := range slices.Sorted(mapKeys(contents)) {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: fname, Method: zip.Deflate, Modified: now})
		if err != nil {
			return Backup{}, fmt.Errorf("zip %s: %w", fname, err)
		}
		if _, err := w.Write(contents[fname]); err != nil {
			return Backup{}, fmt.Errorf("zip %s: %w", fname, err)
		}
	}
	mw, err := zw.CreateHeader(&zip.FileHeader{Name: manifestName, Method: zip.Deflate, Modified: now})
	if err != nil {
		return Backup{}, fmt.Errorf("zip manifest: %w", err)
	}
	if err := json.NewEncoder(mw).Encode(man); err != nil {
		return Backup{}, fmt.Errorf("zip manifest: %w", err)
	}
	if err := zw.Close(); err != nil {
		return Backup{}, fmt.Errorf("finish zip: %w", err)
	}
	path := filepath.Join(s.Dir, name)
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		return Backup{}, fmt.Errorf("write backup: %w", err)
	}
	b, _, err := s.read(path)
	if err != nil {
		os.Remove(path)
		return Backup{}, err
	}
	s.prune()
	return b, nil
}

func mapKeys(m map[string][]byte) func(func(string) bool) {
	return func(yield func(string) bool) {
		for k := range m {
			if !yield(k) {
				return
			}
		}
	}
}

func (s *Store) read(path string) (Backup, map[string][]byte, error) {
	b := Backup{Name: filepath.Base(path), Path: path}
	files := map[string][]byte{}
	st, err := os.Stat(path)
	if err != nil {
		return b, nil, fmt.Errorf("%s: %w", b.Name, ErrNoBackup)
	}
	if st.IsDir() {
		b.Legacy, b.Created = true, st.ModTime()
		entries, err := os.ReadDir(path)
		if err != nil {
			return b, nil, fmt.Errorf("read %s: %w", b.Name, err)
		}
		for _, e := range entries {
			if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(path, e.Name()))
			if err != nil {
				return b, nil, fmt.Errorf("read %s: %w", e.Name(), err)
			}
			files[e.Name()] = data
			b.Files = append(b.Files, e.Name())
		}
		return b, files, nil
	}
	zr, err := zip.OpenReader(path)
	if err != nil {
		return b, nil, fmt.Errorf("%s: %w: %w", b.Name, ErrBadBackup, err)
	}
	defer zr.Close()
	var man Manifest
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			return b, nil, fmt.Errorf("%s/%s: %w: %w", b.Name, f.Name, ErrBadBackup, err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return b, nil, fmt.Errorf("%s/%s: %w: %w", b.Name, f.Name, ErrBadBackup, err)
		}
		if f.Name == manifestName {
			if err := json.Unmarshal(data, &man); err != nil {
				return b, nil, fmt.Errorf("%s manifest: %w: %w", b.Name, ErrBadBackup, err)
			}
			continue
		}
		files[f.Name] = data
		b.Files = append(b.Files, f.Name)
	}
	if len(man.Files) != len(files) {
		return b, nil, fmt.Errorf("%s: manifest lists %d files, archive has %d: %w", b.Name, len(man.Files), len(files), ErrBadBackup)
	}
	for fname, want := range man.Files {
		sum := sha256.Sum256(files[fname])
		if hex.EncodeToString(sum[:]) != want {
			return b, nil, fmt.Errorf("%s/%s: checksum mismatch: %w", b.Name, fname, ErrBadBackup)
		}
	}
	b.Created = man.Created
	slices.Sort(b.Files)
	return b, files, nil
}

func (s *Store) List() ([]Backup, error) {
	entries, err := os.ReadDir(s.Dir)
	if os.IsNotExist(err) {
		return []Backup{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list backups: %w", err)
	}
	out := make([]Backup, 0, len(entries))
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") || (!e.IsDir() && filepath.Ext(e.Name()) != ".zip") {
			continue
		}
		b, _, err := s.read(filepath.Join(s.Dir, e.Name()))
		if err != nil {
			continue
		}
		out = append(out, b)
	}
	slices.SortFunc(out, func(a, b Backup) int { return strings.Compare(a.Name, b.Name) })
	return out, nil
}

func (s *Store) Restore(name, saveDir string) ([]string, error) {
	if name != filepath.Base(name) {
		return nil, fmt.Errorf("%s: %w", name, ErrNoBackup)
	}
	_, files, err := s.read(filepath.Join(s.Dir, name))
	if err != nil {
		return nil, err
	}
	restored := make([]string, 0, len(files))
	for _, fname := range slices.Sorted(mapKeys(files)) {
		if fname != filepath.Base(fname) || filepath.Ext(fname) != ".dat" && filepath.Ext(fname) != ".info" {
			continue
		}
		target := filepath.Join(saveDir, fname)
		cur, err := os.ReadFile(target)
		if os.IsNotExist(err) {
			if err := os.WriteFile(target, nil, 0o600); err != nil {
				return restored, fmt.Errorf("create %s: %w", fname, err)
			}
			cur, err = nil, nil
		}
		if err != nil {
			return restored, fmt.Errorf("read %s: %w", fname, err)
		}
		if err := WriteAtomic(target, files[fname], sha256.Sum256(cur)); err != nil {
			return restored, err
		}
		restored = append(restored, fname)
	}
	return restored, nil
}

func (s *Store) prune() {
	if s.Keep <= 0 {
		return
	}
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		s.Warn(fmt.Errorf("list backups for pruning: %w", err))
		return
	}
	zips := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".zip" {
			zips = append(zips, e.Name())
		}
	}
	slices.Sort(zips)
	for len(zips) > s.Keep {
		if err := os.Remove(filepath.Join(s.Dir, zips[0])); err != nil {
			s.Warn(fmt.Errorf("remove old backup: %w", err))
		}
		zips = zips[1:]
	}
}
