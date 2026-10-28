// Package formats registers file format converters with `yozora convert`.
package formats

import (
	"bufio"
	"encoding/binary"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"os"
	"sort"

	"github.com/ShashankRaoCoding/yozora/methods/convert"
)

func init() {
	convert.Methods[[2]string{"mrc", "png"}] = mrc2png
}

// mrc2png converts one z-slice of an MRC volume (stdin) to an 8-bit
// greyscale PNG (stdout), with percentile contrast stretching.
func mrc2png(args []string) {
	fs := flag.NewFlagSet("mrc2png", flag.ExitOnError)
	z := fs.Int("z", 0, "z slice index")
	clip := fs.Float64("clip", 0.5, "percentile clipped at each end for contrast")
	fs.Parse(args)

	data, err := io.ReadAll(bufio.NewReader(os.Stdin))
	if err != nil {
		fatal(err)
	}
	img, err := decodeMRC(data, *z, *clip)
	if err != nil {
		fatal(err)
	}
	w := bufio.NewWriter(os.Stdout)
	if err := png.Encode(w, img); err != nil {
		fatal(err)
	}
	if err := w.Flush(); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "Error:", err)
	os.Exit(1)
}

func decodeMRC(data []byte, z int, clip float64) (*image.Gray, error) {
	const headerSize = 1024
	if len(data) < headerSize {
		return nil, fmt.Errorf("mrc: file too short for header")
	}
	var bo binary.ByteOrder = binary.LittleEndian
	// MACHST: 0x44 = little endian, 0x11 = big endian.
	if data[212] == 0x11 && data[213] == 0x11 {
		bo = binary.BigEndian
	}
	i32 := func(off int) int { return int(int32(bo.Uint32(data[off:]))) }
	nx, ny, nz, mode, nsymbt := i32(0), i32(4), i32(8), i32(12), i32(92)
	if nx <= 0 || ny <= 0 || nz <= 0 || nsymbt < 0 {
		return nil, fmt.Errorf("mrc: invalid dimensions %dx%dx%d", nx, ny, nz)
	}
	if z < 0 || z >= nz {
		return nil, fmt.Errorf("mrc: z slice %d out of range [0,%d)", z, nz)
	}

	var size int
	switch mode {
	case 0:
		size = 1
	case 1, 6, 12:
		size = 2
	case 2:
		size = 4
	default:
		return nil, fmt.Errorf("mrc: unsupported mode %d", mode)
	}
	n := nx * ny
	start := headerSize + nsymbt + z*n*size
	if start+n*size > len(data) {
		return nil, fmt.Errorf("mrc: file truncated")
	}
	raw := data[start : start+n*size]

	vals := make([]float64, n)
	for i := range vals {
		switch mode {
		case 0:
			vals[i] = float64(int8(raw[i]))
		case 1:
			vals[i] = float64(int16(bo.Uint16(raw[i*2:])))
		case 6:
			vals[i] = float64(bo.Uint16(raw[i*2:]))
		case 12:
			vals[i] = float64(half(bo.Uint16(raw[i*2:])))
		case 2:
			vals[i] = float64(math.Float32frombits(bo.Uint32(raw[i*4:])))
		}
	}

	sorted := make([]float64, 0, n)
	for _, v := range vals {
		if !math.IsNaN(v) && !math.IsInf(v, 0) {
			sorted = append(sorted, v)
		}
	}
	if len(sorted) == 0 {
		return nil, fmt.Errorf("mrc: slice has no finite values")
	}
	sort.Float64s(sorted)
	pick := func(p float64) float64 {
		idx := int(p / 100 * float64(len(sorted)-1))
		if idx < 0 {
			idx = 0
		}
		if idx > len(sorted)-1 {
			idx = len(sorted) - 1
		}
		return sorted[idx]
	}
	lo, hi := pick(clip), pick(100-clip)
	if hi <= lo {
		lo, hi = sorted[0], sorted[len(sorted)-1]
	}

	img := image.NewGray(image.Rect(0, 0, nx, ny))
	for y := 0; y < ny; y++ {
		// MRC origin is bottom-left; flip so row 0 is the top.
		src := (ny - 1 - y) * nx
		for x := 0; x < nx; x++ {
			v := 0.0
			if hi > lo {
				v = (vals[src+x] - lo) / (hi - lo)
			}
			v = math.Max(0, math.Min(1, v))
			img.SetGray(x, y, color.Gray{Y: uint8(v*255 + 0.5)})
		}
	}
	return img, nil
}

// half converts an IEEE 754 binary16 value to float32.
func half(h uint16) float32 {
	sign := uint32(h>>15) & 1
	exp := uint32(h>>10) & 0x1f
	frac := uint32(h) & 0x3ff
	var bits uint32
	switch {
	case exp == 0 && frac == 0:
		bits = sign << 31
	case exp == 0: // subnormal
		e := uint32(127 - 15 + 1)
		for frac&0x400 == 0 {
			frac <<= 1
			e--
		}
		frac &= 0x3ff
		bits = sign<<31 | e<<23 | frac<<13
	case exp == 0x1f:
		bits = sign<<31 | 0xff<<23 | frac<<13
	default:
		bits = sign<<31 | (exp+127-15)<<23 | frac<<13
	}
	return math.Float32frombits(bits)
}
