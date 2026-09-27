package gamedata

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"

	"github.com/sextonworks/gk2-save-editor/internal/typetree"
	"github.com/sextonworks/gk2-save-editor/internal/unityasset"
)

const (
	iconsVersion      = 1
	classTexture2D    = 28
	classSprite       = 213
	classSpriteAtlas  = 687078895
	formatAlpha8      = 1
	formatRGB24       = 3
	formatRGBA32      = 4
	formatARGB32      = 5
	settingsPacked    = 1
	settingsRotation  = 0xf << 2
	bundlePeekLimit   = 64 << 10
	iconAtlasPrefix   = "Icons"
	maxTextureSide    = 8192
	iconsReadyMarker  = ".complete"
	iconFileExtension = ".png"
)

var (
	ErrNoIcon     = errors.New("icon not found")
	ErrBadTexture = errors.New("unsupported texture")

	iconNameRe = regexp.MustCompile(`^[A-Za-z0-9_\-. ()]+$`)
)

type renderData struct {
	texture   int64
	rect      [4]float64
	offset    [2]float64
	settings  int64
	canvasW   int
	canvasH   int
	hasCanvas bool
}

type textureSource struct {
	af     *unityasset.File
	stream func(path string, offset, size int64) ([]byte, error)
	cache  map[int64]*image.NRGBA
}

func IconsDir(cacheDir, source string) string {
	return filepath.Join(cacheDir, fmt.Sprintf("icons-v%d-%s", iconsVersion, source[:min(16, len(source))]))
}

func ValidIconName(name string) bool {
	return iconNameRe.MatchString(name) && !strings.Contains(name, "..")
}

func IconPath(dir, name string) (string, error) {
	if !ValidIconName(name) {
		return "", fmt.Errorf("%w: %q", ErrNoIcon, name)
	}
	p := filepath.Join(dir, name+iconFileExtension)
	if _, err := os.Stat(p); err != nil {
		return "", fmt.Errorf("%w: %q", ErrNoIcon, name)
	}
	return p, nil
}

func ExtractIcons(ctx context.Context, gameDir, outDir string) (int, error) {
	if _, err := os.Stat(filepath.Join(outDir, iconsReadyMarker)); err == nil {
		entries, err := os.ReadDir(outDir)
		if err != nil {
			return 0, fmt.Errorf("read icons: %w", err)
		}
		return len(entries) - 1, nil
	}
	tmp := outDir + ".tmp"
	if err := os.RemoveAll(tmp); err != nil {
		return 0, fmt.Errorf("clean icons: %w", err)
	}
	if err := os.MkdirAll(tmp, 0o700); err != nil {
		return 0, fmt.Errorf("create icons folder: %w", err)
	}
	data := filepath.Join(gameDir, "GraveyardKeeper2_Data")
	total := 0
	for _, name := range []string{"resources.assets", "sharedassets0.assets"} {
		n, err := iconsFromAssets(filepath.Join(data, name), tmp)
		if err != nil {
			return 0, err
		}
		total += n
	}
	bundles, err := iconBundles(ctx, filepath.Join(data, "StreamingAssets", "aa", "StandaloneWindows64"))
	if err != nil {
		return 0, err
	}
	for _, b := range bundles {
		n, err := iconsFromBundle(b, tmp)
		if err != nil {
			return 0, err
		}
		total += n
	}
	if err := os.WriteFile(filepath.Join(tmp, iconsReadyMarker), nil, 0o600); err != nil {
		return 0, fmt.Errorf("finish icons: %w", err)
	}
	if err := os.RemoveAll(outDir); err != nil {
		return 0, fmt.Errorf("replace icons: %w", err)
	}
	if err := os.Rename(tmp, outDir); err != nil {
		return 0, fmt.Errorf("replace icons: %w", err)
	}
	return total, nil
}

func iconsFromAssets(path, out string) (int, error) {
	af, f, err := unityasset.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	dir := filepath.Dir(path)
	src := &textureSource{af: af, cache: map[int64]*image.NRGBA{}, stream: func(p string, off, size int64) ([]byte, error) {
		return readRange(filepath.Join(dir, filepath.Base(p)), off, size)
	}}
	return writeSprites(src, out)
}

func iconsFromBundle(path, out string) (int, error) {
	b, err := unityasset.OpenBundle(path)
	if err != nil {
		return 0, err
	}
	af, _, err := b.Assets()
	if err != nil {
		return 0, err
	}
	src := &textureSource{af: af, cache: map[int64]*image.NRGBA{}, stream: func(p string, off, size int64) ([]byte, error) {
		data, ok := b.Stream(p)
		if !ok || off < 0 || size < 0 || off > int64(len(data)) || size > int64(len(data))-off {
			return nil, fmt.Errorf("%w: stream %s", ErrBadTexture, p)
		}
		return data[off : off+size], nil
	}}
	return writeSprites(src, out)
}

func readRange(path string, off, size int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open texture data: %w", err)
	}
	defer f.Close()
	buf := make([]byte, size)
	if _, err := f.ReadAt(buf, off); err != nil {
		return nil, fmt.Errorf("read texture data: %w", err)
	}
	return buf, nil
}

func iconBundles(ctx context.Context, dir string) ([]string, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.bundle"))
	if err != nil {
		return nil, fmt.Errorf("list bundles: %w", err)
	}
	var (
		mu   sync.Mutex
		hits []string
		wg   sync.WaitGroup
	)
	jobs := make(chan string)
	for range runtime.GOMAXPROCS(0) {
		wg.Go(func() {
			for f := range jobs {
				if ok, _ := isIconBundle(f); ok {
					mu.Lock()
					hits = append(hits, f)
					mu.Unlock()
				}
			}
		})
	}
	for _, f := range files {
		if ctx.Err() != nil {
			break
		}
		jobs <- f
	}
	close(jobs)
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	slices.Sort(hits)
	return hits, nil
}

func isIconBundle(path string) (bool, error) {
	head, err := unityasset.PeekBundle(path, bundlePeekLimit)
	if err != nil {
		return false, err
	}
	if ids, err := unityasset.ClassIDs(head); err == nil && !slices.Contains(ids, classSpriteAtlas) {
		return false, nil
	}
	b, err := unityasset.OpenBundle(path)
	if err != nil {
		return false, err
	}
	af, _, err := b.Assets()
	if err != nil {
		return false, err
	}
	for _, name := range atlasNames(af) {
		if strings.HasPrefix(name, iconAtlasPrefix) {
			return true, nil
		}
	}
	return false, nil
}

func atlasNames(af *unityasset.File) []string {
	node, err := typetree.Load("SpriteAtlas")
	if err != nil {
		return nil
	}
	var out []string
	for _, o := range af.Objects {
		if o.ClassID != classSpriteAtlas {
			continue
		}
		raw, err := af.Read(o)
		if err != nil {
			continue
		}
		if v, err := typetree.ReadPrefix(node, raw, "m_Name"); err == nil {
			out = append(out, typetree.Str(v, "m_Name"))
		}
	}
	return out
}

func writeSprites(src *textureSource, out string) (int, error) {
	spriteNode, err := typetree.Load("Sprite")
	if err != nil {
		return 0, err
	}
	atlases, err := atlasRenderData(src.af)
	if err != nil {
		return 0, err
	}
	written := 0
	for _, o := range src.af.Objects {
		if o.ClassID != classSprite {
			continue
		}
		raw, err := src.af.Read(o)
		if err != nil {
			return written, err
		}
		s, err := typetree.Read(spriteNode, raw)
		if err != nil {
			continue
		}
		name := typetree.Str(s, "m_Name")
		if !ValidIconName(name) {
			continue
		}
		rd, ok := atlases[renderKey(typetree.Map(s, "m_RenderDataKey"))]
		if !ok {
			rd = spriteRenderData(typetree.Map(s, "m_RD"))
		}
		r := typetree.Map(s, "m_Rect")
		rd.canvasW, rd.canvasH = int(math.Round(typetree.Float(r, "width"))), int(math.Round(typetree.Float(r, "height")))
		rd.hasCanvas = true
		img, err := src.sprite(rd)
		if err != nil {
			continue
		}
		if err := writePNG(filepath.Join(out, name+iconFileExtension), img); err != nil {
			return written, err
		}
		written++
	}
	return written, nil
}

func atlasRenderData(af *unityasset.File) (map[string]renderData, error) {
	node, err := typetree.Load("SpriteAtlas")
	if err != nil {
		return nil, err
	}
	out := map[string]renderData{}
	for _, o := range af.Objects {
		if o.ClassID != classSpriteAtlas {
			continue
		}
		raw, err := af.Read(o)
		if err != nil {
			return nil, err
		}
		a, err := typetree.Read(node, raw)
		if err != nil {
			continue
		}
		for _, pair := range typetree.List(a, "m_RenderDataMap") {
			out[renderKey(typetree.Map(pair, "first"))] = spriteRenderData(typetree.Map(pair, "second"))
		}
	}
	return out, nil
}

func renderKey(k map[string]any) string {
	guid := typetree.Map(k, "first")
	return fmt.Sprintf("%v|%v|%v|%v|%d", guid["data[0]"], guid["data[1]"], guid["data[2]"], guid["data[3]"], typetree.Int(k, "second"))
}

func spriteRenderData(rd map[string]any) renderData {
	tex := typetree.Map(rd, "texture")
	rect := typetree.Map(rd, "textureRect")
	off := typetree.Map(rd, "textureRectOffset")
	return renderData{
		texture:  typetree.Int(tex, "m_PathID"),
		rect:     [4]float64{typetree.Float(rect, "x"), typetree.Float(rect, "y"), typetree.Float(rect, "width"), typetree.Float(rect, "height")},
		offset:   [2]float64{typetree.Float(off, "x"), typetree.Float(off, "y")},
		settings: typetree.Int(rd, "settingsRaw"),
	}
}

func (s *textureSource) sprite(rd renderData) (*image.NRGBA, error) {
	if rd.texture == 0 || rd.settings&settingsRotation != 0 {
		return nil, ErrBadTexture
	}
	tex, err := s.texture(rd.texture)
	if err != nil {
		return nil, err
	}
	x, y := int(math.Round(rd.rect[0])), int(math.Round(rd.rect[1]))
	w, h := int(math.Round(rd.rect[2])), int(math.Round(rd.rect[3]))
	b := tex.Bounds()
	if w <= 0 || h <= 0 || x < 0 || y < 0 || x+w > b.Dx() || y+h > b.Dy() {
		return nil, ErrBadTexture
	}
	cw, ch := w, h
	dx, dy := 0, 0
	if rd.hasCanvas && rd.canvasW >= w && rd.canvasH >= h {
		cw, ch = rd.canvasW, rd.canvasH
		dx, dy = int(math.Round(rd.offset[0])), int(math.Round(rd.offset[1]))
		dx, dy = min(max(dx, 0), cw-w), min(max(dy, 0), ch-h)
	}
	img := image.NewNRGBA(image.Rect(0, 0, cw, ch))
	for row := range h {
		srcY := y + row
		dstY := ch - 1 - (dy + row)
		copy(img.Pix[img.PixOffset(dx, dstY):img.PixOffset(dx+w, dstY)], tex.Pix[tex.PixOffset(x, srcY):tex.PixOffset(x+w, srcY)])
	}
	return img, nil
}

func (s *textureSource) texture(pathID int64) (*image.NRGBA, error) {
	if img, ok := s.cache[pathID]; ok {
		return img, nil
	}
	node, err := typetree.Load("Texture2D")
	if err != nil {
		return nil, err
	}
	for _, o := range s.af.Objects {
		if o.PathID != pathID || o.ClassID != classTexture2D {
			continue
		}
		raw, err := s.af.Read(o)
		if err != nil {
			return nil, err
		}
		t, err := typetree.Read(node, raw)
		if err != nil {
			return nil, err
		}
		pix := typetree.Bytes(t, "image data")
		if len(pix) == 0 {
			sd := typetree.Map(t, "m_StreamData")
			if pix, err = s.stream(typetree.Str(sd, "path"), typetree.Int(sd, "offset"), typetree.Int(sd, "size")); err != nil {
				return nil, err
			}
		}
		img, err := decodeTexture(pix, int(typetree.Int(t, "m_Width")), int(typetree.Int(t, "m_Height")), int(typetree.Int(t, "m_TextureFormat")))
		if err != nil {
			return nil, err
		}
		s.cache[pathID] = img
		return img, nil
	}
	return nil, fmt.Errorf("%w: texture %d", ErrBadTexture, pathID)
}

func decodeTexture(pix []byte, w, h, format int) (*image.NRGBA, error) {
	if w <= 0 || h <= 0 || w > maxTextureSide || h > maxTextureSide {
		return nil, fmt.Errorf("%w: size %dx%d", ErrBadTexture, w, h)
	}
	bpp := map[int]int{formatAlpha8: 1, formatRGB24: 3, formatRGBA32: 4, formatARGB32: 4}[format]
	if bpp == 0 {
		return nil, fmt.Errorf("%w: format %d", ErrBadTexture, format)
	}
	if len(pix) < w*h*bpp {
		return nil, fmt.Errorf("%w: %d bytes for %dx%d", ErrBadTexture, len(pix), w, h)
	}
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := range w * h {
		p := pix[i*bpp:]
		var c color.NRGBA
		switch format {
		case formatAlpha8:
			c = color.NRGBA{255, 255, 255, p[0]}
		case formatRGB24:
			c = color.NRGBA{p[0], p[1], p[2], 255}
		case formatRGBA32:
			c = color.NRGBA{p[0], p[1], p[2], p[3]}
		default:
			c = color.NRGBA{p[1], p[2], p[3], p[0]}
		}
		copy(img.Pix[i*4:], []byte{c.R, c.G, c.B, c.A})
	}
	return img, nil
}

func writePNG(path string, img image.Image) error {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return fmt.Errorf("encode icon: %w", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		return fmt.Errorf("write icon: %w", err)
	}
	return nil
}
