// Package image implements `yozora print image`, drawing an image
// (PNG/JPEG/GIF from a file arg or stdin) using kitty graphics, sixel,
// or an ASCII fallback.
package image

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"flag"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"os"
	"strings"

	"github.com/ShashankRaoCoding/yozora/methods/print"
)

func init() {
	print.Methods["image"] = printImage
}

func printImage(args []string) {
	fs := flag.NewFlagSet("image", flag.ExitOnError)
	mode := fs.String("m", "auto", "mode: auto|kitty|sixel|ascii")
	width := fs.Int("w", 0, "max width (pixels for kitty/sixel, columns for ascii)")
	fs.Parse(args)

	var src io.Reader = os.Stdin
	if fs.NArg() > 0 {
		f, err := os.Open(fs.Arg(0))
		if err != nil {
			fatal(err)
		}
		defer f.Close()
		src = f
	}
	img, _, err := image.Decode(bufio.NewReader(src))
	if err != nil {
		fatal(err)
	}

	m := *mode
	if m == "auto" {
		switch {
		case kittyCompatible():
			m = "kitty"
		case sixelCompatible():
			m = "sixel"
		default:
			m = "ascii"
		}
	}
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()
	switch m {
	case "kitty":
		err = renderKitty(out, img, *width)
	case "sixel":
		err = renderSixel(out, img, *width)
	case "ascii":
		renderASCII(out, img, *width)
	default:
		err = fmt.Errorf("unknown mode %q", m)
	}
	if err != nil {
		out.Flush()
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "Error:", err)
	os.Exit(1)
}

func kittyCompatible() bool {
	tp := os.Getenv("TERM_PROGRAM")
	return os.Getenv("KITTY_WINDOW_ID") != "" || strings.Contains(os.Getenv("TERM"), "kitty") ||
		tp == "WezTerm" || tp == "ghostty"
}

func sixelCompatible() bool {
	t := os.Getenv("TERM")
	return strings.Contains(t, "sixel") || strings.HasPrefix(t, "foot") || strings.HasPrefix(t, "mlterm") ||
		os.Getenv("TERM_PROGRAM") == "iTerm.app"
}

// scale returns img resized (nearest neighbour) to at most maxW wide.
func scale(img image.Image, maxW int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if maxW <= 0 || w <= maxW {
		return img
	}
	nw, nh := maxW, h*maxW/w
	if nh < 1 {
		nh = 1
	}
	dst := image.NewNRGBA(image.Rect(0, 0, nw, nh))
	for y := 0; y < nh; y++ {
		for x := 0; x < nw; x++ {
			dst.Set(x, y, img.At(b.Min.X+x*w/nw, b.Min.Y+y*h/nh))
		}
	}
	return dst
}

func renderKitty(w io.Writer, img image.Image, maxW int) error {
	var buf bytes.Buffer
	if err := png.Encode(&buf, scale(img, maxW)); err != nil {
		return err
	}
	enc := base64.StdEncoding.EncodeToString(buf.Bytes())
	const chunk = 4096
	for first := true; len(enc) > 0; first = false {
		n := chunk
		if n > len(enc) {
			n = len(enc)
		}
		more := 0
		if n < len(enc) {
			more = 1
		}
		ctrl := fmt.Sprintf("m=%d", more)
		if first {
			ctrl = "a=T,f=100," + ctrl
		}
		if _, err := fmt.Fprintf(w, "\x1b_G%s;%s\x1b\\", ctrl, enc[:n]); err != nil {
			return err
		}
		enc = enc[n:]
	}
	_, err := fmt.Fprintln(w)
	return err
}

// renderSixel encodes using a fixed 6x6x6 colour cube palette.
func renderSixel(w io.Writer, img image.Image, maxW int) error {
	if maxW <= 0 {
		maxW = 800
	}
	img = scale(img, maxW)
	b := img.Bounds()
	W, H := b.Dx(), b.Dy()

	idx := make([]int, W*H)
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			c := color.NRGBAModel.Convert(img.At(b.Min.X+x, b.Min.Y+y)).(color.NRGBA)
			// Composite over white.
			bl := func(v uint8) int { return (int(v)*int(c.A) + 255*(255-int(c.A))) / 255 }
			q := func(v int) int { return (v*5 + 127) / 255 }
			idx[y*W+x] = q(bl(c.R))*36 + q(bl(c.G))*6 + q(bl(c.B))
		}
	}

	var sb strings.Builder
	sb.WriteString("\x1bPq")
	fmt.Fprintf(&sb, "\"1;1;%d;%d", W, H)
	for i := 0; i < 216; i++ {
		fmt.Fprintf(&sb, "#%d;2;%d;%d;%d", i, (i/36)*20, (i/6%6)*20, (i%6)*20)
	}
	for y0 := 0; y0 < H; y0 += 6 {
		used := map[int]bool{}
		for y := y0; y < y0+6 && y < H; y++ {
			for x := 0; x < W; x++ {
				used[idx[y*W+x]] = true
			}
		}
		first := true
		for c := 0; c < 216; c++ {
			if !used[c] {
				continue
			}
			if !first {
				sb.WriteByte('$')
			}
			first = false
			fmt.Fprintf(&sb, "#%d", c)
			prev, run := byte(0), 0
			flush := func() {
				if run == 0 {
					return
				}
				if run > 3 {
					fmt.Fprintf(&sb, "!%d%c", run, prev)
				} else {
					sb.WriteString(strings.Repeat(string(prev), run))
				}
			}
			for x := 0; x < W; x++ {
				var bits byte
				for k := 0; k < 6 && y0+k < H; k++ {
					if idx[(y0+k)*W+x] == c {
						bits |= 1 << k
					}
				}
				ch := bits + 63
				if run > 0 && ch == prev {
					run++
					continue
				}
				flush()
				prev, run = ch, 1
			}
			flush()
		}
		sb.WriteByte('-')
	}
	sb.WriteString("\x1b\\\n")
	_, err := io.WriteString(w, sb.String())
	return err
}

func renderASCII(w io.Writer, img image.Image, cols int) {
	if cols <= 0 {
		cols = 80
	}
	const ramp = " .:-=+*#%@"
	b := img.Bounds()
	W, H := b.Dx(), b.Dy()
	if cols > W {
		cols = W
	}
	// Terminal cells are ~2x taller than wide.
	rows := H * cols / W / 2
	if rows < 1 {
		rows = 1
	}
	for y := 0; y < rows; y++ {
		line := make([]byte, cols)
		for x := 0; x < cols; x++ {
			c := color.GrayModel.Convert(img.At(b.Min.X+x*W/cols, b.Min.Y+y*H/rows)).(color.Gray)
			line[x] = ramp[int(c.Y)*(len(ramp)-1)/255]
		}
		fmt.Fprintln(w, string(line))
	}
}
