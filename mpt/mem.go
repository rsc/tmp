// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mpt

import (
	"errors"
	"fmt"
)

// A memTree is an in-memory [Tree].
type memTree struct {
	version int64    // version number of tree
	exact   bool     // version is exact
	root    *memNode // root node
	hash    Hash     // overall tree hash
	dirty   bool     // Set called without Snap
	nodes   int      // number of nodes in tree
	err     error    // sticky error condition
}

// A memNode is a single node in the in-memory tree.
type memNode struct {
	key   Key
	val   Val
	ihash Hash
	dirty bool // needs rehashing
	ubit  byte
	left  *memNode
	right *memNode
}

func (n *memNode) bit() int {
	if n.left == nil && n.right == nil {
		return -1
	}
	return int(n.ubit)
}

// NewMemTree returns a new in-memory [Tree].
func NewMemTree() Tree {
	t := &memTree{
		hash:  emptyTreeHash(),
		exact: true,
	}
	return t
}

// hash returns the hash for the given tree node.
// pbit is the parent bit depth, controlling whether n is viewed as a leaf.
func (n *memNode) hash(pbit int) Hash {
	if n.bit() <= pbit {
		return hashLeaf(n.key, n.val)
	}
	return n.ihash
}

// unhash marks n's hash invalid.
func (n *memNode) unhash() {
	n.dirty = true
}

// rehash updates n.hash if needed and then returns it.
func (n *memNode) rehash(pbit int) Hash {
	nbit := n.bit()
	if nbit <= pbit {
		return hashLeaf(n.key, n.val)
	}
	if n.dirty {
		n.ihash = hashInner(nbit, n.left.rehash(nbit), n.right.rehash(nbit))
		n.dirty = false
	}
	return n.ihash
}

// Sync is a no-op since the data is only in memory.
func (t *memTree) Sync() error {
	return nil
}

// Close is a no-op since the data is only in memory.
func (t *memTree) Close() error {
	if t.err != nil {
		return t.err
	}
	t.err = errors.New("tree is closed")
	return nil
}

func (t *memTree) UnsafeUnmap() error { return nil }

// Stat returns the tree metadata.
func (t *memTree) Version() (version int64, exact bool) {
	return t.version, t.exact
}

// Snap returns a snapshot of t.
func (t *memTree) Snap(version int64) (Snapshot, error) {
	if t.err != nil {
		return Snapshot{}, t.err
	}
	if t.dirty {
		// nothing, but keep the read for causing races with Set
	}
	t.dirty = false
	if version >= 0 {
		t.version = version
	}
	if t.root != nil {
		t.hash = t.root.rehash(-1)
		// t.check()
	}
	t.exact = true
	return Snapshot{t.version, t.hash}, nil
}

// Set sets the value associated with key to val.
func (t *memTree) Set(key Key, val Val) error {
	if t.err != nil {
		return t.err
	}
	t.dirty = true
	t.exact = false
	if t.root == nil {
		t.root = &memNode{key: key, val: val}
	} else {
		if setChild(-1, &t.root, key, val) >= 0 {
			panic("bad add")
		}
	}
	// t.check()
	return nil
}

func (n *memNode) set(pbit int, key Key, val Val) int {
	if n.bit() <= pbit {
		// view n as leaf
		b := n.key.overlap(key)
		if b == keyBits {
			n.val = val
			return -1
		}
		// Caller must create a node splitting at bit b.
		return b
	}

	nbit := n.bit()
	ptr := &n.left
	if nbit >= 0 && key.bit(nbit) != 0 {
		ptr = &n.right
	}
	b := setChild(nbit, ptr, key, val)
	if b < 0 {
		n.unhash()
	}
	return b
}

func setChild(nbit int, child **memNode, key Key, val Val) int {
	b := (*child).set(nbit, key, val)
	if nbit < b {
		n := new(memNode)
		var left, right *memNode
		if key.bit(b) == 0 {
			left, right = n, *child
		} else {
			left, right = *child, n
		}
		*n = memNode{
			key:   key,
			val:   val,
			ubit:  uint8(b),
			dirty: true,
			left:  left,
			right: right,
		}
		*child = n
		b = -1
	}
	return b
}

// Predict returns the hash of the tree that would result from
// applying the given changes (sorted by key) to the tree,
// without modifying the tree.
func (t *memTree) Predict(changes []KeyValue) (Hash, error) {
	if t.err != nil {
		return Hash{}, t.err
	}
	if t.dirty {
		return Hash{}, ErrModifiedTree
	}

	s, list := t.predict([]node{}, t.root, -1, changes)
	for _, kv := range list {
		s = reduce(append(s, node{prefix(kv.Key, 256), hashLeaf(kv.Key, kv.Val)}))
	}
	return hashStack(s), nil
}

func (t *memTree) predict(s []node, n *memNode, pbit int, list []KeyValue) ([]node, []KeyValue) {
	if n == nil {
		return s, list
	}
	key, val := n.key, n.val
	nbit := n.bit()
	bits := nbit
	if nbit <= pbit {
		bits = 256
	}
	pkey := prefix(key, bits)

	// Stack modifications before node.
	for len(list) > 0 && prefix(list[0].Key, bits).compare(pkey) < 0 {
		k, v := list[0].Key, list[0].Val
		list = list[1:]
		s = reduce(append(s, node{prefix(k, 256), hashLeaf(k, v)}))
	}

	// Stack leaf node, possibly replaced.
	if bits == 256 {
		if len(list) > 0 && list[0].Key == key {
			val = list[0].Val
			list = list[1:]
		}
		s = reduce(append(s, node{pkey, hashLeaf(key, val)}))
		return s, list
	}

	// Stack entire subtree, if no modifications inside it.
	if len(list) == 0 || pkey.compare(prefix(list[0].Key, bits)) < 0 {
		h := n.hash(pbit)
		s = reduce(append(s, node{pkey, h}))
		return s, list
	}

	// Otherwise, apply modifications within subtree.
	s, list = t.predict(s, n.left, nbit, list)
	s, list = t.predict(s, n.right, nbit, list)
	return s, list
}

// Prove returns a proof of the presence or absence of key in t.
func (t *memTree) Prove(key Key) (Proof, error) {
	if t.err != nil {
		return nil, t.err
	}
	if t.dirty {
		return nil, ErrModifiedTree
	}
	if t.root == nil {
		return Proof(proofEmpty), nil
	}
	return t.root.prove(-1, key), nil
}

func (n *memNode) prove(pbit int, key Key) Proof {
	nbit := n.bit()
	if nbit <= pbit {
		// view n as leaf
		var p Proof
		if n.key == key {
			p = Proof(proofConfirm)
		} else {
			p = append(Proof(proofDeny), n.key[:]...)
		}
		return append(p, n.val[:]...)
	}

	var sib Hash
	var child *memNode
	if key.bit(nbit) == 0 {
		child = n.left
		sib = n.right.hash(nbit)
	} else {
		child = n.right
		sib = n.left.hash(nbit)
	}
	return append(append(child.prove(nbit, key), byte(nbit)), sib[:]...)
}

// check checks all the tree invariants, walking the entire tree.
// It is too slow for real use but helpful to insert when debugging.
func (t *memTree) check() {
	println("check")
	h := t.root.check(1, -1)
	if h != t.hash && (t.root == nil || !t.dirty) {
		fmt.Printf("have %v want %v\n", t.hash, h)
		panic("bad hash")
	}
}

func (n *memNode) check(depth, pbit int) Hash {
	nbit := n.bit()
	if nbit <= pbit {
		// view as leaf
		fmt.Printf("%*s%d leaf %v %v %p %p %p %v\n", depth*2, "", n.bit(), n.key, n.val, n, n.left, n.right, hashLeaf(n.key, n.val))
		return hashLeaf(n.key, n.val)
	}
	fmt.Printf("%*s%d %p %p %p %v\n", depth*2, "", n.bit(), n, n.left, n.right, n.ihash)
	h := hashInner(nbit, n.left.check(depth+1, nbit), n.right.check(depth+1, nbit))
	if h != n.ihash && !n.dirty {
		fmt.Printf("%*shave %v want %v\n", depth*2, "", n.ihash, h)
		panic("bad hash")
	}
	return h
}
