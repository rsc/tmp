// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mapbench

import (
	"fmt"
	"math/bits"
	"sync"
	"testing"
	"unsafe"

	"golang.org/x/sys/cpu"
)

type Call struct{ x [200]byte }

type Balancer struct{ y [100]byte }

func BenchmarkMap(b *testing.B) {
	for _, impl := range []string{"MutexMap", "SyncMap", "ShardMap"} {
		for _, call := range []string{"one", "many"} {
			for _, ballast := range []int{0, 10, 100, 1000} {
				var m callMap
				switch impl {
				case "MutexMap":
					m = newMutexMap[Call, *Balancer]()
				case "SyncMap":
					m = new(syncMap[Call, *Balancer])
				case "ShardMap":
					m = newShardMap[Call, *Balancer]()
				}
				b.Run(fmt.Sprintf("impl=%s/call=%s/ballast=%d", impl, call, ballast), benchMutex(m, call == "many", ballast))
			}
		}
	}
}

type callMap interface {
	Store(*Call, *Balancer)
	LoadAndDelete(*Call) (*Balancer, bool)
}

func benchMutex(impl callMap, manyCall bool, ballast int) func(*testing.B) {
	bb := new(Balancer)
	for i := 0; i < ballast; i++ {
		impl.Store(new(Call), bb)
	}

	return func(b *testing.B) {
		b.SetParallelism(100)
		b.ReportAllocs()

		b.RunParallel(func(pb *testing.PB) {
			var call *Call
			if !manyCall {
				call = new(Call)
			}
			for pb.Next() {
				if manyCall {
					call = new(Call)
				}
				impl.Store(call, bb)

				_, ok := impl.LoadAndDelete(call)
				if !ok {
					b.Fatal("key not found")
				}
			}
		})
	}
}

type mutexMap[K, V any] struct {
	mu sync.Mutex
	m  map[*K]V
}

func newMutexMap[K, V any]() *mutexMap[K, V] {
	return &mutexMap[K, V]{m: make(map[*K]V)}
}

func (m *mutexMap[K, V]) Store(k *K, v V) {
	m.mu.Lock()
	m.m[k] = v
	m.mu.Unlock()
}

func (m *mutexMap[K, V]) LoadAndDelete(k *K) (v V, ok bool) {
	m.mu.Lock()
	v, ok = m.m[k]
	if ok {
		delete(m.m, k)
	}
	m.mu.Unlock()
	return v, ok
}

type syncMap[K, V any] struct {
	m sync.Map
}

func (m *syncMap[K, V]) Store(k *K, v V) {
	m.m.Store(k, v)
}

func (m *syncMap[K, V]) LoadAndDelete(k *K) (v V, ok bool) {
	vv, ok := m.m.LoadAndDelete(k)
	v, _ = vv.(V)
	return v, ok
}

const shardBits = 4

type shardMap[K, V any] struct {
	shards [1 << shardBits]shard[K, V]
}

type shard[K, V any] struct {
	sync.Mutex
	m map[*K]V
	_ cpu.CacheLinePad // prevent false sharing
}

func newShardMap[K, V any]() *shardMap[K, V] {
	m := &shardMap[K, V]{}
	for i := range m.shards {
		m.shards[i].m = make(map[*K]V)
	}
	return m
}

func (m *shardMap[K, V]) shard(k *K) *shard[K, V] {
	// prime is a Fibonacci prime close to the golden ratio * 2^64.
	// Common choice for multiplicative hashing.
	const prime = 11400714819323198485

	ptr := uintptr(unsafe.Pointer(k))
	index := (ptr * prime) >> (bits.UintSize - shardBits)
	return &m.shards[index]
}

func (m *shardMap[K, V]) Store(k *K, v V) {
	s := m.shard(k)
	s.Lock()
	s.m[k] = v
	s.Unlock()
}

func (m *shardMap[K, V]) LoadAndDelete(k *K) (V, bool) {
	s := m.shard(k)
	s.Lock()
	v, ok := s.m[k]
	if ok {
		delete(s.m, k)
	}
	s.Unlock()
	return v, ok
}

func (m *shardMap[K, V]) Count() int {
	n := 0
	for i := range m.shards {
		s := &m.shards[i]
		s.Lock()
		n += len(s.m)
		s.Unlock()
	}
	return n
}
