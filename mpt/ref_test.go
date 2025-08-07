// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mpt

import (
	"fmt"
)

// Reference hash implementation for testing.

// A node is the metdata for a single node: a key prefix and a hash.
type node struct {
	key  keyPrefix
	hash Hash
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

func rootHash(keys []Key, vals []Value) Hash {
	var stk []node
	for i, k := range keys {
		stk = append(stk, node{key: keyPrefix{keyBits, k}, hash: hashLeaf(k, vals[i])})
		for len(stk) >= 3 && needMerge(stk[len(stk)-3], stk[len(stk)-2], stk[len(stk)-1]) {
			stk = append(stk[:len(stk)-3], merge(stk[len(stk)-3], stk[len(stk)-2]), stk[len(stk)-1])
		}
	}
	for len(stk) >= 2 {
		stk = append(stk[:len(stk)-2], merge(stk[len(stk)-2], stk[len(stk)-1]))
	}
	if len(stk) == 0 {
		return sha()
	}
	return stk[0].hash
}

// needMerge reports whether x and y should be merged given that they are followed by z.
// This is true when y shares a longer prefix with x than with z,
// meaning x and y both have the form "<prefix>0..." while z has the form "<prefix>1...",
// so there must be an interior node for <prefix>, and x and y will be a merged child
// of that node.
func needMerge(x, y, z node) bool {
	return x.key.overlap(y.key) > y.key.overlap(z.key)
}

func merge(x, y node) node {
	b := x.key.overlap(y.key)
	return node{x.key.truncate(b), hashInner(b, x.hash, y.hash)}
}
