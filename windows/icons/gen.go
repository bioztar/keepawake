//go:build ignore

// Writes on.ico (green) and off.ico (grey): a disc with a white coffee cup, 16-48 px, 32-bit DIBs.
// Run from windows/: go run icons/gen.go
package main

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
)

type rgb struct{ r, g, b float64 }

func inCup(x, y float64) bool {
	// body: rounded-bottom rectangle
	if x >= 0.27 && x <= 0.63 && y >= 0.36 && y <= 0.70 {
		if y > 0.62 {
			dx := math.Max(0, math.Max(0.35-x, x-0.55))
			dy := y - 0.62
			return dx*dx+dy*dy <= 0.08*0.08
		}
		return true
	}
	// handle: ring on the right
	dx, dy := x-0.64, y-0.50
	r := math.Hypot(dx, dy)
	if x > 0.62 && r >= 0.055 && r <= 0.11 {
		return true
	}
	// saucer
	return x >= 0.22 && x <= 0.68 && y >= 0.74 && y <= 0.79
}

func pixel(c rgb, size int, px, py int) (r, g, b, a float64) {
	const ss = 4
	var cov, white float64
	for i := 0; i < ss; i++ {
		for j := 0; j < ss; j++ {
			x := (float64(px) + (float64(i)+0.5)/ss) / float64(size)
			y := (float64(py) + (float64(j)+0.5)/ss) / float64(size)
			if math.Hypot(x-0.5, y-0.5) <= 0.48 {
				cov++
				if inCup(x, y) {
					white++
				}
			}
		}
	}
	if cov == 0 {
		return 0, 0, 0, 0
	}
	w := white / cov
	return c.r*(1-w) + 255*w, c.g*(1-w) + 255*w, c.b*(1-w) + 255*w, 255 * cov / (ss * ss)
}

func dib(c rgb, size int) []byte {
	var buf bytes.Buffer
	maskRow := ((size + 31) / 32) * 4
	binary.Write(&buf, binary.LittleEndian, struct {
		Size                        uint32
		W, H                        int32
		Planes, Bits                uint16
		Compression, ImageSize      uint32
		XPPM, YPPM, Used, Important uint32
	}{40, int32(size), int32(2 * size), 1, 32, 0, uint32(size*size*4 + maskRow*size), 0, 0, 0, 0})
	for py := size - 1; py >= 0; py-- { // bottom-up
		for px := 0; px < size; px++ {
			r, g, b, a := pixel(c, size, px, py)
			// premultiplied alpha is not used by ICO; store straight BGRA
			buf.Write([]byte{byte(b + 0.5), byte(g + 0.5), byte(r + 0.5), byte(a + 0.5)})
		}
	}
	buf.Write(make([]byte, maskRow*size)) // AND mask unused with 32-bit alpha
	return buf.Bytes()
}

func ico(c rgb) []byte {
	sizes := []int{16, 20, 24, 32, 40, 48}
	imgs := make([][]byte, len(sizes))
	for i, s := range sizes {
		imgs[i] = dib(c, s)
	}
	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, [3]uint16{0, 1, uint16(len(sizes))})
	off := 6 + 16*len(sizes)
	for i, s := range sizes {
		binary.Write(&buf, binary.LittleEndian, struct {
			W, H, Colors, Res uint8
			Planes, Bits      uint16
			Size, Offset      uint32
		}{uint8(s), uint8(s), 0, 0, 1, 32, uint32(len(imgs[i])), uint32(off)})
		off += len(imgs[i])
	}
	for _, b := range imgs {
		buf.Write(b)
	}
	return buf.Bytes()
}

func main() {
	os.WriteFile("icons/on.ico", ico(rgb{0x2D, 0xB3, 0x4A}), 0o644)
	os.WriteFile("icons/off.ico", ico(rgb{0x8E, 0x8E, 0x93}), 0o644)
}
