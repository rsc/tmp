// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mpt

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
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
)

var (
	// maximum patch and tree length.
	// Variables so that testing can override.
	maxPatch int = 1 << 20
	maxTree  int = 16 << 40
)

// File is the interface needed for on-disk storage.
type File interface {
	io.ReaderAt
	io.WriterAt
	io.Closer
	Sync() error
}

// A diskTree is an on-disk [Tree].
type diskTree struct {
	id      [16]byte
	current diskWriter
	next    diskWriter
	span    *span.Span
	mem     []byte
	mut     addr
	patch   []byte // pending patch
	err     error
}

// A diskReader is an input file.
type diskReader struct {
	file io.ReaderAt
	seq  uint64 // tree sequence number
	off  int64  // read offset in file
}

// A diskWriter is an output file.
type diskWriter struct {
	file File
	seq  uint64 // tree sequence number
	off  int64  // write offset in file
}

// Create creates a new, empty on-disk [Tree] stored in the two named files.
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

	return diskOpen(f1, f2, op)
}

func New(file1, file2 File) (Tree, error) {
	return diskOpen(file1, file2, "new")
}

func diskOpen(file1, file2 File, op string) (Tree, error) {
	sp, err := span.Reserve(maxTree)
	if err != nil {
		return nil, err
	}
	t := &diskTree{span: sp}
	runtime.AddCleanup(t, func(*struct{}) { sp.Release() }, nil)

	if op == "new" {
		var buf [1]byte
		n1, err1 := file1.ReadAt(buf[:], 0)
		n2, err2 := file2.ReadAt(buf[:], 0)
		if n1 == 0 && n2 == 0 && err1 == io.EOF && err2 == io.EOF {
			op = "create"
		} else {
			op = "open"
		}
	}

	if op == "create" {
		mem, err := sp.Expand(hdrSize)
		if err != nil {
			return nil, err
		}
		t.mem = mem
		*(*Hash)(mem[hdrHash:]) = emptyTreeHash()
		rand.Read(t.id[:])
		t.current.file = file1
		t.current.seq = 1
		t.next.file = file2
		if err := t.writeTree(&t.current); err != nil {
			return nil, err
		}
		if err := t.current.file.Sync(); err != nil {
			return nil, err
		}
		if err := t.writeTree(&t.next); err != nil {
			return nil, err
		}
		if err := t.next.file.Sync(); err != nil {
			return nil, err
		}
		return t, nil
	}

	id1, seq1, n1, err := readStart(file1)
	if err != nil {
		return nil, err
	}
	id2, seq2, n2, err := readStart(file2)
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
	if seq1 < seq2 {
		file1, file2, seq1, seq2, n1, n2 = file2, file1, seq2, seq1, n2, n1
	}

	r := &diskReader{file: file1, seq: seq1}
	if err := t.readTree(r, n1); err != nil {
		return nil, err
	}
	t.current.file = file1
	t.current.off = r.off
	t.next.file = file2

	// TODO maybe start a compaction
	// for a compaction:
	//	t.flushPatch
	//	mem := t.mem
	//	t.next.off = frameSize+len(mem)+32
	//	t.next.seq = t.seq+1
	//	go func() {
	//		writeTree(t.next.file, mem)
	//		t.flushPatch()
	//		t.next.sync()
	//		t.next.writeSeq(t.next.seq)
	//		t.next.sync()
	//		t.current, t.next = t.next, t.current
	//		t.next.off = -1
	//		t.next.writeSeq(0)
	//		t.next.sync()
	//	}()

	return t, nil
}

func (t *diskTree) readTree(r *diskReader, treeLen int) error {
	mem, err := t.span.Expand(treeLen)
	if err != nil {
		return err
	}
	var buf [len(magic)]byte
	if _, err := r.file.ReadAt(buf[:], 0); err != nil {
		return err
	}
	if string(buf[:]) != magic {
		return corrupt()
	}
	r.off = int64(len(magic))
	n, err := t.readFrame(r, mem)
	if err != nil {
		return err
	}
	if n != treeLen {
		return corrupt()
	}
	t.mem = mem

	patch := make([]byte, maxPatch)
	for {
		n, err := t.readFrame(r, patch)
		if err == errCorrupt || err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
		if err != nil {
			return err
		}
		if err := t.replay(patch[:n]); err != nil {
			return err
		}
	}

	return nil
}

const magic = "mptTree\n"

func readStart(r io.ReaderAt) (id [16]byte, seq uint64, n int, err error) {
	var buf [len(magic) + frameSize]byte
	if _, err = r.ReadAt(buf[:], 0); err != nil {
		return
	}
	if string(buf[:len(magic)]) != magic {
		err = fmt.Errorf("not a tree file")
		return
	}
	copy(id[:], buf[len(magic)+frameID:])
	seq = binary.BigEndian.Uint64(buf[len(magic)+frameSeq:])
	u := binary.BigEndian.Uint64(buf[len(magic)+frameLen:])
	if uint64(int(u)) != u || int(u) < 0 || u > uint64(maxTree) {
		err = fmt.Errorf("invalid length %#x", u)
	}
	n = int(u)
	return
}

var errCorrupt = errors.New("corrupt data")

func corrupt() error {
	//	println(string(debug.Stack()))
	return errCorrupt
}

func (t *diskTree) readFrame(r *diskReader, data []byte) (int, error) {
	var frame [frameSize]byte
	if _, err := r.file.ReadAt(frame[:], r.off); err != nil {
		return 0, err
	}
	r.off += frameSize
	if [16]byte(frame[frameID:]) != t.id {
		return 0, corrupt()
	}
	if binary.BigEndian.Uint64(frame[frameSeq:]) != r.seq {
		return 0, corrupt()
	}
	n := binary.BigEndian.Uint64(frame[frameLen:])
	if n > uint64(len(data)) {
		return 0, corrupt()
	}

	if _, err := r.file.ReadAt(data[:n], r.off); err != nil {
		return 0, err
	}
	r.off += int64(n)

	var fsum [32]byte
	if _, err := r.file.ReadAt(fsum[:], r.off); err != nil {
		return 0, err
	}
	r.off += int64(len(fsum))
	sum := sha2(frame[:], data[:n])
	if sum != fsum {
		return 0, corrupt()
	}

	return int(n), nil
}

func (t *diskTree) writeTree(w *diskWriter) error {
	if _, err := w.file.WriteAt([]byte(magic), 0); err != nil {
		return err
	}
	w.off = int64(len(magic))
	return t.writeFrame(w, t.mem)
}

func (t *diskTree) writeFrame(w *diskWriter, data []byte) error {
	const writeChunk = 1 << 20
	var frame [frameSize]byte
	copy(frame[frameID:], t.id[:])
	binary.BigEndian.PutUint64(frame[frameSeq:], w.seq)
	binary.BigEndian.PutUint64(frame[frameLen:], uint64(len(data)))

	h := sha256.New()
	h.Write(frame[:])
	if w.file == nil {
		panic("nil w.file")
	}
	if _, err := w.file.WriteAt(frame[:], w.off); err != nil {
		panic(err)
		return err
	}
	w.off += frameSize

	buf := make([]byte, writeChunk)
	for len(data) > 0 {
		n := copy(buf, data)
		data = data[n:]
		h.Write(buf[:n])
		if _, err := w.file.WriteAt(buf[:n], w.off); err != nil {
			panic(err)
			return err
		}
		w.off += int64(n)
	}

	sum := h.Sum(nil)
	if _, err := w.file.WriteAt(sum, w.off); err != nil {
		panic(err)
		return err
	}
	w.off += int64(len(sum))
	return nil
}

func sha2(x, y []byte) [32]byte {
	h := sha256.New()
	h.Write(x)
	h.Write(y)
	return [32]byte(h.Sum(nil))
}

func (t *diskTree) startPatch() {
	t.patch = make([]byte, 0, maxPatch)
}

func (t *diskTree) flushPatch() error {
	if err := t.writeFrame(&t.current, t.patch); err != nil {
		return err
	}
	t.patch = t.patch[:0]
	return nil
}

func (t *diskTree) Sync() error {
	if t.err != nil {
		return t.err
	}
	if err := t.flushPatch(); err != nil {
		return err
	}
	if err := t.current.file.Sync(); err != nil {
		t.err = err
		return err
	}
	return nil
}

func (t *diskTree) Close() error {
	if err := t.Sync(); err != nil {
		return err
	}
	if t.mem != nil {
		if err := t.span.Release(); err != nil && t.err == nil {
			t.err = err
		}
		t.mem = nil
	}
	if err := t.current.file.Close(); err != nil && t.err == nil {
		t.err = err
	}
	if err := t.next.file.Close(); err != nil && t.err == nil {
		t.err = err
	}
	if t.err != nil {
		return t.err
	}
	t.err = errors.New("tree is closed")
	return nil
}

func (t *diskTree) memHash() string {
	h := sha256.Sum256(t.mem[:t.mut])
	s := base64.StdEncoding.EncodeToString(h[:])
	return s[:7]
}

func (t *diskTree) mutate(dst, src []byte) error {
	if len(src) > 255 || len(src) > len(dst) {
		return fmt.Errorf("mutation too large")
	}
	if bytes.Equal(dst, src) {
		return nil
	}
	if t.patch == nil {
		t.startPatch()
	}
	var buf [7]byte
	a := t.memToAddr(dst)
	putAddr(buf[:], a)
	buf[6] = byte(len(src))
	if cap(t.patch)-len(t.patch) < len(buf)+len(src) {
		if err := t.flushPatch(); err != nil {
			return err
		}
	}
	t.patch = append(t.patch, buf[:]...)
	t.patch = append(t.patch, src...)
	// h := t.memHash()
	t.mut = max(t.mut, a+addr(len(src)))
	copy(dst, src)
	// println("mut", h, t.memHash(), "\n\n", string(hex.Dump(t.mem)))

	// TODO maybe start a compaction

	return nil
}

func (t *diskTree) replay(patch []byte) error {
	for len(patch) > 0 {
		if len(patch) < 7 {
			return corrupt()
		}
		a := parseAddr(patch)
		n := int(patch[6])
		patch = patch[7:]
		if n == 0 || n > len(patch) {
			return corrupt()
		}
		if a+addr(n) > addr(len(t.mem)) {
			if a+addr(n) > addr(len(t.mem))+1<<20 {
				return corrupt()
			}
			mem, err := t.span.Expand(int(a + addr(n)))
			if err != nil {
				return err
			}
			t.mem = mem
		}
		copy(t.mem[a:], patch[:n])
		t.mut = max(t.mut, a+addr(n))
		patch = patch[n:]
	}
	return nil
}

func (t *diskTree) addrToMem(a addr, n int) ([]byte, error) {
	if a > addr(len(t.mem)) || len(t.mem)-int(a) < n {
		return nil, corrupt()
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
