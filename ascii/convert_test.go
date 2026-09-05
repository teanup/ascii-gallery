package ascii

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"testing"
)

func TestComputeDimensions(t *testing.T) {
	tests := []struct {
		name         string
		srcW, srcH   int
		userW, userH int
		wantW, wantH int
	}{
		{"both given", 100, 50, 40, 20, 40, 20},
		{"only width", 100, 50, 40, 0, 40, 10},
		{"only height", 100, 50, 0, 20, 80, 20},
		{"neither", 100, 50, 0, 0, 80, 20},
		{"clamp min width", 10, 10, 0, 0, 80, 40},
		{"clamp max width", 100, 100, 1000, 0, 320, 80},
		{"clamp min height", 100, 10, 0, 0, 80, 8},
		{"clamp max height", 10, 1000, 0, 0, 80, 80},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, h := ComputeDimensions(tt.srcW, tt.srcH, tt.userW, tt.userH)
			if w != tt.wantW || h != tt.wantH {
				t.Errorf("ComputeDimensions(%d,%d,%d,%d) = (%d,%d), want (%d,%d)",
					tt.srcW, tt.srcH, tt.userW, tt.userH, w, h, tt.wantW, tt.wantH)
			}
		})
	}
}

func TestSelectFrames(t *testing.T) {
	tests := []struct {
		name      string
		count     int
		maxFrames int
		want      []int
	}{
		{"all kept", 5, 10, []int{0, 1, 2, 3, 4}},
		{"exact", 5, 5, []int{0, 1, 2, 3, 4}},
		{"drop evenly", 10, 5, []int{0, 2, 4, 6, 9}},
		{"two frames", 10, 2, []int{0, 9}},
		{"single frame", 10, 1, []int{0}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := selectFrames(tt.count, tt.maxFrames)
			if len(got) != len(tt.want) {
				t.Fatalf("selectFrames(%d,%d) = %v, want %v", tt.count, tt.maxFrames, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("selectFrames(%d,%d) = %v, want %v", tt.count, tt.maxFrames, got, tt.want)
				}
			}
		})
	}
}

func TestRGBToANSI256(t *testing.T) {
	tests := []struct {
		name    string
		r, g, b uint8
		want    uint8
	}{
		{"black", 0, 0, 0, 232},
		{"white", 255, 255, 255, 255},
		{"gray", 128, 128, 128, 232 + 128*24/256},
		{"pure red", 255, 0, 0, 16 + 36*5},
		{"pure green", 0, 255, 0, 16 + 6*5},
		{"pure blue", 0, 0, 255, 16 + 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rgbToANSI256(tt.r, tt.g, tt.b)
			if got != tt.want {
				t.Errorf("rgbToANSI256(%d,%d,%d) = %d, want %d", tt.r, tt.g, tt.b, got, tt.want)
			}
		})
	}
}

func TestQuantize6(t *testing.T) {
	tests := []struct {
		v    uint8
		want int
	}{
		{0, 0}, {47, 0},
		{48, 1}, {114, 1},
		{115, 2}, {154, 2},
		{155, 3}, {194, 3},
		{195, 4}, {234, 4},
		{235, 5}, {255, 5},
	}
	for _, tt := range tests {
		if got := quantize6(tt.v); got != tt.want {
			t.Errorf("quantize6(%d) = %d, want %d", tt.v, got, tt.want)
		}
	}
}

func TestCompressDecompressRoundTrip(t *testing.T) {
	original := "hello world\nthis is a test\n"
	compressed, err := compressString(original)
	if err != nil {
		t.Fatalf("compressString: %v", err)
	}
	got, err := DecompressString(compressed)
	if err != nil {
		t.Fatalf("DecompressString: %v", err)
	}
	if got != original {
		t.Errorf("round trip mismatch: got %q, want %q", got, original)
	}
}

func TestDecompressStringInvalid(t *testing.T) {
	if _, err := DecompressString("not base64!!!"); err == nil {
		t.Error("expected error for invalid base64, got nil")
	}
}

// makeTestGIF builds a small animated GIF with the given frame colors.
func makeTestGIF(t *testing.T, w, h int, frameColors [][]color.RGBA) []byte {
	t.Helper()
	palette := color.Palette{color.RGBA{0, 0, 0, 255}, color.RGBA{255, 255, 255, 255}}
	g := &gif.GIF{
		Config: image.Config{Width: w, Height: h},
	}
	for _, fc := range frameColors {
		img := image.NewPaletted(image.Rect(0, 0, w, h), palette)
		for y := range h {
			for x := range w {
				if fc[0].R > 128 {
					img.SetColorIndex(x, y, 1)
				} else {
					img.SetColorIndex(x, y, 0)
				}
			}
		}
		g.Image = append(g.Image, img)
		g.Delay = append(g.Delay, 10)
	}
	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, g); err != nil {
		t.Fatalf("encode test gif: %v", err)
	}
	return buf.Bytes()
}

func TestConvertGIF(t *testing.T) {
	data := makeTestGIF(t, 20, 10, [][]color.RGBA{
		{{R: 255, G: 255, B: 255, A: 255}},
		{{R: 0, G: 0, B: 0, A: 255}},
	})

	anim, err := ConvertGIF(bytes.NewReader(data), 20, 10, 100, 50)
	if err != nil {
		t.Fatalf("ConvertGIF: %v", err)
	}

	if len(anim.Frames) != 2 {
		t.Errorf("expected 2 frames, got %d", len(anim.Frames))
	}
	if anim.Delay != 100 {
		t.Errorf("expected delay 100ms (10cs * 10), got %d", anim.Delay)
	}
	if anim.Width == 0 || anim.Height == 0 {
		t.Errorf("expected non-zero dimensions, got %dx%d", anim.Width, anim.Height)
	}

	// Frames must decompress to valid ANSI output.
	for i, f := range anim.Frames {
		decoded, err := DecompressString(f)
		if err != nil {
			t.Fatalf("frame %d decompress: %v", i, err)
		}
		if decoded == "" {
			t.Errorf("frame %d is empty", i)
		}
	}
}

func TestConvertGIFInvalid(t *testing.T) {
	if _, err := ConvertGIF(bytes.NewReader([]byte("not a gif")), 20, 10, 100, 50); err == nil {
		t.Error("expected error for invalid gif, got nil")
	}
}
