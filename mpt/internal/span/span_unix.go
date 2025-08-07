//go:build !plan9 && !windows

// Package span implements growable memory spans.
package span

import (
	"fmt"
	"syscall"
)

type Span struct {
	max   int
	alloc int
	mem   []byte
}

const pageSize = 4 << 20

func round(n int) int {
	return (n + pageSize - 1) &^ (pageSize - 1)
}

// Reserve returns a Span with zero memory footprint
// but with space reserved to expand to at most max bytes.
func Reserve(max int) (*Span, error) {
	r := round(max)
	mem, err := syscall.Mmap(-1, 0, r, syscall.PROT_NONE, syscall.MAP_ANON|syscall.MAP_PRIVATE|syscall.MAP_NORESERVE)
	if err != nil {
		return nil, fmt.Errorf("span.Reserve %d: %w", r, err)
	}
	return &Span{max: max, mem: mem}, nil
}

// Expand expands the accessible memory to at least n bytes
// and returns a slice of length n and capacity n.
// Calling Expand with a small n does not release memory
// from an earlier call with a larger n.
func (s *Span) Expand(n int) ([]byte, error) {
	r := round(n)
	if s.alloc < r {
		err := syscall.Mprotect(s.mem[s.alloc:r], syscall.PROT_READ|syscall.PROT_WRITE)
		if err != nil {
			return nil, fmt.Errorf("span.Expand %d..%d: %w", s.alloc, r, err)
		}
		s.alloc = r
	}
	return s.mem[:n:n], nil
}

// Release unmaps the span. Previously returned memory must not be used after calling Release.
func (s *Span) Release() error {
	if err := syscall.Munmap(s.mem); err != nil {
		return fmt.Errorf("span.Release: %w", err)
	}
	s.mem = nil
	return nil
}
