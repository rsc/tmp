// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mpt

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
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
)

// A diskTree is an on-disk [Tree].
type diskTree struct {
	id    [16]byte
	seq   uint64
	span  *span.Span
	mem   []byte
	patch []byte // pending patch
	err   error
}

// A diskSeg is a single segment of the on-disk tree.
type diskSeg struct {
	mem  []byte
	base addr // disk address of mem[0]
}

// NewDiskTree returns a new on-disk [Tree].
func NewDiskTree() Tree {
	sp, err := span.Reserve(1 << 44)
	if err != nil {
		panic("span: " + err.Error()) // TODO
	}
	mem, err := sp.Expand(hdrSize)
	if err != nil {
		panic("span") // TODO
	}
	t := &diskTree{
		span: sp,
		mem:  mem,
	}
	t.hdr().setHash(t, emptyTreeHash())

	runtime.AddCleanup(t, func(*struct{}) { sp.Release() }, nil)
	return t
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
	t.patch = make([]byte, frameSize, patchMax)
	copy(t.patch[frameID:], t.id[:])
	binary.BigEndian.PutUint64(t.patch[frameSeq:], t.seq)
}

func (t *diskTree) flushPatch() {
	binary.BigEndian.PutUint64(t.patch[frameLen:], uint64(len(t.patch)-frameSize))
	sum := sha256.Sum256(t.patch)
	t.patch = append(t.patch, sum[:]...)
	// TODO write patch somewhere
	t.patch = t.patch[:frameSize]
}

func (t *diskTree) mutate(dst, src []byte) {
	n := copy(dst, src)
	if n > 255 {
		panic("mutation too large")
	}
	if t.patch == nil {
		t.startPatch()
	}
	var buf [7]byte
	putAddr(buf[:], t.memToAddr(dst))
	buf[6] = byte(n)
	t.patch = append(t.patch, buf[:]...)
	t.patch = append(t.patch, dst[:n]...)
	if len(t.patch) > patchFlush {
		t.flushPatch()
	}
}

func (t *diskTree) addrToMem(a addr, n int) []byte {
	if a > addr(len(t.mem)) || len(t.mem)-int(a) < n {
		panic("invalid address") // TODO might be corruption
	}
	return t.mem[a : a+addr(n)]
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

func (h *diskHdr) setEpoch(t *diskTree, epoch int64) {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(epoch))
	t.mutate(h[hdrEpoch:], buf[:])
}

func (h *diskHdr) setDirty(t *diskTree, d bool) {
	var buf [1]byte
	if d {
		buf[0] = 1
	}
	t.mutate(h[hdrDirty:], buf[:])
}

func (h *diskHdr) setRoot(t *diskTree, n *diskNode) {
	a := t.addr(n)
	var buf [6]byte
	putAddr(buf[:], a)
	t.mutate(h[hdrRoot:], buf[:])
}

func (h *diskHdr) setHash(t *diskTree, hash Hash) {
	t.mutate(h[hdrHash:], hash[:])
}

func (h *diskHdr) setNodes(t *diskTree, n int) {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(n))
	t.mutate(h[hdrNodes:], buf[:])
}

func (t *diskTree) hdr() *diskHdr {
	return (*diskHdr)(t.addrToMem(0, hdrSize))
}

// A diskNode is the memory copy of a node.
// The *diskNodes passed around in this implementation
// are pointers into the in-memory copy t.mem.
type diskNode [nodeSize]byte

func (t *diskTree) node(a addr) *diskNode {
	if a == 0 {
		return nil
	}
	return (*diskNode)(t.addrToMem(a, nodeSize))
}

func (t *diskTree) addr(n *diskNode) addr {
	if n == nil {
		return 0
	}
	return t.memToAddr(n[:])
}

// addrAt reads a node address from the address a.
func (t *diskTree) addrAt(a addr) addr {
	return parseAddr(t.addrToMem(a, addrSize))
}

// setAddrAt writes the node address b to the address a.
func (t *diskTree) setAddrAt(a, b addr) {
	var buf [addrSize]byte
	putAddr(buf[:], b)
	t.mutate(t.addrToMem(a, addrSize), buf[:])
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

func (n *diskNode) init(t *diskTree, key Key, val Value, bit int, left, right *diskNode) {
	var buf [nodeSize]byte
	copy(buf[nodeKey:], key[:])
	copy(buf[nodeVal:], val[:])
	buf[nodeUbit] = byte(bit)
	buf[nodeDirty] = 1
	putAddr(buf[nodeLeft:], t.addr(left))
	putAddr(buf[nodeRight:], t.addr(right))
	t.mutate(n[:], buf[:])
}

func (n *diskNode) setVal(t *diskTree, val Value) { t.mutate(n[nodeVal:], val[:]) }
func (n *diskNode) setIHash(t *diskTree, h Hash)  { t.mutate(n[nodeIHash:], h[:]) }
func (n *diskNode) setDirty(t *diskTree, d bool) {
	var p [1]byte
	if d {
		p[0] = 1
	}
	t.mutate(n[nodeDirty:], p[:])
}

// hash returns the hash for the given tree node.
// pbit is the parent bit depth, controlling whether n is viewed as a leaf.
func (n *diskNode) hash(pbit int) Hash {
	if n.bit() <= pbit {
		return hashLeaf(n.key(), n.val())
	}
	return n.ihash()
}

// unhash marks n's hash invalid.
func (n *diskNode) unhash(t *diskTree) {
	if !n.dirty() {
		n.setDirty(t, true)
	}
}

// rehash updates n.hash if needed and then returns it.
func (n *diskNode) rehash(t *diskTree, pbit int) Hash {
	nbit := n.bit()
	if nbit <= pbit {
		return hashLeaf(n.key(), n.val())
	}
	if n.dirty() {
		n.setIHash(t,
			hashInner(nbit,
				t.node(n.left()).rehash(t, nbit),
				t.node(n.right()).rehash(t, nbit)))
		n.setDirty(t, false)
	}
	return n.ihash()
}

// Snap returns a snapshot of t.
func (t *diskTree) Snap() (Snapshot, error) {
	if t.err != nil {
		return Snapshot{}, t.err
	}
	if t.hdr().dirty() {
		t.hdr().setEpoch(t, t.hdr().epoch()+1)
		t.hdr().setDirty(t, false)

		root := t.node(t.hdr().root())
		t.hdr().setHash(t, root.rehash(t, -1))
		// t.check()
	}
	return Snapshot{t.hdr().epoch(), t.hdr().hash()}, nil
}

// Set sets the value associated with key to val.
func (t *diskTree) Set(key Key, val Value) error {
	if t.err != nil {
		return t.err
	}
	if !t.hdr().dirty() {
		t.hdr().setDirty(t, true)
	}
	if t.hdr().root() == 0 {
		n, err := t.newNode()
		if err != nil {
			return err
		}
		n.init(t, key, val, 0, nil, nil)
		t.hdr().setRoot(t, n)
	} else {
		b, err := t.setChild(-1, hdrRoot, key, val)
		if err != nil {
			return err
		}
		if b >= 0 {
			panic("bad add")
		}
		t.node(t.hdr().root()).unhash(t)
	}
	// t.check()
	return nil
}

func (n *diskNode) set(t *diskTree, pbit int, key Key, val Value) (int, error) {
	nbit := n.bit()
	if nbit <= pbit {
		// view n as leaf
		b := n.key().overlap(key)
		if b == keyBits {
			n.setVal(t, val)
			return -1, nil
		}
		// Caller must create a node splitting at bit b.
		return b, nil
	}

	ptr := t.addr(n) + nodeLeft
	if nbit >= 0 && key.bit(nbit) != 0 {
		ptr = t.addr(n) + nodeRight
	}
	b, err := t.setChild(nbit, ptr, key, val)
	if err != nil {
		return 0, err
	}
	if b < 0 {
		n.unhash(t)
	}
	return b, nil
}

func (t *diskTree) setChild(nbit int, childp addr, key Key, val Value) (int, error) {
	child := t.node(t.addrAt(childp))
	b, err := child.set(t, nbit, key, val)
	if err != nil {
		return 0, err
	}
	if nbit < b {
		n, err := t.newNode()
		if err != nil {
			return 0, err
		}
		var left, right *diskNode
		if key.bit(b) == 0 {
			left, right = n, child
		} else {
			left, right = child, n
		}
		n.init(t, key, val, b, left, right)
		t.setAddrAt(childp, t.addr(n))
		b = -1
	}
	return b, nil
}

// Prove returns a proof of the presence or absence of key in t.
func (t *diskTree) Prove(key Key) (Proof, error) {
	if t.err != nil {
		return nil, t.err
	}
	if t.hdr().dirty() {
		return nil, ErrModifiedTree
	}
	root := t.node(t.hdr().root())
	if root == nil {
		return Proof(proofEmpty), nil
	}
	return root.prove(t, -1, key), nil
}

func (n *diskNode) prove(t *diskTree, pbit int, key Key) Proof {
	nbit := n.bit()
	if nbit <= pbit {
		// view n as leaf
		var p Proof
		nkey := n.key()
		if nkey == key {
			p = Proof(proofConfirm)
		} else {
			p = append(Proof(proofDeny), nkey[:]...)
		}
		nval := n.val()
		return append(p, nval[:]...)
	}

	var sib Hash
	var child *diskNode
	if key.bit(nbit) == 0 {
		child = t.node(n.left())
		sib = t.node(n.right()).hash(nbit)
	} else {
		child = t.node(n.right())
		sib = t.node(n.left()).hash(nbit)
	}
	return append(append(child.prove(t, nbit, key), byte(nbit)), sib[:]...)
}

func (t *diskTree) check() {
	println("check")
	if t.node(t.hdr().root()) == nil {
		return
	}
	var sawNil bool
	h := t.node(t.hdr().root()).check(t, 1, -1, &sawNil)
	if h != t.hdr().hash() && !t.hdr().dirty() {
		fmt.Printf("have %v want %v\n", t.hdr().hash(), h)
		panic("bad hash")
	}
	if !sawNil {
		panic("lost nil")
	}
	println("check OK")
}

func (n *diskNode) check(t *diskTree, depth, pbit int, sawNil *bool) Hash {
	if n.bit() == -1 {
		if *sawNil {
			panic("multiple nils")
		}
		*sawNil = true
	}
	if n.bit() <= pbit {
		// view as leaf
		fmt.Printf("%*sleaf(%d) %#x %v %v %#x %#x %v dirty=%v\n", depth*2, "", n.bit(), t.addr(n), n.key(), n.val(), n.left(), n.right(), hashLeaf(n.key(), n.val()), n.dirty())
		return hashLeaf(n.key(), n.val())
	}
	fmt.Printf("%*s%d %#x %#x %#x %v dirty=%v\n", depth*2, "", n.bit(), t.addr(n), n.left(), n.right(), n.ihash(), n.dirty())
	h := hashInner(n.bit(),
		t.node(n.left()).check(t, depth+1, n.bit(), sawNil),
		t.node(n.right()).check(t, depth+1, n.bit(), sawNil))
	if h != n.ihash() && !n.dirty() {
		fmt.Printf("%*shave %v want %v\n", depth*2, "", n.ihash(), h)
		panic("bad hash")
	}
	return h
}
