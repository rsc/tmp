// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mpt

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"

	"rsc.io/tmp/mpt/internal/slicemath"
	"rsc.io/tmp/mpt/internal/span"
)

// On-Disk Tree
//
// The layout of the disk representation is:
//
//	treeID   [16]
//	treeSeq  [8]
//	treeLen  [8]
//	treeMem  [treeLen]
//	checksum [32]
//	patch blocks
//
// The checksum is a SHA256 of the data from treeID through treeMem.
//
// Each patch block is
//
//	treeID	 [16]
//	treeSeq	 [8]
//	patchLen [8]
//	patchMem [patchLen]
//	checksum [32]
//
// The patchMem layout is a sequence of mutations:
//
//	offset [6]
//	len    [1]
//	data   [len]
//
// Note that the initial tree and each patch block have the same framing,
// but they differ in the content: the initial tree is the actual memory,
// while each patch block contains a sequence of mutations to that memory
// (or extending that memory).
//
// The actual tree memory starts with a header:
//
//	epoch [8]
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
	hdrEpoch = 0
	hdrDirty = 8
	hdrRoot  = 10
	hdrHash  = 16
	hdrNodes = 48
	hdrSize  = 56

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

	// framing before memory image or patch block
	frameID   = 0
	frameSeq  = 16
	frameLen  = 24
	frameSize = 32

	// patch size
	patchMax   = 64 << 20
	patchFlush = patchMax - 1024 // actual is 7+255+32 but 1024 is less error-prone

	treeMax = 16 << 40
)

// A diskTree is an on-disk [Tree].
type diskTree struct {
	file1   *os.File
	file2   *os.File
	id      [16]byte
	current diskFile
	next    diskFile
	span    *span.Span
	mem     []byte
	patch   []byte // pending patch
	err     error
}

type diskFile struct {
	file *os.File
	seq  uint64
	off  int64
}

// Create creates a new, empty on-disk [Tree] stored in the two named files.
// At any moment, one file or the other contains the entire tree,
// but the implementation flips between the two files to implement
// reliable updates.
//
// The files must not already exist, unless they are both os.DevNull,
// in which case the Tree is held only in memory.
func Create(file1, file2 string) (Tree, error) {
	mode := os.O_WRONLY | os.O_CREATE | os.O_EXCL
	if file1 == os.DevNull && file2 == os.DevNull {
		mode &^= os.O_EXCL
	}
	return open(file1, file2, mode, "create")
}

// Open opens an on-disk [Tree] stored in the two named files.
// At any moment, one file or the other contains the entire tree,
// but the implementation flips between the two files to implement
// reliable updates.
//
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

	sp, err := span.Reserve(1 << 44)
	if err != nil {
		return nil, err
	}
	t := &diskTree{span: sp}
	runtime.AddCleanup(t, func(*struct{}) { sp.Release() }, nil)

	if op == "create" {
		mem, err := sp.Expand(hdrSize)
		if err != nil {
			return nil, err
		}
		t.mem = mem
		*(*Hash)(mem[hdrHash:]) = emptyTreeHash()
		rand.Read(t.id[:])
		t.current = diskFile{file: f1}
		t.next = diskFile{file: f2}
		if err := t.writeTree(&t.current); err != nil {
			return nil, err
		}
		if err := t.writeTree(&t.next); err != nil {
			return nil, err
		}
		return t, nil
	}

	id1, seq1, n1, err := readFrameInfo(t.file1)
	if err != nil {
		return nil, err
	}
	id2, seq2, n2, err := readFrameInfo(t.file2)
	if err != nil {
		return nil, err
	}
	if id1 != id2 {
		return nil, fmt.Errorf("inconsistent tree files: id %x != %x", id1[:], id2[:])
	}
	t.id = id1
	if seq1 != 0 && seq1 == seq2 {
		return nil, fmt.Errorf("inconsistent tree files: both seq %d", seq1)
	}

	t.current = diskFile{file: f1, seq: seq1}
	t.next = diskFile{file: f2, seq: seq2}
	if t.current.seq < t.next.seq {
		t.current, n1, t.next, n2 = t.next, n2, t.current, n1
	}
	if err := t.readTree(&t.current, n1); err != nil {
		t.current, n1, t.next, n2 = t.next, n2, t.current, n1
		if err := t.readTree(&t.current, n1); err != nil {
			return t, err
		}
	}

	// TODO maybe start a compaction

	return t, nil
}

func (t *diskTree) readTree(f *diskFile, treeLen int) error {
	mem, err := t.span.Expand(treeLen)
	if err != nil {
		return err
	}
	if n, err := t.readFrame(f, mem); err != nil {
		return err
	} else if n != treeLen {
		return errCorrupt
	}
	t.mem = mem

	// TODO read patches

	return nil
}

func readFrameInfo(f *os.File) (id [16]byte, seq uint64, len int, err error) {
	var buf [frameSize]byte
	if _, err = f.ReadAt(buf[:], 0); err != nil {
		return
	}
	copy(id[:], buf[frameID:])
	seq = binary.BigEndian.Uint64(buf[frameSeq:])
	n := binary.BigEndian.Uint64(buf[frameLen:])
	if uint64(int(n)) != n || int(n) < 0 || n > treeMax {
		err = fmt.Errorf("invalid length %#x", n)
	}
	return
}

var errCorrupt = errors.New("corrupt data")

func (t *diskTree) readFrame(f *diskFile, data []byte) (int, error) {
	var frame [frameSize]byte
	if _, err := f.file.ReadAt(frame[:], f.off); err != nil {
		return 0, err
	}
	if [16]byte(frame[frameID:]) != t.id {
		return 0, errCorrupt
	}
	if binary.BigEndian.Uint64(frame[frameSeq:]) != f.seq {
		return 0, errCorrupt
	}
	n := binary.BigEndian.Uint64(frame[frameLen:])
	if n > uint64(len(data)) {
		return 0, errCorrupt
	}

	if _, err := f.file.ReadAt(data[:n], f.off+frameSize); err != nil {
		if err == io.ErrUnexpectedEOF {
			return 0, errCorrupt
		}
		return 0, err
	}

	var fsum [32]byte
	if _, err := f.file.ReadAt(fsum[:], f.off+frameSize+int64(n)); err != nil {
		if err == io.ErrUnexpectedEOF {
			return 0, errCorrupt
		}
		return 0, err
	}
	sum := sha2(frame[:], data)
	if sum != fsum {
		return 0, errCorrupt
	}
	f.off += frameSize + int64(n) + 32

	return int(n), nil
}

func (t *diskTree) writeTree(f *diskFile) error {
	return t.writeFrame(f, t.mem)
}

func (t *diskTree) writeFrame(f *diskFile, data []byte) error {
	var frame [frameSize]byte
	copy(frame[frameID:], t.id[:])
	binary.BigEndian.PutUint64(frame[frameSeq:], f.seq)
	binary.BigEndian.PutUint64(frame[frameLen:], uint64(len(data)))
	sum := sha2(frame[:], data)

	if _, err := f.file.WriteAt(frame[:], f.off); err != nil {
		panic(err)
		return err
	}
	if _, err := f.file.WriteAt(data, f.off+frameSize); err != nil {
		panic(err)
		return err
	}
	if _, err := f.file.WriteAt(sum[:], f.off+frameSize+int64(len(data))); err != nil {
		panic(err)
		return err
	}
	f.off += frameSize + int64(len(data)) + 32
	return nil
}

func sha2(x, y []byte) [32]byte {
	h := sha256.New()
	h.Write(x)
	h.Write(y)
	return [32]byte(h.Sum(nil))
}

func (t *diskTree) Close() error {
	if t.mem != nil {
		if err := t.span.Release(); err != nil && t.err == nil {
			t.err = err
		}
		t.mem = nil
	}
	return t.err
}

func (t *diskTree) startPatch() {
	t.patch = make([]byte, patchMax)
}

func (t *diskTree) flushPatch() error {
	if err := t.writeFrame(&t.current, t.patch); err != nil {
		return err
	}
	t.patch = t.patch[:0]
	return nil
}

func (t *diskTree) mutate(dst, src []byte) error {
	n := copy(dst, src)
	if n > 255 {
		return fmt.Errorf("mutation too large")
	}
	if t.patch == nil {
		t.startPatch()
	}
	var buf [7]byte
	putAddr(buf[:], t.memToAddr(dst))
	buf[6] = byte(n)
	t.patch = append(t.patch, buf[:]...)
	t.patch = append(t.patch, dst[:n]...)
	if len(t.patch) < patchFlush {
		return nil
	}
	if err := t.flushPatch(); err != nil {
		return err
	}
	// TODO maybe start a compaction
	return nil
}

func (t *diskTree) addrToMem(a addr, n int) ([]byte, error) {
	if a > addr(len(t.mem)) || len(t.mem)-int(a) < n {
		return nil, errCorrupt
	}
	return t.mem[a : a+addr(n)], nil
}

func (t *diskTree) memToAddr(p []byte) addr {
	if !slicemath.Contains(t.mem, p) {
		panic("mpt: memToAddr misuse")
	}
	return addr(slicemath.Offset(t.mem, p))
}

const spanChunk = 64 << 20

func (t *diskTree) alloc(n int) ([]byte, error) {
	if cap(t.mem)-len(t.mem) < n {
		mem, err := t.span.Expand(len(t.mem) + spanChunk)
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

func (h *diskHdr) epoch() int64 { return int64(binary.BigEndian.Uint64(h[hdrEpoch:])) }
func (h *diskHdr) dirty() bool  { return h[hdrDirty] != 0 }
func (h *diskHdr) root() addr   { return parseAddr(h[hdrRoot:]) }
func (h *diskHdr) hash() Hash   { return Hash(h[hdrHash:]) }
func (h *diskHdr) nodes() int   { return int(binary.BigEndian.Uint64(h[hdrNodes:])) }

func (h *diskHdr) setEpoch(t *diskTree, epoch int64) error {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(epoch))
	return t.mutate(h[hdrEpoch:], buf[:])
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

func (n *diskNode) bit() int {
	if n.left() == 0 && n.right() == 0 {
		return -1
	}
	return int(n[nodeUbit])
}

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
