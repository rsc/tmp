// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mpt

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"

	"rsc.io/tmp/mpt/internal/pmem"
)

// Tree Format
//
// The tree memory starts with a header:
//
//	version [8]
//	dirty [1]
//	pad [1]
//	root [6]
//	hash [32]
//	nodes [8]
//
// The header is followed by a sequence of Patricia nodes of the form:
//
//	key [32]
//	val [32]
//	bit [1]
//	dirty [1]
//	pad [2]
//	left [6]
//	right [6]
//	ihash [32]
//
// The root, left, and right “pointers” are byte offsets from the start of the tree memory.
// A nil pointer is stored as offset 0, which would otherwise point at the tree header.

const (
	// header offsets
	hdrVersion = 0
	hdrDirty   = 8
	hdrRoot    = 10
	hdrHash    = 16
	hdrNodes   = 48
	hdrSize    = 56

	// node offsets
	// setFields knows that key, val, bits are contiguous.
	// setLeftRight knows that left and right are contiguous.
	nodeKey   = 0
	nodeVal   = 32
	nodeUbit  = 64
	nodeDirty = 65
	nodeLeft  = 68
	nodeRight = 74
	nodeIHash = 80
	nodeSize  = 112

	// address size
	addrSize = 6
)

// File is the interface needed for on-disk storage.
type File interface {
	io.ReaderAt
	io.WriterAt
	io.Closer
	Sync() error
}

// File must implement pmem.File.
// It really should be exactly pmem.File but we don't want to
// expose pmem in the API definitions, so File is a copy instead.
var _ pmem.File = File(nil)

// A diskTree is an on-disk [Tree].
type diskTree struct {
	pmem   *pmem.Mem
	mem    []byte // cache of pmem.Data()
	file1  File
	file2  File
	closed bool
	err    error // sticky error
}

// broken marks the tree broken with err as the reason.
// Any method on t or function taking a t as an argument
// is expected to call t.broken for I/O or data corruption errors.
// If the error comes from another method on t or function taking t as an argument,
// then that callee can be assumed to have called t.broken.
func (t *diskTree) broken(err error) error {
	if t.err == nil {
		t.err = err
	}
	return err
}

// Create creates a new, empty on-disk [Tree] stored in the two named files.
// The files must not already exist, unless they are both os.DevNull,
// in which case the Tree is held only in memory.
func Create(file1, file2 string) (Tree, error) {
	return open(file1, file2, os.O_WRONLY|os.O_CREATE|os.O_EXCL, "create")
}

// Open opens an on-disk [Tree] stored in the two named files.
// The files must have been created by a previous call to [Create].
func Open(file1, file2 string) (Tree, error) {
	return open(file1, file2, os.O_RDWR, "open")
}

func open(file1, file2 string, mode int, op string) (Tree, error) {
	f1, err := os.OpenFile(file1, mode, 0666)
	if err != nil {
		return nil, err
	}
	f2, err := os.OpenFile(file2, mode, 0666)
	if err != nil {
		f1.Close()
		return nil, err
	}

	return memOpen(f1, f2, op)
}

// New creates or opens an on-disk [Tree] in the given files.
// If both files are empty, New creates a new tree in those files.
// Otherwise, New opens a pre-existing tree stored in those files.
// Only one file contains the latest tree at a time, but the
// implementation alternates between files to implement atomic updates.
func New(file1, file2 File) (Tree, error) {
	var op string
	var buf [1]byte
	n1, err1 := file1.ReadAt(buf[:], 0)
	n2, err2 := file2.ReadAt(buf[:], 0)
	if n1 == 0 && n2 == 0 && err1 == io.EOF && err2 == io.EOF {
		op = "create"
	} else {
		op = "open"
	}
	return memOpen(file1, file2, op)
}

func setActive(f File, b bool) {
	if f, ok := f.(interface{ setActive(bool) }); ok {
		f.setActive(b)
	}
}

// memOpen is the general implementation of open.
// op is "create", "open", or "new", indicating the operation
// being performed on the files; sync indicates whether to
// try to use the files' Sync method.
// (When using /dev/null for an in-memory tree,
// we avoid calling Sync, because it will fail.)
func memOpen(file1, file2 File, op string) (_ Tree, err error) {
	pmemOp := pmem.Open
	if op == "create" {
		pmemOp = pmem.Create
	}
	mem, err := pmemOp("mpt tree\n", file1, file2)
	if err != nil {
		return nil, err
	}
	t := &diskTree{
		pmem:  mem,
		file1: file1,
		file2: file2,
	}
	defer func() {
		if err != nil {
			mem.Release()
			mem.UnsafeUnmap()
		}
	}()

	runtime.AddCleanup(t, func(*struct{}) { mem.Release() }, nil)

	if op == "create" {
		// Write initial tree.
		mem, err := t.pmem.Expand(hdrSize)
		if err != nil {
			return nil, err
		}
		h := emptyTreeHash()
		if err := t.mutate(mem[hdrHash:], h[:]); err != nil {
			return nil, err
		}
		if err := t.pmem.Sync(); err != nil {
			return nil, err
		}
	}

	t.mem = t.pmem.Data()

	return t, nil
}

var errCorrupt = errors.New("corrupt tree data")

// Sync syncs written data to disk.
func (t *diskTree) Sync() error {
	if t.err != nil {
		return t.err
	}
	if err := t.pmem.Sync(); err != nil {
		return t.broken(err)
	}
	return nil
}

// TODO figure out whether pmem should Close.

// Close closes the tree and the files it uses.
func (t *diskTree) Close() error {
	if err := t.Sync(); err != nil {
		t.broken(err)
	}
	if t.closed {
		return fmt.Errorf("tree already closed")
	}
	if err := t.pmem.Release(); err != nil {
		t.broken(err)
	}
	if err := t.file1.Close(); err != nil {
		t.broken(err)
	}
	if err := t.file2.Close(); err != nil {
		t.broken(err)
	}
	if t.err != nil {
		return t.err
	}
	t.closed = true
	t.err = errors.New("tree is closed") // stop future method calls
	return nil
}

func (t *diskTree) UnsafeUnmap() error {
	if !t.closed {
		return fmt.Errorf("UnsafeUnmap without Close")
	}
	return t.pmem.UnsafeUnmap()
}

// TODO: should mutate be done by editing dst in place and then calling t.mutated(dst)?

// mutate is like copy(dst, src) where dst is inside t.mem.
// It also records the mutation in the patch buffer, to be written
// to disk when the current patch block fills or Sync is called.
func (t *diskTree) mutate(dst, src []byte) error {
	n := min(len(dst), len(src))
	if err := t.pmem.Mutate(dst[:n], src[:n]); err != nil {
		return t.broken(err)
	}
	return nil
}

// addrToMem returns the tree memory at address a and length n.
func (t *diskTree) addrToMem(a addr, n int) ([]byte, error) {
	if a > addr(len(t.mem)) || len(t.mem)-int(a) < n {
		return nil, t.broken(errCorrupt)
	}
	return t.mem[a : a+addr(n)], nil
}

// memToAddr converts a byte slice p, which must be from t.mem,
// into an addr.
func (t *diskTree) memToAddr(p []byte) addr {
	off, ok := t.pmem.Offset(p)
	if !ok {
		panic("mpt: memToAddr misuse")
	}
	return addr(off)
}

// alloc allocates n more bytes of tree memory, returning it as a slice.
func (t *diskTree) alloc(n int) ([]byte, error) {
	if cap(t.mem)-len(t.mem) < n {
		mem, err := t.pmem.Expand(len(t.mem) + n)
		if err != nil {
			t.err = err
			return nil, err
		}
		t.mem = mem[:len(t.mem)]
	}
	off := len(t.mem)
	t.mem = t.mem[:off+n]
	return t.mem[off : off+n], nil
}

// An addr is an offset into the disk layout.
// It is stored on disk as a 48-bit big-endian value.
type addr uint64

// parseAddr returns the node address at the given byte offset.
func parseAddr(p []byte) addr {
	return addr(binary.BigEndian.Uint16(p))<<32 | addr(binary.BigEndian.Uint32(p[2:]))
}

// putAddr stores the node address at the given byte offset.
func putAddr(p []byte, a addr) {
	binary.BigEndian.PutUint32(p[2:], uint32(a))
	binary.BigEndian.PutUint16(p, uint16(a>>32))
}

// A diskHdr is the memory copy of the tree header.
type diskHdr [hdrSize]byte

func (h *diskHdr) version() int64 { return int64(binary.BigEndian.Uint64(h[hdrVersion:])) }
func (h *diskHdr) dirty() bool    { return h[hdrDirty] != 0 }
func (h *diskHdr) root() addr     { return parseAddr(h[hdrRoot:]) }
func (h *diskHdr) hash() Hash     { return Hash(h[hdrHash:]) }
func (h *diskHdr) nodes() int     { return int(binary.BigEndian.Uint64(h[hdrNodes:])) }

func (h *diskHdr) setVersion(t *diskTree, version int64) error {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(version))
	return t.mutate(h[hdrVersion:], buf[:])
}

func (h *diskHdr) setDirty(t *diskTree, d bool) error {
	var buf [1]byte
	if d {
		buf[0] = 1
	}
	return t.mutate(h[hdrDirty:], buf[:])
}

func (h *diskHdr) setRoot(t *diskTree, n *diskNode) error {
	a := t.addr(n)
	var buf [6]byte
	putAddr(buf[:], a)
	return t.mutate(h[hdrRoot:], buf[:])
}

func (h *diskHdr) setHash(t *diskTree, hash Hash) error {
	return t.mutate(h[hdrHash:], hash[:])
}

func (h *diskHdr) setNodes(t *diskTree, n int) error {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(n))
	return t.mutate(h[hdrNodes:], buf[:])
}

// hdr returns a pointer to the in-memory tree header.
func (t *diskTree) hdr() *diskHdr {
	mem, err := t.addrToMem(0, hdrSize)
	if err != nil {
		panic(err) // mem should always be big enough for the header
	}
	return (*diskHdr)(mem)
}

// A diskNode is the memory copy of a node.
// The *diskNodes passed around in this implementation
// are pointers into the in-memory copy t.mem.
type diskNode [nodeSize]byte

// node returns the diskNode at the given address.
func (t *diskTree) node(a addr) (*diskNode, error) {
	if a == 0 {
		return nil, nil
	}
	mem, err := t.addrToMem(a, nodeSize)
	if err != nil {
		return nil, err
	}
	return (*diskNode)(mem), nil
}

// addr returns the address of the given diskNode.
func (t *diskTree) addr(n *diskNode) addr {
	if n == nil {
		return 0
	}
	return t.memToAddr(n[:])
}

// addrAt reads a node address from the address a.
// The caller must ensure that a is a valid address,
// or else addrAt panics.
func (t *diskTree) addrAt(a addr) addr {
	mem, err := t.addrToMem(a, addrSize)
	if err != nil {
		panic(err)
	}
	return parseAddr(mem)
}

// setAddrAt writes the node address b to the address a.
func (t *diskTree) setAddrAt(a, b addr) error {
	mem, err := t.addrToMem(a, addrSize)
	if err != nil {
		return err
	}
	var buf [addrSize]byte
	putAddr(buf[:], b)
	return t.mutate(mem, buf[:])
}

// newNode allocates and returns a new node in the tree.
func (t *diskTree) newNode() (*diskNode, error) {
	n, err := t.alloc(nodeSize)
	if err != nil {
		return nil, err
	}
	return (*diskNode)(n), nil
}

func (n *diskNode) key() Key    { return Key(n[nodeKey:]) }
func (n *diskNode) val() Value  { return Value(n[nodeVal:]) }
func (n *diskNode) dirty() bool { return n[nodeDirty] != 0 }
func (n *diskNode) left() addr  { return parseAddr(n[nodeLeft:]) }
func (n *diskNode) right() addr { return parseAddr(n[nodeRight:]) }
func (n *diskNode) ihash() Hash { return Hash(n[nodeIHash:]) }

// bit returns the bit number recorded in the node.
// The single leaf node that is not also an inner node,
// identified by having no children, has bit number -1.
func (n *diskNode) bit() int {
	if n.left() == 0 && n.right() == 0 {
		return -1
	}
	return int(n[nodeUbit])
}

// init initializes the node n with the given key, val, bit, left, and right;
// it also sets dirty=true and clears ihash.
func (n *diskNode) init(t *diskTree, key Key, val Value, bit int, left, right *diskNode) error {
	var buf [nodeSize]byte
	copy(buf[nodeKey:], key[:])
	copy(buf[nodeVal:], val[:])
	buf[nodeUbit] = byte(bit)
	buf[nodeDirty] = 1
	putAddr(buf[nodeLeft:], t.addr(left))
	putAddr(buf[nodeRight:], t.addr(right))
	return t.mutate(n[:], buf[:])
}

func (n *diskNode) setVal(t *diskTree, val Value) error { return t.mutate(n[nodeVal:], val[:]) }
func (n *diskNode) setIHash(t *diskTree, h Hash) error  { return t.mutate(n[nodeIHash:], h[:]) }
func (n *diskNode) setDirty(t *diskTree, d bool) error {
	var p [1]byte
	if d {
		p[0] = 1
	}
	return t.mutate(n[nodeDirty:], p[:])
}

func (t *diskTree) memHash() string {
	h := sha256.Sum256(t.mem)
	s := base64.StdEncoding.EncodeToString(h[:])
	return fmt.Sprintf("%s/%#x", s[:7], len(t.mem))
}
