// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mpt

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"math/rand/v2"
	"runtime/debug"
	"testing"
)

// A tester is a two-file simulator that checks after each write that
// reopening the disk works properly, even if the write only happens
// partially or even gets corrupted (unlikely but we can handle it).
type tester struct {
	t     *testing.T
	tree  *diskTree       // in-memory tree
	file  [2]testFile     // files backing tree
	valid map[string]bool // hashes of acceptable tree memory images
}

// A testFile is a single simulated file.
type testFile struct {
	tester  *tester
	data    []byte // data in file
	sync    int    // offset of last sync; writes only append
	current bool   // whether file is current
}

func (f *testFile) setCurrent(current bool) {
	f.current = current
}

func (f *testFile) clone() *testFile {
	return &testFile{data: bytes.Clone(f.data)}
}

// ReadAt reads from the test file.
func (f *testFile) ReadAt(data []byte, off int64) (int, error) {
	if off < 0 || off >= int64(len(f.data)) {
		return 0, io.EOF
	}
	n := copy(data, f.data[off:])
	if n < len(data) {
		return n, io.ErrUnexpectedEOF
	}
	return n, nil
}

// WriteAt writes to the test file.
func (f *testFile) WriteAt(data []byte, off int64) (int, error) {
	if f.tester == nil {
		panic("write to read-only file")
	}

	// Writes to the current file should only ever append;
	// not overwriting is part of our reliability story.
	// Writes to the next file can be scattered, because
	// we are writing the tree interleaved with new patches.
	if f.current && off != int64(len(f.data)) {
		return 0, fmt.Errorf("non-appending write\n\n%s", debug.Stack())
	}
	if off > int64(len(f.data)) {
		// Fill hole in file.
		f.data = append(f.data, make([]byte, int(off)-len(f.data))...)
	}
	f.tester.t.Logf("%s write %#x+%#x = %#x", f.name(), off, len(data), off+int64(len(data)))
	n := copy(f.data[off:], data)
	f.data = append(f.data, data[n:]...)
	f.tester.try(f)
	return len(data), nil
}

// Close closes the test file.
func (f *testFile) Close() error {
	return nil
}

func (f *testFile) name() string {
	if f.tester == nil {
		return "???"
	}
	if f == &f.tester.file[0] {
		return "file0"
	}
	return "file1"
}

// Sync syncs the test file.
// Now bytes before the current offset cannot be lost or corrupted.
func (f *testFile) Sync() error {
	if f.tester == nil {
		panic("sync of read-only file")
	}

	f.sync = len(f.data)
	f.tester.t.Logf("%s sync at %#x", f.name(), f.sync)
	clear(f.tester.valid) // older snapshots no longer acceptable
	f.tester.try(f)
	return nil
}

func (tt *tester) setTree(tree *diskTree) {
	tt.tree = tree
	if tt.valid == nil {
		tt.valid = make(map[string]bool)
	}
	h := tt.tree.memHash()
	tt.t.Logf("initial hash %v", h)
	tt.valid[h] = true
}

// try tries reopening the files with various i/o problems.
func (tt *tester) try(f *testFile) {
	if tt.tree == nil {
		// Initial tree not created yet.
		return
	}

	h := tt.tree.memHash()
	tt.t.Logf("hash %v", h)
	tt.valid[h] = true

	// Test file truncated to last sync.
	whole := f.data
	f.data = whole[:f.sync]
	tt.reopen("truncated to last sync at %#x", f.sync)

	// Test file truncated past the sync point.
	if n := len(whole) - f.sync; n >= 2 {
		for range 5 {
			pos := f.sync + 1 + rand.N(n-1)
			f.data = whole[:pos]
			tt.reopen("truncated to %#x", pos)
		}
	}

	// Test file with correct length but corrupt data past the sync point.
	f.data = whole
	if len(f.data) > f.sync {
		for range 5 {
			pos := f.sync + rand.N(len(f.data)-f.sync)
			f.data[pos] ^= 1
			tt.reopen("corrupted at %#x", pos)
			f.data[pos] ^= 1
		}
	}

	// Test file with write actually succeeding.
	tt.reopen("as written")
}

func (tt *tester) reopen(format string, args ...any) {
	kind := fmt.Sprintf(format, args...)
	tree, err := New(tt.file[0].clone(), tt.file[1].clone())
	if err != nil {
		tt.t.Fatalf("reopen: %s: %v", kind, err)
	}
	h := tree.(*diskTree).memHash()
	if !tt.valid[h] {
		tt.t.Fatalf("reopen (%d %d): %s: invalid hash %v want %v\n\n%s\n\n%s\n\n%s", len(tt.file[0].data), len(tt.file[1].data), kind, h, tt.valid, debug.Stack(), hex.Dump(tt.tree.mem), hex.Dump(tree.(*diskTree).mem))
	}
}

func TestDiskRecovery(t *testing.T) {
	for i := range 10 {
		t.Run(fmt.Sprint(i), testDiskRecovery)
	}
}

func testDiskRecovery(t *testing.T) {
	oldPatch := maxPatch
	oldTree := maxTree
	defer func() {
		maxPatch = oldPatch
		maxTree = oldTree
	}()

	maxPatch = 256 + 7 + 32
	maxTree = 1 << 30

	tt := &tester{t: t}
	for i := range tt.file {
		tt.file[i].tester = tt
	}

	tree, err := New(&tt.file[0], &tt.file[1])
	if err != nil {
		t.Fatal(err)
	}
	tt.setTree(tree.(*diskTree))

	for range 1000 {
		switch rand.N(10) {
		default:
			i := rand.N(100)
			j := rand.N(100)
			t.Logf("set %d %d", i, j)
			check(t, tree.Set(Key(v(i)), v(j)))

		case 0:
			t.Log("snap")
			_, err := tree.Snap()
			check(t, err)

		case 1:
			t.Log("sync")
			check(t, tree.Sync())
		}
	}

	check(t, tree.Close())
}

func TestDiskReopen(t *testing.T) {
	// Test that very basic tree written to disk can be reopened, restored.
	// Simulations are all well and good, but test real files a bit too.
	dir := t.TempDir()
	tree1, err := Create(dir+"/tree1", dir+"/tree2")
	if err != nil {
		t.Fatal(err)
	}
	check(t, err)
	defer tree1.Close()

	for i := range 10 {
		check(t, tree1.Set(Key(v(i)), v(i)))
	}

	_, err = tree1.Snap()
	check(t, err)
	check(t, tree1.Sync())

	tree2, err := Open(dir+"/tree1", dir+"/tree2")
	check(t, err)
	defer tree2.Close()

	if !bytes.Equal(tree1.(*diskTree).mem, tree2.(*diskTree).mem) {
		t.Fatalf("tree memory differs\n\n%s\n\n%s",
			hex.Dump(tree1.(*diskTree).mem),
			hex.Dump(tree2.(*diskTree).mem))
	}
}

func check(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
