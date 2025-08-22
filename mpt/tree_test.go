// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mpt

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math/rand"
	"runtime/debug"
	"strings"
	"testing"
)

var goldenTrees = []struct {
	keys []Key
	hash Hash
}{
	{
		[]Key{},
		// sha256 /dev/null
		sha(),
	},
	{
		[]Key{h("0...0")},
		sha(h("00...0"), h("420...0")),
	},
	{
		[]Key{h("80...0")},
		sha(h("80...0"), h("420...0")),
	},
	{
		[]Key{h("0...0"), h("80...0")},
		sha(
			sha(h("00...0"), h("420...0")),
			sha(h("80...0"), h("420...01")),
			"\x00",
		),
	},
	{
		[]Key{h("0...0"), h("0010...0")},
		sha(
			sha(h("0...0"), h("420...0")),
			sha(h("0010...0"), h("420...01")),
			"\x0b",
		),
	},
	{
		[]Key{h("0...0"), h("0010...0"), h("80...0")},
		sha(
			sha(
				sha(h("0...0"), h("420...0")),
				sha(h("0010...0"), h("420...01")),
				"\x0b",
			),
			sha(h("80...0"), h("420...02")),
			"\x00",
		),
	},
}

var missing = []Key{
	h("02...2"),
	h("22...2"),
	h("42...2"),
	h("62...2"),
	h("82...2"),
	h("a2...2"),
	h("c2...2"),
	h("e2...2"),
	h("f2...2"),
}

func testImpls(t *testing.T, run func(*testing.T, func(*testing.T) *testTree)) {
	t.Run("impl=mem", func(t *testing.T) { run(t, testMemTree) })
	t.Run("impl=disk", func(t *testing.T) { run(t, testDiskTree) })
}

func TestGoldenTrees(t *testing.T) {
	testImpls(t, func(t *testing.T, newTree func(*testing.T) *testTree) {
		for i, tree := range goldenTrees {
			t.Run(fmt.Sprint(i), func(t *testing.T) {
				tt := newTree(t)
				defer tt.tree.Close()
				e := int64(1)
				if len(tree.keys) == 0 {
					e = 0
				}
				for i, k := range tree.keys {
					tt.set(k, v(i))
				}
				tt.snap(e, tree.hash)
				for i, k := range tree.keys {
					tt.get(k, v(i), true)
				}
				for _, k := range missing {
					tt.get(k, Value{}, false)
				}
			})
		}
	})
}

func TestAllTrees(t *testing.T) {
	testImpls(t, func(t *testing.T, newTree func(*testing.T) *testTree) {
		for _, keys := range []string{"hi", "lo"} {
			t.Run(keys, func(t *testing.T) {
				const B = 3
				const N = 1 << B
				k := func(i int) Key {
					var k Key
					if keys == "hi" {
						k[0] = byte(i) << (8 - B)
					} else {
						k[len(k)-1] = byte(i)
					}
					return k
				}
				for leaves := range 1 << N {
					tt := newTree(t)
					var keys []Key
					var vals []Value
					for i := range N {
						if leaves&(1<<i) != 0 {
							keys = append(keys, k(i))
							vals = append(vals, v(i))
						}
					}
					for _, i := range rand.Perm(len(keys)) {
						tt.set(keys[i], vals[i])
					}

					e := 1
					if len(keys) == 0 {
						e = 0
					}
					for round := range 3 {
						tt.snap(int64(e*(1+round)), rootHash(keys, vals))
						for i := range N {
							if leaves&(1<<i) != 0 {
								tt.get(k(i), v(i+N*round), true)
							} else {
								tt.get(k(i), Value{}, false)
							}
						}
						keys = keys[:0]
						vals = vals[:0]
						for i := range N {
							if leaves&(1<<i) != 0 {
								val := v(i + N*(round+1))
								tt.set(k(i), val)
								keys = append(keys, k(i))
								vals = append(vals, val)
							}
						}
					}
					tt.tree.Close()
				}
			})
		}
	})
}

func h(x string) [32]byte {
	if l, r, ok := strings.Cut(x, "..."); ok && l != "" && r != "" && l[len(l)-1] == r[0] {
		x = l + strings.Repeat(r[0:1], 64-len(l)-len(r)) + r
	}
	h, err := hex.DecodeString(x)
	if err != nil || len(h) != 32 {
		panic("bad hex: " + x)
	}
	return [32]byte(h)
}

func v(i int) Value {
	return h(fmt.Sprintf("42%062x", i))
}

func enc(list ...any) []byte {
	var out []byte
	for _, item := range list {
		switch item := item.(type) {
		default:
			panic(fmt.Sprintf("enc %T", item))
		case string:
			out = append(out, item...)
		case [32]byte:
			out = append(out, item[:]...)
		case Key:
			out = append(out, item[:]...)
		case Value:
			out = append(out, item[:]...)
		case Hash:
			out = append(out, item[:]...)
		}
	}
	return out
}

func sha(list ...any) [32]byte {
	return sha256.Sum256(enc(list...))
}

type testTree struct {
	t    *testing.T
	tree Tree
	log  bytes.Buffer
}

func testMemTree(t *testing.T) *testTree {
	return &testTree{t: t, tree: NewMemTree()}
}

// DevNull returns a file like the Unix /dev/null: it can be written but is always empty.
// Passing two DevNull files to New creates a Mem with no on-disk backing.
func DevNull() File {
	return new(devNull)
}

type devNull struct{}

func (*devNull) ReadAt(b []byte, off int64) (int, error)  { return 0, io.EOF }
func (*devNull) WriteAt(b []byte, off int64) (int, error) { return len(b), nil }

func (*devNull) Close() error { return nil }
func (*devNull) Sync() error  { return nil }

func newDiskTree() Tree {
	t, err := New(DevNull(), DevNull())
	if err != nil {
		panic(err)
	}
	return t
}

func testDiskTree(t *testing.T) *testTree {
	return &testTree{t: t, tree: newDiskTree()}
}

func (tt *testTree) set(key Key, val Value) {
	err := tt.tree.Set(key, val)
	if err != nil {
		tt.t.Fatalf("Set %v: %v\n\nLog:\n%s", key, err, &tt.log)
	}
	fmt.Fprintf(&tt.log, "set(%v, %v)\n", key, val)
}

func (tt *testTree) snap(version int64, hash Hash) {
	tt.t.Helper()
	snap, err := tt.tree.Snap(version)
	if err != nil {
		tt.t.Fatalf("Tree.Snap: %v\n\nLog:\n%s", err, &tt.log)
	}
	if snap.Version != version || snap.Hash != hash {
		tt.t.Fatalf("Tree.Snap = %d, %v, want %d, %v\n\nLog:\n%s",
			snap.Version, snap.Hash, version, hash, &tt.log)
	}
	fmt.Fprintf(&tt.log, "snap(%d) = %v\n", version, hash)
}

func (tt *testTree) get(key Key, val Value, ok bool) {
	tt.t.Helper()

	defer func() {
		if r := recover(); r != nil {
			tt.t.Fatalf("panic: %v\n\nLog:\n%s\n%s", r, &tt.log, debug.Stack())
		}
	}()

	snap, err := tt.tree.Snap(1)
	if err != nil {
		tt.t.Fatalf("Tree.Snap: %v\n\nLog:\n%s", err, &tt.log)
	}

	proof, err := tt.tree.Prove(key)
	if err != nil {
		tt.t.Fatalf("Tree.Prove: %v\n\nLog:\n%s", err, &tt.log)
	}

	v, o, err := Verify(snap, key, proof)
	if err != nil {
		tt.t.Fatalf("Verify %v: %v\nSnap: %v\nProof: %x\n\nLog:\n%s", key, err, snap, proof, &tt.log)
	}
	if v != val || o != ok {
		tt.t.Fatalf("get %v:\nhave %v, %v\nwant %v, %v\n\nLog:\n%s", key, v, o, val, ok, &tt.log)
	}

	//fmt.Fprintf(&tt.log, "get(%v)\n", key)
}

func BenchmarkSet1K_100K(b *testing.B) {
	b.Run("impl=mem", func(b *testing.B) { benchmarkSet(b, NewMemTree(), 1000, 100e3) })
	b.Run("impl=disk", func(b *testing.B) { benchmarkSet(b, newDiskTree(), 1000, 100e3) })
}

func BenchmarkSet1K_1M(b *testing.B) {
	b.Run("impl=mem", func(b *testing.B) { benchmarkSet(b, NewMemTree(), 1000, 1e6) })
	b.Run("impl=disk", func(b *testing.B) { benchmarkSet(b, newDiskTree(), 1000, 1e6) })
}

func BenchmarkSet1K_10M(b *testing.B) {
	b.Run("impl=mem", func(b *testing.B) { benchmarkSet(b, NewMemTree(), 1000, 10e6) })
	b.Run("impl=disk", func(b *testing.B) { benchmarkSet(b, newDiskTree(), 1000, 10e6) })
}

func benchmarkSet(b *testing.B, tree Tree, n, treeSize int) {
	println("make", treeSize)
	var todo [][2]Hash
	for i := range n {
		todo = append(todo, [2]Hash{sha(v(i)), Hash(v(i))})
	}
	for i := range treeSize {
		tree.Set(sha("old", v(i)), v(i))
	}
	tree.Snap(1)

	b.ReportAllocs()
	for b.Loop() {
		//	tree1 := *tree
		for _, kv := range todo {
			tree.Set(Key(kv[0]), Value(kv[1]))
		}
		tree.Snap(2)
	}
}

func BenchmarkProofIn100K(b *testing.B) {
	b.Run("impl=mem", func(b *testing.B) { benchmarkProof(b, NewMemTree(), 100e3) })
	b.Run("impl=disk", func(b *testing.B) { benchmarkProof(b, newDiskTree(), 100e3) })
}

func BenchmarkProofIn1M(b *testing.B) {
	b.Run("impl=mem", func(b *testing.B) { benchmarkProof(b, NewMemTree(), 1e6) })
	b.Run("impl=disk", func(b *testing.B) { benchmarkProof(b, newDiskTree(), 1e6) })
}

func BenchmarkProofIn10M(b *testing.B) {
	b.Run("impl=mem", func(b *testing.B) { benchmarkProof(b, NewMemTree(), 10e6) })
	b.Run("impl=disk", func(b *testing.B) { benchmarkProof(b, newDiskTree(), 10e6) })
}

func benchmarkProof(b *testing.B, tree Tree, treeSize int) {
	for i := range treeSize {
		tree.Set(sha("old", v(i)), v(i))
	}
	tree.Snap(1)
	key := sha("old", v(0))

	b.ReportAllocs()
	for b.Loop() {
		proof, err := tree.Prove(key)
		if err != nil {
			b.Fatal(err)
		}
		_ = proof
	}
}
