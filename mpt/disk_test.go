// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mpt

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"maps"
	"math/rand/v2"
	"runtime/debug"
	"slices"
	"testing"
)

// A memFile is an in-memory file with ReadAt, WriteAt, Close, and Sync methods.
type memFile struct {
	readOnly bool
	data     []byte
}

func (f *memFile) ReadAt(data []byte, off int64) (int, error) {
	if off < 0 || off >= int64(len(f.data)) {
		return 0, io.EOF
	}
	n := copy(data, f.data[off:])
	if n < len(data) {
		return n, io.ErrUnexpectedEOF
	}
	return n, nil
}

func (f *memFile) WriteAt(data []byte, off int64) (int, error) {
	if f.readOnly {
		panic("write to read-only file")
	}
	if off > int64(len(f.data)) {
		// Fill hole in file.
		f.data = append(f.data, make([]byte, int(off)-len(f.data))...)
	}
	n := copy(f.data[off:], data)
	f.data = append(f.data, data[n:]...)
	return len(data), nil
}

func (f *memFile) Close() error {
	return nil
}

func (f *memFile) Sync() error {
	return nil
}

func memHash(t *diskTree) string {
	h := sha256.New()
	h.Write(t.mem)
	n := 1 + (len(t.mem)-hdrSize)/nodeSize
	const pmemHdrSize = 16
	leaf := pmemHdrSize + n*64
	switch f := t.leaf.(type) {
	default:
		panic(fmt.Sprintf("unknown leaf type %T", t.leaf))
	case *memFile:
		h.Write(f.data[:leaf])
	case *testFile:
		if len(f.data) != leaf {
			panic(fmt.Sprintf("unexpected leaf size in real tree: %d != %d (t.mem=%d)", len(f.data), leaf, len(t.mem)))
		}
		h.Write(f.data)
	}
	s := base64.StdEncoding.EncodeToString(h.Sum(nil))
	return fmt.Sprintf("%s/%#x", s[:7], len(t.mem))
}

// A tester is a two-file simulator that checks after each write that
// reopening the disk works properly, even if the write only happens
// partially or even gets corrupted (unlikely but we can handle it).
type tester struct {
	t      *testing.T
	tree   *diskTree       // in-memory tree
	file   [3]testFile     // files backing tree
	valid  map[string]bool // hashes of acceptable tree memory images
	replay []int           // replay log for recovery
}

// A testFile is a single simulated file.
type testFile struct {
	memFile
	tester  *tester
	sync    int  // offset of last sync; writes only append
	current bool // whether file is current
}

func (f *testFile) name() string {
	if f.tester == nil {
		return "???"
	}
	for i := range 3 {
		if f == &f.tester.file[i] {
			return fmt.Sprint("file", i+1)
		}
	}
	return "???"
}

func (f *testFile) setCurrent(current bool) {
	f.current = current
}

func (f *testFile) clone() *memFile {
	return &memFile{readOnly: true, data: bytes.Clone(f.data)}
}

// WriteAt writes to the test file.
func (f *testFile) WriteAt(data []byte, off int64) (int, error) {
	// Writes to the current file should only ever append;
	// not overwriting is part of our reliability story.
	// Writes to the next file can be scattered, because
	// we are writing the tree interleaved with new patches.
	if f.current && off != int64(len(f.data)) {
		return 0, fmt.Errorf("non-appending write\n\n%s", debug.Stack())
	}
	f.tester.t.Logf("%s write %#x+%#x = %#x", f.name(), off, len(data), off+int64(len(data)))
	return f.memFile.WriteAt(data, off)
}

// Sync syncs the test file.
// Now bytes before the current offset cannot be lost or corrupted.
func (f *testFile) Sync() error {
	if f.tester == nil {
		panic("sync of read-only file")
	}

	f.sync = len(f.data)
	f.tester.t.Logf("%s sync at %#x", f.name(), f.sync)
	return nil
}

func (tt *tester) markOK() {
	h := memHash(tt.tree)
	tt.t.Logf("ok %v", h)
	tt.valid[h] = true
}

func (tt *tester) test(minVer int64, minExact bool) {
	tt.try(&tt.file[0], minVer, minExact)
	tt.try(&tt.file[1], minVer, minExact)
}

// try tries reopening the files with various i/o problems.
func (tt *tester) try(f *testFile, minVer int64, minExact bool) {
	if tt.tree == nil {
		// Initial tree not created yet.
		return
	}

	// Test file with write actually succeeding.
	tt.reopen(minVer, minExact, "as written")
}

func (tt *tester) reopen(minVer int64, minExact bool, format string, args ...any) {
	kind := fmt.Sprintf(format, args...)
	f1 := tt.file[0].clone()
	f2 := tt.file[1].clone()
	f3 := tt.file[2].clone()
	f3.readOnly = false
	tree, err := New(f1, f2, f3)
	if err != nil {
		tt.t.Fatalf("reopen: %s: %v", kind, err)
	}
	defer tree.Close()

	version, exact := tree.Version()
	if err != nil {
		tt.t.Fatalf("reopen: %s: %v", kind, err)
	}
	if version < minVer || minExact != exact {
		tt.t.Fatalf("reopen: %s: version = %d,%v, want ≥ %d,%v", kind, version, exact, minVer, minExact)
	}
	if !exact {
		f1.readOnly = false
		f2.readOnly = false

		// Find [-1, version] marking snapshot of recorded version.
		i := 0
		if version > 0 {
			for i < len(tt.replay) && (tt.replay[i] != -1 || int64(tt.replay[i+1]) != version) {
				i += 2
			}
			if i >= len(tt.replay) {
				tt.t.Fatalf("reopen: %s: recover %d %v: cannot find version %d", kind, version, exact, version)
			}
			i += 2
		}
		// Replay rest of log.
		for ; i < len(tt.replay); i += 2 {
			if tt.replay[i] == -1 {
				if _, err := tree.Snap(int64(tt.replay[i+1])); err != nil {
					tt.t.Fatalf("reopen: %s: Snap: %v", kind, err)
				}
			} else {
				if err := tree.Set(Key(v(tt.replay[i])), v(tt.replay[i+1])); err != nil {
					tt.t.Fatalf("reopen: %s: Set: %v", kind, err)
				}
			}
		}
	}

	h := memHash(tree.(*diskTree))
	if !tt.valid[h] {
		tt.t.Fatalf("reopen (%d %d): %s: (%d %v): invalid hash %v want %v\n\n%s\nactual tree:\n%s\nrecovered tree:\n%s\nactual leaf:\n%s\nrecovered leaf (%v):\n%s",
			len(tt.file[0].data), len(tt.file[1].data), kind,
			version, exact,
			h, slices.Sorted(maps.Keys(tt.valid)),
			debug.Stack(),
			hexDump(tt.tree.mem),
			hexDump(tree.(*diskTree).mem),
			hexDump(tt.tree.leaf.(*testFile).data),
			tree.(*diskTree).leaf.(*memFile) == f3,
			hexDump(tree.(*diskTree).leaf.(*memFile).data))
	}
}

func hexDump(data []byte) string {
	return hex.Dump(data[:min(len(data), 1024)])
}

// TODO maybe for testing enable a pmem mode that
// writes every mutation to a separate patch,
// and then reopen after every file write?

func TestDiskRecovery(t *testing.T) {
	for i := range 10 {
		t.Run(fmt.Sprint(i), testDiskRecovery)
	}
}

func testDiskRecovery(t *testing.T) {
	tt := &tester{t: t}
	for i := range tt.file {
		tt.file[i].tester = tt
	}

	xtree, err := New(&tt.file[0], &tt.file[1], &tt.file[2])
	if err != nil {
		t.Fatal(err)
	}
	tree := xtree.(*diskTree)
	defer tree.Close() // relelase pmem on test failure

	tree.pmem.SetConstantFlushing(true)
	tt.tree = tree
	tt.valid = make(map[string]bool)
	tt.markOK()
	version := int64(0)
	exact := false
	syncVersion := version
	syncExact := false

	for range 10 {
		switch r := rand.N(10); r {
		default:
			i := rand.N(100)
			j := rand.N(100)
			t.Logf("set %d %d", i, j)
			tt.replay = append(tt.replay, i, j)
			check(t, tree.Set(Key(v(i)), v(j)))
			exact = false
			syncExact = false
			tt.markOK()
			tt.test(syncVersion, syncExact)

		case 0, 1:
			version++
			exact = true
			t.Logf("snap %d", version)
			tt.replay = append(tt.replay, -1, int(version))
			_, err := tree.Snap(version)
			check(t, err)
			tt.markOK()
			tt.test(syncVersion, syncExact)
			fallthrough

		case 3:
			t.Log("sync")
			check(t, tree.Sync())
			syncVersion = version
			syncExact = exact
			clear(tt.valid)
			tt.markOK()
			tt.test(syncVersion, syncExact)
		}
	}

	check(t, tree.Close())
}

func TestDiskReopen(t *testing.T) {
	// Test that very basic tree written to disk can be reopened, restored.
	// Simulations are all well and good, but test real files a bit too.
	dir := t.TempDir()
	tree1, err := Create(dir+"/tree1", dir+"/tree2", dir+"/disk")
	if err != nil {
		t.Fatal(err)
	}
	check(t, err)
	defer tree1.Close()

	for i := range 10 {
		check(t, tree1.Set(Key(v(i)), v(i)))
	}

	_, err = tree1.Snap(1)
	check(t, err)
	check(t, tree1.Sync())

	tree2, err := Open(dir+"/tree1", dir+"/tree2", dir+"/disk")
	check(t, err)
	defer tree2.Close()

	if !bytes.Equal(tree1.(*diskTree).mem, tree2.(*diskTree).mem) {
		t.Fatalf("tree memory differs\n\n%s\n\n%s",
			hex.Dump(tree1.(*diskTree).mem[:1024]),
			hex.Dump(tree2.(*diskTree).mem[:1024]))
	}
}

func check(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
