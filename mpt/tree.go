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
	"iter"
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
	// It is an error to call Predict with changes that are not
	// sorted by increasing key or that contain duplicate keys.
	// Implementations may return [ErrInvalidPredict] in this case.
	//
	// It is an error to call Predict if Set has been called without
	// a subsequent call to Snap: in that case, the caller does not
	// know what the current hash is.
	Predict(changes []KeyVal) (Hash, error)

	// Snap sets the tree's version number and returns the current tree snapshot.
	//
	// Snap is a mutating operation and must not be called
	// concurrently with any other Tree method calls
	// (including other calls to Snap).
	//
	// As a special case, if version is negative, Snap does not
	// set the version.
	Snap(version int64) (Snapshot, error)

	// Prove looks up key in the tree and returns a claimed
	// associated value (if any) and whether the key is present at all,
	// along with a proof of those two claimed results.
	// Use [Verify] to verify the proof before trusting the claims.
	//
	// If Prove returns normally (with err == nil), then proof is non-nil,
	// although it may be empty.
	//
	// If Prove returns a non-nil error error, then val is Val{},
	// ok is false, and proof is nil.
	//
	// Prove is a read-only operation and can be called
	// concurrently with other calls to Prove, but not other
	// calls to Set or Snap.
	//
	// It is an error to call Prove if Set has been called without
	// a subsequent call to Snap: in that case, the caller does not
	// know what the root hash is, so the proof will be unverifiable.
	Prove(key Key) (val Val, ok bool, proof Proof, err error)

	// Sync flushes all changes from past Set and Snap calls to
	// the underlying files and then calls the files' Sync methods
	// to flush the changes to disk. (If the files are *os.File files,
	// Sync calls fsync(2).)
	//
	// Sync must not be called concurrently with other calls to Sync
	// or (as noted above) with calls to Set.
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

// ErrInvalidPredict indicates that Predict was passed a set of
// changes that was not sorted by increasing key or contained
// duplicates.
var ErrInvalidPredict = errors.New("invalid predicted changes")

func checkChanges(changes []KeyVal) error {
	for i := 0; i < len(changes)-1; i++ {
		if bytes.Compare(changes[i].Key[:], changes[i+1].Key[:]) >= 0 {
			return ErrInvalidPredict
		}
	}
	return nil
}

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

// Compare returns the result of comparing two keys.
func (k Key) Compare(q Key) int {
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

// KeyVal is a key-value pair.
type KeyVal struct {
	Key Key
	Val Val
}

// Compare returns the result of comparing keys kv.Key and other.Key.
// It ignores the Val fields.
func (kv KeyVal) Compare(other KeyVal) int {
	return kv.Key.Compare(other.Key)
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

// TreeHash computes the snapshot hash of a tree consisting of
// the sequence of key-value items.
//
// The sequence must be sorted by increasing
// key value (such as by [Key.Compare] or [KeyVal.Compare]),
// and a key cannot appear multiple times in the list.
// TreeHash panics if the sequence is not sorted or a key appears twice.
//
// Use [slices.Values] to apply TreeHash to a slice of KeyVal.
func TreeHash(seq iter.Seq[KeyVal]) Hash {
	var s []node
	for kv := range seq {
		s = reduce(append(s, node{prefix(kv.Key, keyBits), hashLeaf(kv.Key, kv.Val)}))
	}
	return hashStack(s)
}

// A Proof is a proof of the result of looking up a target key in a
// specific snapshot of a Tree.
type Proof []byte

// Proof Format
//
// The format of the proof depends on the lookup result being proved.
//
// The proof of an empty tree carries no data; to verify the proof is to check that the
// tree snapshot is the empty tree hash and that the lookup returned Val{}, false.
//
// The proof of a key-value pair being in the tree is the path from that pair up to the tree root.
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

var (
	// ErrInvalidProof indicates that a proof is not valid for the claimed result.
	ErrInvalidProof = errors.New("invalid mpt proof")

	// ErrInvalidLookup indicates that ok is false but val is non-zero.
	ErrInvalidLookup = errors.New("invalid mpt lookup result")
)

// Verify verifies that p is a valid proof that a lookup for key in snap
// should return the result (val, ok).
// If the proof is not valid, Verify returns a non-nil error.
//
// [VerifyPresent] and [VerifyNotPresent] are convenience functions
// that wrap Verify.
func Verify(snap Snapshot, key Key, val Val, ok bool, proof Proof) error {
	//fmt.Printf("Verify %v %x\n", key, proof)
	if !ok && val != (Val{}) {
		return ErrInvalidLookup
	}
	if !ok && len(proof) == 0 {
		if snap.Hash == emptyTreeHash() {
			return nil
		}
		return ErrInvalidProof
	}

	var pkey Key
	if ok {
		pkey = key
	} else {
		if len(proof) < 64 {
			return ErrInvalidProof
		}
		pkey = Key(proof[:32])
		val = Val(proof[32:64])
		proof = proof[64:]
		if pkey == key {
			return ErrInvalidProof
		}
	}

	h := hashLeaf(pkey, val)
	b := 256
	for len(proof) >= 1+32 && int(proof[0]) < b {
		var sib Hash
		b, sib, proof = int(proof[0]), Hash(proof[1:1+32]), proof[1+32:]
		if key.bit(b) != pkey.bit(b) {
			return ErrInvalidProof
		}
		if key.bit(b) == 0 {
			h = hashInner(b, h, sib)
		} else {
			h = hashInner(b, sib, h)
		}
	}
	if len(proof) != 0 || h != snap.Hash {
		return ErrInvalidProof
	}
	return nil
}

// VerifyPresent is shorthand for [Verify](snap, key, val, true, proof).
func VerifyPresent(snap Snapshot, key Key, val Val, proof Proof) error {
	return Verify(snap, key, val, true, proof)
}

// VerifyNotPresent is shorthand for [Verify](snap, key, Val{}, false, proof).
func VerifyNotPresent(snap Snapshot, key Key, proof Proof) error {
	return Verify(snap, key, Val{}, false, proof)
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
