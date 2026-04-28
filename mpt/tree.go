// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package mpt implements a Merkle Patricia Tree.
package mpt

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/bits"
)

// A Tree is a Merkle Patricia Tree implementation.
type Tree interface {
	// Set adds the given key-value pair to the tree.
	// If there is already an entry for the given key,
	// then val replaces the old value.
	//
	// Set is a mutating operation and must not be called
	// concurrently with any other Tree method calls
	// (including other calls to Set).
	Set(key Key, val Val) error

	// Predict returns the hash of the tree that would result from
	// applying the given changes (sorted by key) to the tree,
	// without modifying the tree.
	//
	// It is an error to call Predict if Set has been called without
	// a subsequent call to Snap: in that case, the caller does not
	// know what the current hash is.
	Predict(changes []KeyValue) (Hash, error)

	// Snap sets the tree's version number and returns the current tree snapshot.
	//
	// Snap is a mutating operation and must not be called
	// concurrently with any other Tree method calls
	// (including other calls to Snap).
	//
	// As a special case, if version is negative, Snap does not
	// set the version.
	Snap(version int64) (Snapshot, error)

	// Prove looks up key in the tree and returns a proof
	// either of key's value or that key is not present.
	// Use [Verify] to retrieve the lookup result.
	//
	// Prove is a read-only operation and can be called
	// concurrently with other calls to Prove, but not other
	// calls to Set or Snap.
	//
	// It is an error to call Prove if Set has been called without
	// a subsequent call to Snap: in that case, the caller does not
	// know what the root hash is, so the proof will be unverifiable.
	Prove(key Key) (Proof, error)

	// Sync flushes all changes from past Set and Snap calls to
	// the underlying files and then calls the files' Sync methods
	// to flush the changes to disk. (If the files are *os.File files,
	// Sync calls fsync(2).)
	//
	// Even in the absence of calls to Sync, a Tree provides the
	// guarantee that on recovery from a crash, it can identify the
	// latest snapshot whose Set calls are fully included in the tree.
	// A client can call Version() to find the stored tree's version V
	// and a boolean indicating whether any later Set calls may also
	// be reflected in the tree. When a recovered version is inexact,
	// some Set calls made after that version may be present and
	// others may not be, no matter the order in which the Set calls were made.
	Sync() error

	// Version returns the version number of the tree's last complete snapshot.
	// All Set calls made prior to Snap(version) are guaranteed to be
	// recorded in the tree. However, if exact is false, then the tree may
	// include the effect of Set calls made after that snapshot.
	// In that case, to bring the tree into a consistent state, the client is
	// expected to replay all Set calls up to the next version.
	Version() (version int64, exact bool)

	// Close calls Sync and then closes the underlying files.
	Close() error
}

// ErrModifiedTree indicates that Prove was called after a Set without a Snap.
var ErrModifiedTree = errors.New("tree modified without snapshot")

// A Key is a key used by a Tree.
// It is usually a cryptographic hash of the actual key data.
type Key [32]byte

// keyBits is the number of bits in a Key.
const keyBits = len(Key{}) * 8

func (k Key) String() string {
	return hex.EncodeToString(k[:])
}

// bit returns the n'th bit of the key.
func (k Key) bit(n int) int {
	return (int(k[n>>3]) >> (7 - n&7)) & 1
}

// overlap returns the number of leading bits p and q have in common.
func (p Key) overlap(q Key) int {
	for i := range p {
		pf := p[i]
		qf := q[i]
		if pf != qf {
			return i*8 + bits.LeadingZeros8(pf^qf)

		}
	}
	return 256
}

// compare returns the result of comparing two keys.
func (k Key) compare(q Key) int {
	return bytes.Compare(k[:], q[:])
}

// Value is the old name for Val.
// Run “go fix” to update client code to use Val instead of Value.
//
//go:fix inline
type Value = Val

// A Val is a value stored in a Tree.
// It is usually a cryptographic hash of the actual value data.
type Val [32]byte

func (v Val) String() string {
	return hex.EncodeToString(v[:])
}

// A KeyValue is a key-value pair.
type KeyValue struct {
	Key Key
	Val Val
}

func (kv KeyValue) compare(other KeyValue) int {
	return kv.Key.compare(other.Key)
}

// A keyPrefix is a prefix of a key, identifying a specific node.
type keyPrefix struct {
	// bits is the prefix length in bits (0..256, inclusive).
	bits int

	// full is the key prefix bytes, zero-padded on the right.
	full Key
}

func (p keyPrefix) String() string {
	return fmt.Sprintf("%x/%d", p.full[:(p.bits+7)/8], p.bits)
}

// overlap returns the number of leading bits p and q have in common.
func (p keyPrefix) overlap(q keyPrefix) int {
	return min(p.bits, q.bits, p.full.overlap(q.full))
}

func (p keyPrefix) truncate(bits int) keyPrefix {
	p.bits = bits
	clear(p.full[(bits+7)/8:])
	if n := bits & 7; n != 0 {
		p.full[bits/8] &= 0xFF << (8 - n)
	}
	return p
}

func (p keyPrefix) compare(q keyPrefix) int {
	return bytes.Compare(p.full[:], q.full[:])
}

func prefix(key Key, bits int) keyPrefix {
	p := keyPrefix{bits: bits, full: key}
	return p.truncate(bits)
}

// A node represents the metadata for a single node.
type node struct {
	key  keyPrefix
	hash Hash
}

func (x node) merge(y node) node {
	b := x.key.overlap(y.key)
	return node{x.key.truncate(b), hashInner(b, x.hash, y.hash)}
}

// A Snapshot is a cryptographic snapshot of a Tree at a point in time.
// It is expected that every snapshot is recorded in a transparent log.
//
// The snapshot epoch is a sequence number identifying a specific snapshot.
// An empty Tree has epoch 0, and then the epoch is incremented each
// time a new snapshot is created (by calling [Tree.Snap] after new records
// are added).
//
// The snapshot hash is a cryptographic hash of the entire tree content.
type Snapshot struct {
	Version int64
	Hash    Hash
}

// A Hash is a Merkle hash of a node.
type Hash [32]byte

func (h Hash) String() string {
	return hex.EncodeToString(h[:])
}

// A Proof is a proof of the result of looking up a target key in a
// specific snapshot of a Tree.
type Proof []byte

// Proof Format
//
// Proofs start with "mptproof", followed by a one-byte tag that determines
// the format of the additional data. The tags are:
//
//   - 0: proof of empty tree; no data
//   - 1: proof key is in tree; data is value and path
//   - 2: proof key in not in tree; data is alt key, value, and path
//
// The proof of an empty tree carries no data; to verify the proof is to check that the
// tree snapshot is the empty tree hash.
//
// The proof of a key being in the tree is the key's value followed by the
// path from that key-value pair up to the tree root.
// For each node along the path, the data contains a one-byte overlap count
// (the number of bits shared by the left and right children of the node)
// and the 32-byte hash of the sibling not on the path.
// Verifying the proof requires computing the leaf hash corresponding to key-value
// and then combining that leaf hash with the overlap counts and sibling hashes,
// eventually producing a root tree hash that must match the tree snapshot.
//
// The proof of a key not being in the tree is an alternate key-value pair
// followed by the path from that key-value pair up to the tree root.
// Verifying the proof requires checking that the alt-key is not equal to the
// target key, then recomputing the tree hash from alt-key-value and path.
// During the recomputation, the verifier must check that for every overlap count
// in the path, the target key and the alt-key agree at that bit position,
// verifying that a search for the target would find the alt-key instead.
const (
	proofMagic   = "mptproof"
	proofEmpty   = proofMagic + "\x00"
	proofConfirm = proofMagic + "\x01"
	proofDeny    = proofMagic + "\x02"
)

var (
	// ErrMalformedProof indicates that a proof is not formatted correctly.
	ErrMalformedProof = errors.New("malformed mpt proof")

	// ErrMismatchedProof indicates that a proof does not match
	// the snapshot and key passed to Verify.
	ErrMismatchedProof = errors.New("mismatched mpt proof")
)

// Verify verifies that p is a valid proof of a lookup for key in snap,
// returning the proved lookup result (val, ok).
// If the proof is not valid for key in snap, Verify returns a non-nil error.
func Verify(snap Snapshot, key Key, proof Proof) (val Val, ok bool, err error) {
	//fmt.Printf("Verify %v %x\n", key, proof)
	if string(proof) == proofEmpty {
		if snap.Hash == emptyTreeHash() {
			return Val{}, false, nil
		}
		return Val{}, false, ErrMismatchedProof
	}

	var data []byte
	var pkey Key
	if data, ok = bytes.CutPrefix(proof, []byte(proofConfirm)); ok && len(data) >= 32 {
		pkey = key
		val, data = Val(data[:32]), data[32:]
	} else if data, ok = bytes.CutPrefix(proof, []byte(proofDeny)); ok && len(data) >= 64 {
		pkey, val, data = Key(data[:32]), Val(data[32:64]), data[64:]
		if pkey == key {
			return Val{}, false, ErrMalformedProof
		}
	}
	h := hashLeaf(pkey, val)
	b := 256
	for len(data) >= 1+32 && int(data[0]) < b {
		var sib Hash
		b, sib, data = int(data[0]), Hash(data[1:1+32]), data[1+32:]
		if key.bit(b) != pkey.bit(b) {
			return Val{}, false, ErrMalformedProof
		}
		if key.bit(b) == 0 {
			h = hashInner(b, h, sib)
		} else {
			h = hashInner(b, sib, h)
		}
	}
	if len(data) != 0 || h != snap.Hash {
		return Val{}, false, ErrMalformedProof
	}
	if pkey == key {
		return val, true, nil
	}
	return Val{}, false, nil
}

// emptyTreeHash returns the parent hash for a root no child nodes.
func emptyTreeHash() Hash {
	h := sha256.Sum256(nil)
	//fmt.Printf("hash0() = %x\n", h)
	return h
}

// hashLeaf returns the hash of a leaf with a given key and value.
func hashLeaf(key Key, val Val) Hash {
	var kv [64]byte
	copy(kv[:32], key[:])
	copy(kv[32:64], val[:])
	h := sha256.Sum256(kv[:])
	//fmt.Printf("hashLeaf %v %v -> %x\n", key, val, h[:])
	return h
}

// hashInner returns the hash of an inner node
// with the given bit position and left and right child hashes.
func hashInner(b int, left, right Hash) Hash {
	var enc [65]byte
	copy(enc[:32], left[:])
	copy(enc[32:64], right[:])
	enc[64] = byte(b)
	h := sha256.Sum256(enc[:])
	if right == (Hash{}) {
		panic("zero")
	}
	// fmt.Printf("hashInner %v %v %d -> %x\n", left, right, bits, h[:])
	return h
}

func reduce(s []node) []node {
	for len(s) >= 3 && s[len(s)-3].key.overlap(s[len(s)-2].key) > s[len(s)-2].key.overlap(s[len(s)-1].key) {
		m := s[len(s)-3].merge(s[len(s)-2])
		s = append(s[:len(s)-3], m, s[len(s)-1])
	}
	return s
}

func hashStack(s []node) Hash {
	if len(s) == 0 {
		return emptyTreeHash()
	}
	for len(s) >= 2 {
		s = append(s[:len(s)-2], s[len(s)-2].merge(s[len(s)-1]))
	}
	return s[0].hash
}
