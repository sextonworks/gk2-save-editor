package gamedata

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/sextonworks/gk2-save-editor/internal/unityasset"
)

const (
	schemaVersion = 3
	balanceName   = "GameBalance"
	English       = "en"
)

var (
	ErrNoBalance  = errors.New("item definitions not found in the game files")
	ErrBadStrings = errors.New("unexpected localization layout")

	Languages = []string{"en", "ru", "zh_cn"}

	idRe          = regexp.MustCompile(`([\x03-\x60])\x00\x00\x00([a-z][a-z0-9_:]+)`)
	balanceMarker = [][]byte{[]byte("\x05\x00\x00\x00u_bag"), []byte("\x07\x00\x00\x00sword_0")}
)

type Catalog struct {
	Schema  int                          `json:"schema"`
	Source  string                       `json:"source"`
	IDs     []string                     `json:"ids"`
	Items   []string                     `json:"items"`
	Names   map[string]map[string]string `json:"names"`
	Aliases map[string]string            `json:"aliases"`

	known map[string]bool
	items map[string]bool
}

type Entry struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	IsItem bool   `json:"isItem"`
}

func ResourcesPath(gameDir string) string {
	return filepath.Join(gameDir, "GraveyardKeeper2_Data", "resources.assets")
}

func Load(gameDir, cacheDir string, refresh bool) (*Catalog, error) {
	path := ResourcesPath(gameDir)
	sum, err := fileHash(path)
	if err != nil {
		return nil, err
	}
	cachePath := filepath.Join(cacheDir, fmt.Sprintf("catalog-v%d.json", schemaVersion))
	if !refresh {
		if c, err := readCache(cachePath, sum); err == nil {
			return c, nil
		}
	}
	c, err := Extract(path)
	if err != nil {
		return nil, err
	}
	c.Source = sum
	if err := writeCache(cachePath, c); err != nil {
		return nil, err
	}
	return c, nil
}

func fileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open game data: %w", err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hash game data: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func readCache(path, sum string) (*Catalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read catalog cache: %w", err)
	}
	var c Catalog
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("decode catalog cache: %w", err)
	}
	if c.Schema != schemaVersion || c.Source != sum {
		return nil, errors.New("catalog cache is stale")
	}
	c.index()
	return &c, nil
}

func writeCache(path string, c *Catalog) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create cache folder: %w", err)
	}
	data, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("encode catalog cache: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write catalog cache: %w", err)
	}
	return nil
}

func Extract(resourcesPath string) (*Catalog, error) {
	af, f, err := unityasset.Open(resourcesPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	wanted := map[string]bool{balanceName: true}
	for _, l := range Languages {
		wanted["lng_"+l] = true
	}
	found, err := af.FindMonoBehaviours(func(n string) bool { return wanted[n] })
	if err != nil {
		return nil, err
	}
	balance, ok := found[balanceName]
	if !ok || !hasMarkers(balance) {
		return nil, ErrNoBalance
	}
	c := &Catalog{Schema: schemaVersion, Names: map[string]map[string]string{}, Aliases: map[string]string{}}
	c.IDs, c.Items = scanIDs(balance)
	for _, l := range Languages {
		raw, ok := found["lng_"+l]
		if !ok {
			continue
		}
		table, aliases, err := parseLanguage(raw)
		if err != nil {
			return nil, fmt.Errorf("lng_%s: %w", l, err)
		}
		c.Names[l] = table
		if l == English {
			c.Aliases = aliases
		}
	}
	c.index()
	return c, nil
}

func hasMarkers(b []byte) bool {
	for _, m := range balanceMarker {
		if !bytes.Contains(b, m) {
			return false
		}
	}
	return true
}

func scanIDs(balance []byte) (ids, items []string) {
	seen := map[string]bool{}
	isItem := map[string]bool{}
	for _, m := range idRe.FindAllSubmatchIndex(balance, -1) {
		n := int(balance[m[2]])
		start := m[4]
		if m[5]-start < n {
			continue
		}
		id := string(balance[start : start+n])
		seen[id] = true
		if slices.Contains(followingStrings(balance, start+n, 8), "u_bag") {
			isItem[id] = true
		}
	}
	ids = make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	items = make([]string, 0, len(isItem))
	for id := range isItem {
		items = append(items, id)
	}
	slices.Sort(ids)
	slices.Sort(items)
	return ids, items
}

func followingStrings(b []byte, pos, limit int) []string {
	pos += (4 - pos%4) % 4
	if len(b)-pos < 4 {
		return nil
	}
	count := int(binary.LittleEndian.Uint32(b[pos:]))
	if count < 1 || count > limit {
		return nil
	}
	pos += 4
	out := make([]string, 0, count)
	for range count {
		if len(b)-pos < 4 {
			return nil
		}
		n := int(binary.LittleEndian.Uint32(b[pos:]))
		if n < 1 || n > 96 || n > len(b)-pos-4 {
			return nil
		}
		raw := b[pos+4 : pos+4+n]
		if !printable(raw) {
			return nil
		}
		out = append(out, string(raw))
		pos += 4 + n
		pos += (4 - pos%4) % 4
	}
	return out
}

func printable(b []byte) bool {
	for _, c := range b {
		if c < 0x20 || c > 0x7e {
			return false
		}
	}
	return true
}

var (
	markupRe  = regexp.MustCompile(`<[^>]*>`)
	invisible = strings.NewReplacer("\u200b", "", "\u00a0", " ")
)

func Clean(s string) string {
	return strings.TrimSpace(invisible.Replace(markupRe.ReplaceAllString(s, "")))
}

type strReader struct {
	b   []byte
	pos int
}

func (r *strReader) str() (string, error) {
	if len(r.b)-r.pos < 4 {
		return "", ErrBadStrings
	}
	n := int(binary.LittleEndian.Uint32(r.b[r.pos:]))
	r.pos += 4
	if n < 0 || n > len(r.b)-r.pos {
		return "", ErrBadStrings
	}
	s := string(r.b[r.pos : r.pos+n])
	r.pos += n
	r.pos += (4 - r.pos%4) % 4
	return s, nil
}

func (r *strReader) strs() ([]string, error) {
	if len(r.b)-r.pos < 4 {
		return nil, ErrBadStrings
	}
	n := int(binary.LittleEndian.Uint32(r.b[r.pos:]))
	r.pos += 4
	if n < 0 || n > (len(r.b)-r.pos)/4 {
		return nil, ErrBadStrings
	}
	out := make([]string, 0, n)
	for range n {
		s, err := r.str()
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

func parseLanguage(raw []byte) (map[string]string, map[string]string, error) {
	_, off, err := unityasset.MonoBehaviourName(raw)
	if err != nil {
		return nil, nil, err
	}
	r := &strReader{b: raw, pos: off}
	aliasKeys, err := r.strs()
	if err != nil {
		return nil, nil, err
	}
	aliasTargets, err := r.strs()
	if err != nil {
		return nil, nil, err
	}
	if len(aliasKeys) != len(aliasTargets) {
		return nil, nil, fmt.Errorf("%w: %d alias keys, %d targets", ErrBadStrings, len(aliasKeys), len(aliasTargets))
	}
	if _, err := r.str(); err != nil {
		return nil, nil, err
	}
	keys, err := r.strs()
	if err != nil {
		return nil, nil, err
	}
	values, err := r.strs()
	if err != nil {
		return nil, nil, err
	}
	if len(keys) != len(values) {
		return nil, nil, fmt.Errorf("%w: %d keys, %d values", ErrBadStrings, len(keys), len(values))
	}
	table := make(map[string]string, len(keys))
	for i, k := range keys {
		table[k] = Clean(values[i])
	}
	aliases := make(map[string]string, len(aliasKeys))
	for i, k := range aliasKeys {
		aliases[k] = aliasTargets[i]
	}
	return table, aliases, nil
}

func (c *Catalog) index() {
	c.known = make(map[string]bool, len(c.IDs))
	for _, id := range c.IDs {
		c.known[id] = true
	}
	c.items = make(map[string]bool, len(c.Items))
	for _, id := range c.Items {
		c.items[id] = true
	}
}

func (c *Catalog) Known(id string) bool {
	return c.known[id]
}

func (c *Catalog) IsItem(id string) bool {
	if c.items[id] {
		return true
	}
	base, _, found := strings.Cut(id, ":")
	if !found {
		return false
	}
	if c.items[base] {
		return true
	}
	_, named := c.lookup(id, English)
	return named
}

func (c *Catalog) lookup(id, lang string) (string, bool) {
	table := c.Names[lang]
	if v, ok := table[id]; ok && v != "" {
		return v, true
	}
	if target, ok := c.Aliases[id]; ok {
		if v, ok := table[target]; ok && v != "" {
			return v, true
		}
	}
	base, _, found := strings.Cut(id, ":")
	if found {
		if v, ok := table[base]; ok && v != "" {
			return v, true
		}
	}
	return "", false
}

func (c *Catalog) Name(id, lang string) string {
	if v, ok := c.lookup(id, lang); ok {
		return v
	}
	if v, ok := c.lookup(id, English); ok {
		return v
	}
	return id
}

var firstLevelSuffix = regexp.MustCompile(`\s*I$`)

func (c *Catalog) BaseName(id, lang string) string {
	if n := c.Name(id, lang); n != id {
		return n
	}
	if n := c.Name(id+"_1", lang); n != id+"_1" {
		return firstLevelSuffix.ReplaceAllString(n, "")
	}
	return id
}

func (c *Catalog) Description(id, lang string) string {
	for _, key := range []string{id + "_d", id + "_1_d"} {
		if n := c.Name(key, lang); n != key {
			return n
		}
	}
	return ""
}

func (c *Catalog) Search(query, lang string, itemsOnly bool, limit int) []Entry {
	q := strings.ToLower(strings.TrimSpace(query))
	out := make([]Entry, 0, min(limit, 64))
	pool := c.IDs
	if itemsOnly {
		pool = c.Items
	}
	for _, id := range pool {
		name := c.Name(id, lang)
		if q != "" && !strings.Contains(id, q) && !strings.Contains(strings.ToLower(name), q) {
			continue
		}
		out = append(out, Entry{ID: id, Name: name, IsItem: c.IsItem(id)})
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}
