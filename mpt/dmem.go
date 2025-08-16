// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mpt

import "fmt"

// hash returns the hash for the given tree node.
// pbit is the parent bit depth, controlling whether n is viewed as a leaf.
func (n *diskNode) hash(pbit int) Hash {
	if n.bit() <= pbit {
		return hashLeaf(n.key(), n.val())
	}
	return n.ihash()
}

// unhash marks n's hash invalid.
func (n *diskNode) unhash(t *diskTree) error {
	if n.dirty() {
		return nil
	}
	return n.setDirty(t, true)
}

// rehash updates n.hash if needed and then returns it.
func (n *diskNode) rehash(t *diskTree, pbit int) (Hash, error) {
	nbit := n.bit()
	if nbit <= pbit {
		return hashLeaf(n.key(), n.val()), nil
	}
	if n.dirty() {
		left, err := t.node(n.left())
		if err != nil {
			return Hash{}, err
		}
		lhash, err := left.rehash(t, nbit)
		if err != nil {
			return Hash{}, err
		}
		right, err := t.node(n.right())
		if err != nil {
			return Hash{}, err
		}
		rhash, err := right.rehash(t, nbit)
		if err != nil {
			return Hash{}, err
		}
		if err := n.setIHash(t, hashInner(nbit, lhash, rhash)); err != nil {
			return Hash{}, err
		}
		if err := n.setDirty(t, false); err != nil {
			return Hash{}, err
		}
	}
	return n.ihash(), nil
}

// Snap returns a snapshot of t.
func (t *diskTree) Snap() (Snapshot, error) {
	if err := t.snap(); err != nil {
		return Snapshot{}, err
	}
	// t.check()
	return Snapshot{t.hdr().epoch(), t.hdr().hash()}, nil
}

func (t *diskTree) snap() error {
	if t.err != nil {
		return t.err
	}
	if !t.hdr().dirty() {
		return nil
	}

	if err := t.hdr().setEpoch(t, t.hdr().epoch()+1); err != nil {
		return err
	}
	if err := t.hdr().setDirty(t, false); err != nil {
		return err
	}
	root, err := t.node(t.hdr().root())
	if err != nil {
		return err
	}
	hash, err := root.rehash(t, -1)
	if err != nil {
		return err
	}
	if err := t.hdr().setHash(t, hash); err != nil {
		return err
	}
	return nil
}

// Set sets the value associated with key to val.
func (t *diskTree) Set(key Key, val Value) error {
	if t.err != nil {
		return t.err
	}
	if !t.hdr().dirty() {
		if err := t.hdr().setDirty(t, true); err != nil {
			return err
		}
	}
	if t.hdr().root() == 0 {
		n, err := t.newNode()
		if err != nil {
			return err
		}
		n.init(t, key, val, 0, nil, nil)
		if err := t.hdr().setRoot(t, n); err != nil {
			return err
		}
	} else {
		b, err := t.setChild(-1, hdrRoot, key, val)
		if err != nil {
			return err
		}
		if b >= 0 {
			panic("bad add")
		}
		root, err := t.node(t.hdr().root())
		if err != nil {
			return err
		}
		if err := root.unhash(t); err != nil {
			return err
		}
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
			if err := n.setVal(t, val); err != nil {
				return 0, err
			}
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
		if err := n.unhash(t); err != nil {
			return 0, err
		}
	}
	return b, nil
}

func (t *diskTree) setChild(nbit int, childp addr, key Key, val Value) (int, error) {
	child, err := t.node(t.addrAt(childp))
	if err != nil {
		return 0, err
	}
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
		if err := t.setAddrAt(childp, t.addr(n)); err != nil {
			return 0, err
		}
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
	root, err := t.node(t.hdr().root())
	if err != nil {
		return nil, err
	}
	if root == nil {
		return Proof(proofEmpty), nil
	}
	return root.prove(t, -1, key)
}

func (n *diskNode) prove(t *diskTree, pbit int, key Key) (Proof, error) {
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
		return append(p, nval[:]...), nil
	}

	childAddr, sibAddr := n.left(), n.right()
	if key.bit(nbit) == 1 {
		childAddr, sibAddr = sibAddr, childAddr
	}
	child, err := t.node(childAddr)
	if err != nil {
		return nil, err
	}
	sib, err := t.node(sibAddr)
	if err != nil {
		return nil, err
	}
	sibHash := sib.hash(nbit)

	p, err := child.prove(t, nbit, key)
	if err != nil {
		return nil, err
	}
	return append(append(p, byte(nbit)), sibHash[:]...), nil
}

func (t *diskTree) check() {
	println("check")
	root, err := t.node(t.hdr().root())
	if err != nil {
		panic(err)
	}
	if root == nil {
		return
	}
	var sawNil bool
	h := root.check(t, 1, -1, &sawNil)
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

	left, err := t.node(n.left())
	if err != nil {
		panic(err)
	}
	right, err := t.node(n.right())
	if err != nil {
		panic(err)
	}
	h := hashInner(n.bit(),
		left.check(t, depth+1, n.bit(), sawNil),
		right.check(t, depth+1, n.bit(), sawNil))
	if h != n.ihash() && !n.dirty() {
		fmt.Printf("%*shave %v want %v\n", depth*2, "", n.ihash(), h)
		panic("bad hash")
	}
	return h
}
