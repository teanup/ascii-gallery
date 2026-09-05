package ascii

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"image"
	"image/gif"
	"io"
	"math"
	"strings"

	"github.com/teanup/ascii-gallery/model"
)

// ASCII ramp from dark to light (more levels for better density).
// These are ordered by visual density (approximate coverage).
var asciiRamp = []string{
	" ", "`", ".", ",", "'", ":", ";", "-", "~", "=",
	"+", "*", "?", "!", "/", "(", ")", "[", "]", "{", "}",
	"1", "i", "!", "l", "I", "t", "r", "v", "x", "z",
	"c", "C", "o", "O", "0", "8", "X", "&", "%", "#", "@",
}

// 256-color ANSI palette (6x6x6 color cube + grayscale ramp).
// This maps an RGB color to the nearest 8-bit ANSI color code.
func rgbToANSI256(r, g, b uint8) uint8 {
	// If the color is close to gray (R=G=B), use the grayscale ramp.
	// Grayscale ramp: codes 232-255 (24 steps from black to white).
	if absDiff(r, g) < 10 && absDiff(g, b) < 10 && absDiff(r, b) < 10 {
		gray := int(r)
		// Map 0-255 to 232-255 (24 steps).
		code := min(232+gray*24/256, 255)
		return uint8(code)
	}

	// Map to the 6x6x6 color cube (codes 16-231).
	// Each channel is quantized to 6 levels (0, 95, 135, 175, 215, 255).
	ir := quantize6(r)
	ig := quantize6(g)
	ib := quantize6(b)

	// Code = 16 + 36*R + 6*G + B
	return uint8(16 + 36*ir + 6*ig + ib)
}

func quantize6(v uint8) int {
	switch {
	case v < 48:
		return 0
	case v < 115:
		return 1
	case v < 155:
		return 2
	case v < 195:
		return 3
	case v < 235:
		return 4
	default:
		return 5
	}
}

func absDiff(a, b uint8) int {
	if a > b {
		return int(a - b)
	}
	return int(b - a)
}

// ComputeDimensions calculates the output ASCII dimensions given the source
// GIF size and optional user constraints. Rules applied in order:
//  1. If both width and height are given, use them as-is (no aspect ratio adjustment).
//  2. If only width is given, compute height preserving aspect ratio, accounting
//     for the 2:1 terminal character aspect (height is halved).
//  3. If only height is given, compute width preserving aspect ratio.
//  4. If neither is given, default width=80 and compute height from aspect ratio.
//  5. Clamp: width in [16, 320], height in [8, 80].
func ComputeDimensions(srcW, srcH int, userWidth, userHeight int) (width, height int) {
	const (
		minWidth  = 16
		maxWidth  = 320
		minHeight = 8
		maxHeight = 80
	)

	if userWidth > 0 && userHeight > 0 {
		width = userWidth
		height = userHeight
	} else if userWidth > 0 {
		width = userWidth
		height = int(math.Round(float64(srcH) / float64(srcW) * float64(width) * 0.5))
	} else if userHeight > 0 {
		height = userHeight
		width = int(math.Round(float64(srcW) / float64(srcH) * float64(height) * 2.0))
	} else {
		width = 80
		height = int(math.Round(float64(srcH) / float64(srcW) * float64(width) * 0.5))
	}

	if width < minWidth {
		width = minWidth
	}
	if width > maxWidth {
		width = maxWidth
	}
	if height < minHeight {
		height = minHeight
	}
	if height > maxHeight {
		height = maxHeight
	}

	return width, height
}

// ConvertGIF decodes a GIF from the reader, resizes it to the target dimensions,
// converts each frame to colored ASCII art, and returns an Animation.
// It also handles frame rate limiting: drops frames evenly if there are too many,
// and ensures the delay is at least a minimum threshold.
func ConvertGIF(r io.Reader, targetWidth, targetHeight int, maxFrames int, minDelayMs int) (*model.Animation, error) {
	g, err := gif.DecodeAll(r)
	if err != nil {
		return nil, fmt.Errorf("decode gif: %w", err)
	}

	if len(g.Image) == 0 {
		return nil, fmt.Errorf("gif has no frames")
	}

	// Determine delay in milliseconds.
	// GIF stores delay in centiseconds (1/100 sec), convert to ms.
	delayMs := max(g.Delay[0]*10, minDelayMs)

	// Select frames: drop evenly if over maxFrames.
	indices := selectFrames(len(g.Image), maxFrames)

	srcW := g.Config.Width
	srcH := g.Config.Height

	// First pass: build pixel grids for all frames.
	frames := make([]framePixels, len(indices))
	for fi, srcIdx := range indices {
		frames[fi].pixels = buildPixels(g.Image[srcIdx], srcW, srcH, targetWidth, targetHeight)
	}

	// Compute crop bounds across all frames.
	cropTop, cropBottom, cropLeft, cropRight := computeCropBounds(frames, targetWidth, targetHeight)

	croppedW := targetWidth - cropLeft - cropRight
	croppedH := targetHeight - cropTop - cropBottom

	anim := &model.Animation{
		Width:  croppedW,
		Height: croppedH,
		Delay:  delayMs,
		Frames: make([]string, len(indices)),
	}

	// Second pass: render and compress each cropped frame.
	for fi := range frames {
		frameStr := renderFrame(frames[fi].pixels, targetWidth, targetHeight, cropTop, cropBottom, cropLeft, cropRight)

		compressed, err := compressString(frameStr)
		if err != nil {
			return nil, fmt.Errorf("compress frame %d: %w", fi, err)
		}
		anim.Frames[fi] = compressed
	}

	return anim, nil
}

// selectFrames returns the indices of frames to keep.
// If count <= maxFrames, all frames are kept.
// Otherwise, frames are dropped evenly to stay within maxFrames.
func selectFrames(count, maxFrames int) []int {
	if maxFrames <= 0 || count <= maxFrames {
		indices := make([]int, count)
		for i := range indices {
			indices[i] = i
		}
		return indices
	}

	indices := make([]int, maxFrames)
	for i := range maxFrames {
		// Distribute evenly: pick frames at roughly equal intervals.
		idx := 0
		if maxFrames > 1 {
			idx = i * (count - 1) / (maxFrames - 1)
		}
		if idx >= count {
			idx = count - 1
		}
		indices[i] = idx
	}
	return indices
}

// pixelInfo holds the computed data for a single pixel in a frame.
type pixelInfo struct {
	char rune  // ASCII character to display
	code uint8 // 256-color ANSI code (ignored if char is space)
}

// framePixels holds the pixel grid for one frame.
type framePixels struct {
	pixels []pixelInfo
}

// buildPixels converts a single GIF frame to a grid of pixelInfo values.
// It uses block averaging: each output character represents the average color
// of a rectangular block of source pixels, giving much smoother results than
// nearest-neighbor sampling.
func buildPixels(src *image.Paletted, srcW, srcH, targetW, targetH int) []pixelInfo {
	pixels := make([]pixelInfo, targetW*targetH)

	// Precompute source pixel ranges for each output column and row.
	colStarts := make([]int, targetW+1)
	for x := 0; x <= targetW; x++ {
		colStarts[x] = x * srcW / targetW
	}
	rowStarts := make([]int, targetH+1)
	for y := 0; y <= targetH; y++ {
		rowStarts[y] = y * srcH / targetH
	}

	for y := range targetH {
		rowStart := rowStarts[y]
		rowEnd := rowStarts[y+1]
		if rowEnd <= rowStart {
			rowEnd = rowStart + 1
		}

		for x := range targetW {
			colStart := colStarts[x]
			colEnd := colStarts[x+1]
			if colEnd <= colStart {
				colEnd = colStart + 1
			}

			// Average all source pixels in this block.
			var sumR, sumG, sumB uint64
			count := 0

			for sy := rowStart; sy < rowEnd; sy++ {
				for sx := colStart; sx < colEnd; sx++ {
					ci := src.ColorIndexAt(sx, sy)
					c := src.Palette[ci]
					r, g, b, _ := c.RGBA()
					sumR += uint64(r >> 8)
					sumG += uint64(g >> 8)
					sumB += uint64(b >> 8)
					count++
				}
			}

			avgR := uint8(sumR / uint64(count))
			avgG := uint8(sumG / uint64(count))
			avgB := uint8(sumB / uint64(count))

			// Compute luminance from averaged color.
			lum := 0.299*float64(avgR) + 0.587*float64(avgG) + 0.114*float64(avgB)
			idx := int(lum * float64(len(asciiRamp)-1) / 255.0)
			if idx < 0 {
				idx = 0
			} else if idx >= len(asciiRamp) {
				idx = len(asciiRamp) - 1
			}

			pixels[y*targetW+x] = pixelInfo{
				char: []rune(asciiRamp[idx])[0],
				code: rgbToANSI256(avgR, avgG, avgB),
			}
		}
	}

	return pixels
}

// renderFrame renders a cropped frame from the pixel grid to an ANSI string.
// Consecutive same-color non-space characters are batched under one ANSI code.
func renderFrame(pixels []pixelInfo, fullW, fullH, cropTop, cropBottom, cropLeft, cropRight int) string {
	croppedW := fullW - cropLeft - cropRight
	croppedH := fullH - cropTop - cropBottom

	var buf strings.Builder
	buf.Grow(croppedW * croppedH * 6)

	for y := cropTop; y < fullH-cropBottom; y++ {
		rowStart := y*fullW + cropLeft
		rowEnd := y*fullW + fullW - cropRight
		row := pixels[rowStart:rowEnd]

		lastCode := uint8(0)
		lastCodeSet := false

		for _, pi := range row {
			if pi.char == ' ' {
				buf.WriteByte(' ')
				lastCodeSet = false
				continue
			}

			if !lastCodeSet || pi.code != lastCode {
				fmt.Fprintf(&buf, "\033[38;5;%dm", pi.code)
				lastCode = pi.code
				lastCodeSet = true
			}
			buf.WriteRune(pi.char)
		}
		buf.WriteString("\033[0m\n")
	}

	return buf.String()
}

// computeCropBounds finds the bounding box of non-space content across all frames.
// Returns top, bottom, left, right crop amounts.
func computeCropBounds(frames []framePixels, fullW, fullH int) (top, bottom, left, right int) {
	// Find top boundary.
	top = fullH
	for fi := range frames {
		for y := 0; y < top; y++ {
			row := frames[fi].pixels[y*fullW : (y+1)*fullW]
			if !isRowEmpty(row) {
				if y < top {
					top = y
				}
				break
			}
		}
	}

	// Find bottom boundary.
	bottom = fullH
	for fi := range frames {
		for y := fullH - 1; y >= fullH-bottom; y-- {
			row := frames[fi].pixels[y*fullW : (y+1)*fullW]
			if !isRowEmpty(row) {
				dist := fullH - 1 - y
				if dist < bottom {
					bottom = dist
				}
				break
			}
		}
	}

	// Find left boundary.
	left = fullW
	for fi := range frames {
		for x := 0; x < left; x++ {
			if !isColEmpty(frames[fi].pixels, fullW, fullH, x) {
				if x < left {
					left = x
				}
				break
			}
		}
	}

	// Find right boundary.
	right = fullW
	for fi := range frames {
		for x := fullW - 1; x >= fullW-right; x-- {
			if !isColEmpty(frames[fi].pixels, fullW, fullH, x) {
				dist := fullW - 1 - x
				if dist < right {
					right = dist
				}
				break
			}
		}
	}

	return top, bottom, left, right
}

func isRowEmpty(row []pixelInfo) bool {
	for _, pi := range row {
		if pi.char != ' ' {
			return false
		}
	}
	return true
}

func isColEmpty(pixels []pixelInfo, fullW, fullH, col int) bool {
	for y := range fullH {
		if pixels[y*fullW+col].char != ' ' {
			return false
		}
	}
	return true
}

// compressString gzip-compresses and base64-encodes a string.
func compressString(s string) (string, error) {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write([]byte(s)); err != nil {
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// DecompressString base64-decodes and gzip-decompresses a string.
func DecompressString(encoded string) (string, error) {
	compressed, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("base64 decode: %w", err)
	}

	r, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return "", fmt.Errorf("gzip reader: %w", err)
	}
	defer r.Close()

	data, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("gzip decompress: %w", err)
	}
	return string(data), nil
}
