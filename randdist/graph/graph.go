// Copyright 2015 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package graph

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io/ioutil"
	"math"
	"math/big"
	"net/http"
	"strconv"

	"github.com/golang/freetype"
	"github.com/golang/freetype/raster"
	"golang.org/x/image/math/fixed"
)

func Handler(w http.ResponseWriter, req *http.Request) {
	const size = 500
	c := image.NewRGBA(image.Rect(0, 0, size, size))
	white := &image.Uniform{C: color.White}
	draw.Draw(c, c.Bounds(), white, image.ZP, draw.Src)

	p := raster.NewRGBAPainter(c)
	p.SetColor(color.Black)
	r := raster.NewRasterizer(500, 500)
	r.UseNonZeroWinding = true
	var path raster.Path
	path.Start(nudge(fixed.P(50, 50)))
	path.Add1(nudge(fixed.P(50, 450)))
	path.Add1(nudge(fixed.P(450, 450)))
	r.AddStroke(path, fixed.I(2), raster.ButtCapper, raster.BevelJoiner)
	r.Rasterize(p)
	r.Clear()

	p.SetColor(color.Gray16{0x7FFF})
	path.Clear()
	r.Clear()
	path.Start(nudge(fixed.P(450, 450)))
	path.Add1(nudge(fixed.P(450, 50)))
	path.Add1(nudge(fixed.P(50, 50)))
	r.AddStroke(path, fixed.I(1), raster.ButtCapper, raster.BevelJoiner)
	r.Rasterize(p)

	p.SetColor(color.Gray16{0x7FFF})
	for x := 50; x <= 450; x += 50 {
		path.Clear()
		path.Start(nudge(fixed.P(x, 450)))
		path.Add1(nudge(fixed.P(x, 460)))
		r.AddStroke(path, fixed.I(1), raster.ButtCapper, raster.BevelJoiner)
	}
	for x := 50; x <= 450; x += 50 {
		path.Clear()
		path.Start(nudge(fixed.P(50, x)))
		path.Add1(nudge(fixed.P(40, x)))
		r.AddStroke(path, fixed.I(1), raster.ButtCapper, raster.BevelJoiner)
	}
	r.Rasterize(p)

	p.SetColor(color.RGBA{0x00, 0x00, 0xFF, 0xFF})
	r.Clear()
	path.Clear()
	path.Start(nudge(fixed.P(50, 450)))
	path.Add1(nudge(fixed.P(450, 50)))
	r.AddStroke(path, fixed.I(1), raster.ButtCapper, raster.BevelJoiner)
	r.Rasterize(p)

	p.SetColor(color.RGBA{0xCC, 0x00, 0x00, 0xFF})
	r.Clear()
	window := fixed.Rectangle26_6{fixed.P(50, 450), fixed.P(450, 50)}
	//	path = plotPath(ratSin, big.NewRat(0, 1), big.NewRat(-1, 1), big.NewRat(10, 1), big.NewRat(1, 1), window)

	//	path = plotPath(ratProb, big.NewRat(0, 1), big.NewRat(0, 1), big.NewRat(1, 1<<57), big.NewRat(1, 1<<57), window)

	var lo, hi *big.Rat
	var locap, hicap, scalecap string
	id, _ := strconv.Atoi(req.FormValue("x"))
	switch id {
	case 0:
		lo = big.NewRat(1<<53-1<<3, 1<<54)
		hi = big.NewRat(1<<53+1<<3, 1<<54)
		locap = "1/2 - 1/2^51"
		hicap = "1/2 + 1/2^51"
		scalecap = "1/2^53"
	case 1:
		lo = big.NewRat(1<<54-1<<4, 1<<54)
		hi = big.NewRat(1<<54, 1<<54)
		locap = "1 - 1/2^50"
		hicap = "1"
		scalecap = "1/2^53"
	case 2:
		lo = big.NewRat(0, 1<<54)
		hi = big.NewRat(1<<4, 1<<54)
		locap = "0"
		hicap = "1/2^50"
		scalecap = "1/2^53"
	case 3:
		lo = big.NewRat(0, 1<<54)
		hi = big.NewRat(1<<2, 1<<62)
		locap = "0"
		hicap = "1/2^59"
		scalecap = "1/2^63"
	}

	mode, _ := strconv.Atoi(req.FormValue("mode"))
	path = plotPath(ratProb(mode), lo, lo, hi, hi, window)
	r.AddStroke(path, fixed.I(1), raster.ButtCapper, raster.BevelJoiner)
	r.Rasterize(p)

	var modestr string
	switch mode {
	case 0:
		modestr = "original behavior (can return 1)"
	case 1:
		modestr = "new behavior (too much 0)"
	case 2:
		modestr = "retry loop"
	}

	data, err := ioutil.ReadFile("/tmp/luxisr.ttf")
	if err != nil {
		panic(err)
	}
	tfont, err := freetype.ParseFont(data)
	if err != nil {
		panic(err)
	}
	const pt = 10
	ft := freetype.NewContext()
	ft.SetDst(c)
	ft.SetDPI(100)
	ft.SetFont(tfont)
	ft.SetFontSize(float64(pt))
	ft.SetSrc(image.NewUniform(color.Black))

	ft.SetClip(image.Rect(0, 0, 0, 0))
	wid, err := ft.DrawString(locap, freetype.Pt(0, 0))
	if err != nil {
		panic(err)
	}
	pp := freetype.Pt(50, 480)
	pp.X -= wid.X / 2
	ft.SetClip(c.Bounds())
	ft.DrawString(locap, pp)

	ft.SetClip(image.Rect(0, 0, 0, 0))
	wid, err = ft.DrawString(hicap, freetype.Pt(0, 0))
	if err != nil {
		panic(err)
	}
	pp = freetype.Pt(450, 480)
	pp.X -= wid.X / 2
	ft.SetClip(c.Bounds())
	ft.DrawString(hicap, pp)

	r.Clear()
	p.SetColor(color.Black)
	path.Clear()
	const dy = 5
	path.Start(nudge(fixed.P(400, 400)))
	path.Add1(nudge(fixed.P(400, 400-dy)))
	path.Add1(nudge(fixed.P(400, 400+dy)))
	path.Add1(nudge(fixed.P(400, 400)))
	path.Add1(nudge(fixed.P(450, 400)))
	path.Add1(nudge(fixed.P(450, 400-dy)))
	path.Add1(nudge(fixed.P(450, 400+dy)))
	path.Add1(nudge(fixed.P(450, 400)))
	r.AddStroke(path, fixed.I(2), raster.ButtCapper, raster.BevelJoiner)
	r.Rasterize(p)

	ft.SetClip(image.Rect(0, 0, 0, 0))
	wid, err = ft.DrawString(scalecap, freetype.Pt(0, 0))
	if err != nil {
		panic(err)
	}
	pp = freetype.Pt(425, 420)
	pp.X -= wid.X / 2
	ft.SetClip(c.Bounds())
	ft.DrawString(scalecap, pp)

	ft.SetClip(image.Rect(0, 0, 0, 0))
	wid, err = ft.DrawString(modestr, freetype.Pt(0, 0))
	if err != nil {
		panic(err)
	}
	pp = freetype.Pt(250, 490)
	pp.X -= wid.X / 2
	ft.SetClip(c.Bounds())
	ft.DrawString(modestr, pp)

	w.Write(pngEncode(c))
}

func nudge(x fixed.Point26_6) fixed.Point26_6 {
	return fixed.Point26_6{x.X + 1<<5, x.Y + 1<<5}
}

func pngEncode(c image.Image) []byte {
	var b bytes.Buffer
	png.Encode(&b, c)
	return b.Bytes()
}

func plotPath(f func(*big.Rat) *big.Rat, fminx, fminy, fmaxx, fmaxy *big.Rat, r fixed.Rectangle26_6) raster.Path {
	px := r.Min.X
	var path raster.Path
	for ; px <= r.Max.X; px += 1 << 6 {
		fx := new(big.Rat).Add(fminx, new(big.Rat).Mul(new(big.Rat).Sub(fmaxx, fminx), new(big.Rat).Quo(big.NewRat(int64(px-r.Min.X), 1<<6), big.NewRat(int64(r.Max.X-r.Min.X), 1<<6))))
		fy := f(fx)
		fdy := new(big.Rat).Mul(new(big.Rat).Sub(fy, fminy), new(big.Rat).Quo(big.NewRat(int64(r.Max.Y-r.Min.Y), 1), new(big.Rat).Sub(fmaxy, fminy)))
		dy, _ := fdy.Float64()
		py := r.Min.Y + fixed.Int26_6(dy+0.5)
		if len(path) == 0 {
			path.Start(fixed.Point26_6{px, py})
		} else {
			path.Add1(fixed.Point26_6{px, py})
		}
	}
	return path
}

func ratSin(x *big.Rat) *big.Rat {
	fx, _ := x.Float64()
	fy := math.Sin(fx)
	return new(big.Rat).SetFloat64(fy)
}

const cutoff1 = 1<<63 - 1<<9
const cutoff2 = 1<<63 - 1<<8

func init() {
	var f1, f2 float64
	f1 = cutoff1 - 1
	f1 /= 1 << 63
	f2 = cutoff1
	f2 /= 1 << 63
	if f1 == 1 || f2 != 1 {
		panic(fmt.Sprintf("BAD: %#x => %g %g", cutoff1, f1, f2))
	}
}

var bigOne = big.NewInt(1)
var big2p63 = new(big.Int).Lsh(bigOne, 63)

func ratProb(mode int) func(*big.Rat) *big.Rat {
	return func(x *big.Rat) *big.Rat {
		lo := big.NewInt(0)
		hi := new(big.Int).Set(big2p63)
		n := 0
		for lo.Cmp(hi) != 0 {
			m := new(big.Int).Add(lo, hi)
			m = m.Rsh(m, 1)
			if n++; n > 100 {
				fmt.Printf("??? %v %v %v\n", lo, hi, m)
				break
			}
			v := new(big.Rat).SetFrac(m, big2p63)
			f, _ := v.Float64()
			v.SetFloat64(f)
			if v.Cmp(x) < 0 {
				lo.Add(m, bigOne)
			} else {
				hi.Set(m)
			}
		}
		switch mode {
		default: // case 0
			return new(big.Rat).SetFrac(lo, big2p63)
		case 1:
			if lo.Cmp(big.NewInt(cutoff1)) <= 0 {
				lo.Add(lo, big.NewInt(1<<63-cutoff1))
			}
			return new(big.Rat).SetFrac(lo, big2p63)
		case 2:
			return new(big.Rat).SetFrac(lo, big.NewInt(cutoff1))
		}
	}
}

func div2p63(x int64) *big.Rat {
	return new(big.Rat).SetFrac(big.NewInt(x), big2p63)
}
