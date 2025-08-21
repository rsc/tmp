// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// TODO test constant flushing mode

package pmem

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"math/rand/v2"
	"runtime/debug"
	"testing"
)

func TestRecovery(t *testing.T) {
	for i := range 10 {
		t.Run(fmt.Sprint(i), testRecovery)
	}
}

func testRecovery(t *testing.T) {
	tmp := make([]byte, 1000)

	oldPatch := maxPatch
	oldMem := maxMem
	defer func() {
		maxPatch = oldPatch
		maxMem = oldMem
	}()

	maxPatch = 256
	maxMem = 1 << 30

	tt := &tester{t: t}
	for i := range tt.file {
		tt.file[i].tester = tt
	}

	mem, err := Create("magic", &tt.file[0], &tt.file[1])
	if err != nil {
		t.Fatal(err)
	}
	tt.setMem(mem)
	tt.markOK()

	const (
		MaxOff   = 100
		MaxCount = 100
	)
	for range 1000 {
		switch rand.N(10) {
		case 0, 1, 2, 3, 4:
			// Write many random memory sections,
			// more than will fit in a single patch block.
			for range 5 {
				off := rand.N(MaxOff)
				n := 1 + rand.N(MaxCount)
				tt.t.Logf("mutate %#x+%#x", off, n)
				_, err := mem.Expand(off + n)
				tt.markOK()
				check(tt.t, err)
				check(tt.t, mem.Mutate(mem.Data()[off:off+n], randFill(tmp[:n])))
			}

		case 5, 6, 7, 8:
			// Write a pair of grouped updates.
			// Have to limit to single patch block but try to use
			// almost the entire block so that a flush will be needed.
			n := maxPatch - 4*(maxVarint+1)
			n1 := 1 + rand.N(n-1)
			n2 := n - n1
			off1 := rand.N(MaxOff)
			off2 := rand.N(MaxOff)
			tt.t.Logf("begingroup (len=%#x)", len(mem.mem))
			check(tt.t, mem.BeginGroup())
			_, err := mem.Expand(off1 + n1)
			check(tt.t, err)
			tt.t.Logf("mutate %#x+%#x", off1, n1)
			check(tt.t, mem.Mutate(mem.Data()[off1:off1+n1], randFill(tmp[:n1])))
			_, err = mem.Expand(off2 + n2)
			check(tt.t, err)
			tt.t.Logf("mutate %#x+%#x", off2, n2)
			check(tt.t, mem.Mutate(mem.Data()[off2:off2+n2], randFill(tmp[:n2])))
			tt.t.Logf("endgroup")
			tt.markOK()
			check(tt.t, mem.EndGroup())

		case 9:
			// Sync.
			tt.t.Logf("sync")
			check(tt.t, mem.Sync())
		}
	}

	check(t, mem.Release())
	check(t, mem.UnsafeUnmap())
}

func randFill(b []byte) []byte {
	for i := range b {
		b[i] = byte(rand.N(256))
	}
	return b
}

type tester struct {
	t     *testing.T
	mem   *Mem
	file  [2]testFile
	valid map[string]bool // hashes of acceptable memory images
}

type testFile struct {
	tester  *tester
	data    []byte // data in file
	sync    int    // offset of last sync; writes only append
	current bool   // whether file is current
}

func (f *testFile) setCurrent(current bool, off int) {
	f.current = current
	f.data = f.data[:off]
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

	// Try corrupting the writes and see what happens.
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
// After Sync, bytes before the current offset cannot be lost or corrupted.
func (f *testFile) Sync() error {
	if f.tester == nil {
		return nil
	}

	f.sync = len(f.data)
	f.tester.t.Logf("%s sync at %#x", f.name(), f.sync)
	f.tester.try(f)
	return nil
}

func (tt *tester) setMem(mem *Mem) {
	tt.mem = mem
	mem.syncHook = tt.syncHook
	mem.mutateHook = tt.markOK
	if tt.valid == nil {
		tt.valid = make(map[string]bool)
	}
	h := tt.mem.hash()
	tt.t.Logf("initial hash %v", h)
	tt.valid[h] = true
}

func (tt *tester) markOK() {
	tt.t.Helper()
	h := tt.mem.hash()
	tt.t.Logf("ok %s", h)
	tt.valid[h] = true
}

func (tt *tester) syncHook() {
	clear(tt.valid) // older snapshots no longer acceptable
	tt.markOK()
}

// try tries reopening the files with various i/o problems.
func (tt *tester) try(f *testFile) {
	if tt.mem == nil {
		// Initial tree not created yet.
		return
	}

	tt.reopen("as written")

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
	mem, err := Open("magic", tt.file[0].clone(), tt.file[1].clone())
	if err != nil {
		tt.t.Fatalf("reopen: %s: %v\n\n%s", kind, err, hex.Dump(tt.file[0].data))
	}
	h := mem.hash()
	if !tt.valid[h] {
		tt.t.Fatalf("reopen (%d %d): %s: invalid hash %v want %v\n\n%s\n\n%s\n\n%s", len(tt.file[0].data), len(tt.file[1].data), kind, h, tt.valid, debug.Stack(), hex.Dump(tt.mem.mem), hex.Dump(mem.mem))
	}
	check(tt.t, mem.Release())
	check(tt.t, mem.UnsafeUnmap())
}

func check(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
