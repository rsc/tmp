// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package pmem implements persistent, transactional
// memory backed by on-disk files.
// This package only runs on 64-bit Unix architectures.
// (It could be made to run on Windows;
// it cannot be made to run on 32-bit systems, nor on Plan 9.)
//
// Each memory image is represented by a [Mem]
// and stored in a pair of on-disk files.
// See the [Mem] documentation for more details.
package pmem

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"io"
	"log"
	"runtime/debug"
	"strings"
	"time"

	"rsc.io/tmp/mpt/internal/slicemath"
	"rsc.io/tmp/mpt/internal/span"
)

const verboseIO = false

// File Layout
//
//	magic [multiple of 8 bytes]
//	frames...
//
// Frame Layout
//
//	id [16]
//	seq [8]
//	len [8]
//	data [len]
//	sum [32]
//
// Patch frames are a sequence of mutations, each of the form:
//
//	offset [v]
//	len [v]
//	data [len]
//
// where [v] denotes a uvarint-encoded int.

const (
	hashSize = sha256.Size

	// field offsets
	frameID  = 0
	frameSeq = 16
	frameLen = 24

	// frame header size
	frameSize = 32

	// total frame overhead, including final checksum
	frameExtra = frameSize + hashSize
)

// MaxGroupBytes is the maximum number of modified bytes
// that a mutation group can store: 1 MB.
// Writing the same bytes over and over counts toward the
// group limit each time.
const MaxGroupBytes = 1 << 20

// Logically, these are all constants, but we use variables
// so that they can be made smaller for testing.
var (
	// maxMem is the maximum memory size.
	// We reserve the entire address space when opening the memory,
	// so that it can be extended in place rather than needing to
	// move the memory.
	// 16 TB (44 bits) should be far more than enough for all practical uses,
	// while leaving 19 bits of headroom, so that many different
	// on-disk files can be used simultaneously.
	maxMem = 16 << 40

	// maxVarint is the maximum number of bytes needed for a varint
	// encoding numbers up to maxMem. maxMem is 1<<44, so we
	// need 45 bits, and at 7 bits per byte, that's 7 bytes.
	maxVarint = 7

	// maxPatch is the maximum size of a patch block,
	// which must be able to hold a full mutation group.
	// In the worst case, the framing of every data byte in
	// a mutation might be framed by a maxVarint-byte offset
	// and a 1-byte count. That's 1MB*8 = 8 MB.
	// There needs to be headroom for the memory-length patch,
	// so the minimum would be 8MB + 8 bytes, but we bump the
	// patch block size to 16 MB instead.
	maxPatch = 16 << 20
)

var errCorrupt = errors.New("corrupt input file")

// File is the interface needed for on-disk storage.
type File interface {
	io.ReaderAt
	io.WriterAt
	Sync() error
}

// DevNull returns a file like the Unix /dev/null: it can be written but is always empty.
// Passing two DevNull files to New creates a Mem with no on-disk backing.
func DevNull() File {
	return new(devNull)
}

type devNull struct{}

func (*devNull) ReadAt(b []byte, off int64) (int, error)  { return 0, io.EOF }
func (*devNull) WriteAt(b []byte, off int64) (int, error) { return len(b), nil }

func (*devNull) Sync() error { return nil }

// A Mem represents a persistent memory backed by a pair of on-disk files.
// The files must implement the [File] interface ([os.File] is the usual implementation).
// The on-disk footprint is approximately six times the size of the memory image.
//
// At any moment, the current memory image is stored in one of the files,
// while the other holds an in-progress image. The two files swap meanings
// once the in-progress image has been fully written. Alternating between the
// two files provides consistency as well as atomicity of updates,
// including transactional grouping of writes.
//
// [Create] creates a new memory, and [Open] opens an existing one.
//
// The [Mem.Data] method provides access to a view of the memory,
// but modifications to it must be made using [Mem.Mutate],
// so that the changes can be logged to disk as well.
// The memory starts out with zero length and can be expanded
// to larger sizes using [Mem.Expand].
// (Shrinking the memory is not implemented.)
//
// Mutations can be grouped into atomic transactions using
// [Mem.BeginGroup] and [Mem.EndGroup].
//
// Calling [Mem.Sync] ensures that all modifications have been flushed
// to the underlying files, guaranteeing that a future [Open] will observe them.
//
// Calling [Mem.Close] closes the memory and leaves the mapping unreadable.
// Future accesses to the slice data returned by [Mem.Data] must be avoided.
// Those accesses will fault, meaning they crash the program unless
// [runtime/debug.SetPanicOnFault] has been used.
type Mem struct {
	magic     string
	id        [16]byte
	tmp       [frameExtra]byte
	ptmp      [2 * binary.MaxVarintLen64]byte
	span      *span.Span
	mem       []byte
	patched   int // length of “patched” section of memory
	current   *writer
	next      *writer
	disk      File  // disk-only (not in memory) storage
	diskOff   int64 // offset where user writes begin
	patch     []byte
	group     int // group start in patch, or -1 if not in group
	groupData int // total group data
	err       error
	closed    bool
	compact   compact

	constantFlushing bool

	syncHook   func()
	mutateHook func()
}

// A reader is the state for reading an input file.
type reader struct {
	file   File
	id     [16]byte  // id found in file
	seq    uint64    // file sequence number
	off    int64     // read offset in file
	hash   hash.Hash // hash of frame
	memLen int       // memory length
	tmp    [max(frameSize, 2*hashSize)]byte
}

// A writer is the state for writing to an output file.
type writer struct {
	file  File
	seq   uint64 // file sequence number
	off   int64  // write offset in file
	wrote bool   // any writes since last sync?

	hash hash.Hash
	tmp  [max(hashSize, frameSize)]byte
}

// A compact is the state for compaction.
type compact struct {
	hash hash.Hash
	off  int
	end  int
}

// broken marks the memory broken with err as the reason
// and returns err back to the caller.
// Only the first error is recorded.
// Any method on m that calls a non-m-method is expected to
// call m.broken if the method fails, so that I/O or data corruption
// errors permanently break all future uses of the memory.
func (m *Mem) broken(err error) error {
	if m.err == nil {
		fmt.Println("BROKEN ", err, " \n", string(debug.Stack()))
		m.err = err
	}
	return err
}

// Create initializes a new memory stored in mem1, mem2,
// with disk-only storage in disk if non-nil.
//
// The magic string is recorded at the start of the file to
// distinguish different uses of persistent memory files.
// A future call to [Open] must pass the same magic string.
// The magic string must not contain any NUL (\x00) bytes.
//
// Create does not check that the files are empty, in order
// to support using pre-allocated disk files or raw disk partitions.
func Create(magic string, mem1, mem2, disk File) (*Mem, error) {
	return create(magic, mem1, mem2, disk)
}

// newMem allocates a new Mem for use by Create and Open.
func newMem(magic string) (*Mem, error) {
	if strings.Contains(magic, "\x00") {
		return nil, fmt.Errorf("magic %q must not contain NUL (\x00) byte", magic)
	}
	magic += "\x00\x00\x00\x00\x00\x00\x00"[:7&-len(magic)] // pad to 8 bytes

	sp, err := span.Reserve(maxMem)
	if err != nil {
		return nil, err
	}
	m := &Mem{
		magic: magic,
		span:  sp,
		patch: make([]byte, 0, maxPatch),
		group: -1,
		compact: compact{
			hash: sha256.New(),
		},
	}
	return m, nil
}

func newWriter(file File, seq uint64) *writer {
	return &writer{
		file: file,
		hash: sha256.New(),
		seq:  seq,
	}
}

// create implements Create but avoids exposing named results in the docs.
func create(magic string, file1, file2, disk File) (_ *Mem, err error) {
	m, err := newMem(magic)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			m.span.Release()
			m.span.UnsafeUnmap()
		}
	}()

	rand.Read(m.id[:])
	m.current = newWriter(file1, 1)
	m.next = newWriter(file2, 0)

	if err := m.writeEmptyTree(m.current); err != nil {
		return nil, err
	}
	if err := m.writeEmptyTree(m.next); err != nil {
		return nil, err
	}
	if disk != nil {
		w := newWriter(disk, 0)
		if err := m.writeEmptyTree(w); err != nil {
			return nil, err
		}
		m.disk = disk
		m.diskOff = w.off
	}
	return m, nil
}

func (m *Mem) writeEmptyTree(w *writer) error {
	if err := w.write([]byte(m.magic)); err != nil {
		return m.broken(err)
	}
	if err := m.writeFrame(w, nil); err != nil {
		return err
	}
	return m.sync(w)
}

// SetConstantFlushing sets whether the memory should write every
// mutation to the files as quickly as possible. The setting defaults to false,
// in which case mutations are written to the files only when enough mutations
// have accumulated to fill a patch block or when Sync is called.
// Enabling constant flushing instead writes a separate patch block for
// every individual mutation outside a group, and for each group.
// This is an inefficient use of both file space and I/O bandwidth,
// but it can be useful for testing purposes to explore the full set of
// possible intermediate memory images that might be observed
// after a crash.
func (m *Mem) SetConstantFlushing(on bool) {
	m.constantFlushing = on
	if on {
		m.flushPatch(false)
	}
}

func (w *writer) write(b []byte) error {
	if err := w.writeAt(b, w.off); err != nil {
		return err
	}
	w.off += int64(len(b))
	return nil
}

func (w *writer) writeAt(b []byte, off int64) error {
	start := time.Now()
	_, err := w.file.WriteAt(b, off)
	if verboseIO {
		log.Printf("write %d %.6fs\n", len(b), time.Since(start).Seconds())
	}
	if err != nil {
		return err
	}
	w.wrote = true
	return nil
}

// Open opens the memory stored in the file pair mem1, mem2,
// with disk-only storage in disk if non-nil.
// The magic string must match the one used when the
// file pair was created with [Create].
func Open(magic string, mem1, mem2, disk File) (*Mem, error) {
	return open(magic, mem1, mem2, disk)
}

// open implements Open but avoids exposing named results in the docs.
func open(magic string, file1, file2, disk File) (_ *Mem, err error) {
	m, err := newMem(magic)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			m.span.Release()
			m.span.UnsafeUnmap()
		}
	}()

	// Peek at initial frame in both files.
	r1, err := m.readStart(file1)
	if err != nil {
		return nil, err
	}
	r2, err := m.readStart(file2)
	if err != nil {
		return nil, err
	}
	if r1.id != r2.id {
		return nil, fmt.Errorf("inconsistent pmem files: mismatched IDs")
	}
	if r1.seq == r2.seq {
		return nil, fmt.Errorf("inconsistent pmem files: identical sequence numbers (%#x == %#x)", r1.seq, r2.seq)
	}
	if r1.seq < r2.seq {
		r1, r2 = r2, r1
	}
	if disk != nil {
		rd, err := m.readStart(disk)
		if err != nil {
			return nil, err
		}
		if rd.id != r1.id {
			return nil, fmt.Errorf("inconsistent pmem files: disk ID does not match mem ID")
		}
		m.disk = disk
		m.diskOff = rd.off + hashSize
	}
	if err := m.readFile(r1); err != nil {
		return nil, err
	}
	m.current = newWriter(r1.file, r1.seq)
	m.current.off = r1.off
	m.next = newWriter(r2.file, 0)
	return m, nil
}

// readStart reads the start of the file and returns a reader
// that can read the remainder of the file as well as
// the initial metadata observed at the start.
// The reader is positioned immediately after the magic string.
func (m *Mem) readStart(file File) (*reader, error) {
	r := &reader{
		file: file,
		hash: sha256.New(),
	}
	magic := make([]byte, len(m.magic))
	if err := m.read(r, magic); err != nil {
		return nil, err
	}
	if string(magic) != m.magic {
		return nil, m.broken(fmt.Errorf("bad magic: %q != %q",
			strings.TrimRight(string(magic), "\x00"),
			strings.TrimRight(m.magic, "\x00")))
	}

	var err error
	r.id, r.seq, r.memLen, err = r.readFrameHeader()
	if err != nil {
		return nil, m.broken(err)
	}
	return r, nil
}

// read reads exactly len(data) bytes into data from r.file at r.off.
func (m *Mem) read(r *reader, data []byte) error {
	_, err := r.file.ReadAt(data, r.off)
	if err != nil {
		return m.broken(err)
	}
	r.off += int64(len(data))
	return nil
}

// readFrameHeader reads and parses the next frame header from r.
// It resets r.hash and writes the header to it.
func (r *reader) readFrameHeader() (id [16]byte, seq uint64, n int, err error) {
	f := r.tmp[:frameSize]
	if _, err = r.file.ReadAt(f, r.off); err != nil {
		return
	}
	r.off += int64(len(f))
	r.hash.Reset()
	r.hash.Write(f)
	copy(id[:], f[frameID:])
	seq = binary.BigEndian.Uint64(f[frameSeq:])
	n = int(binary.BigEndian.Uint64(f[frameLen:]))
	if n < 0 {
		err = errCorrupt
	}
	return
}

// readFrame reads a single framed block from r,
// storing the data into data.
// If data is not large enough to hold the framed data,
// readFrame returns errCorrupt.
func (r *reader) readFrame(data []byte) (int, error) {
	id, seq, n, err := r.readFrameHeader()
	if err != nil {
		return 0, err
	}
	if id != r.id || seq != r.seq || n > len(data) {
		//println("BAD seq", seq, r.seq, n, len(data))
		return 0, errCorrupt
	}
	if _, err := r.file.ReadAt(data[:n], r.off); err != nil {
		return 0, err
	}
	r.off += int64(n)
	r.hash.Write(data[:n])
	fsum := r.tmp[:hashSize]
	if _, err := r.file.ReadAt(fsum, r.off); err != nil {
		return 0, err
	}
	r.off += int64(len(fsum))
	hsum := r.hash.Sum(r.tmp[hashSize:hashSize])
	if [hashSize]byte(fsum) != [hashSize]byte(hsum) {
		//println("BAD hash")
		return 0, errCorrupt
	}
	return n, nil
}

// readFile reads an entire memory image file from r.
// It assumes the magic string has been checked already.
// It reads an initial memory image of length memLen bytes
// followed by any number of patch blocks modifying or
// extending that image.
func (m *Mem) readFile(r *reader) error {
	r.off = int64(len(m.magic))
	mem, err := m.span.Expand(r.memLen)
	if err != nil {
		return m.broken(err)
	}
	n, err := r.readFrame(mem[:r.memLen])
	if err != nil {
		return m.broken(err)
	}
	if n != r.memLen {
		// Unreachable unless file is changing underfoot.
		// Caller just read the frame length and it was memLen.
		return m.broken(errCorrupt)
	}
	m.mem = mem
	m.patched = len(mem)

	patch := make([]byte, maxPatch)
	for {
		n, err := r.readFrame(patch)
		if err == errCorrupt || err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
		if err != nil {
			return m.broken(err)
		}
		if err := m.replay(patch[:n]); err != nil {
			return err
		}
	}
	return nil
}

// replay applies the mutations listed in patch to m.mem.
// It expands m.mem as needed to apply the patch.
func (m *Mem) replay(patch []byte) error {
	//fmt.Printf("REPLAY\n%s\n", hex.Dump(patch))
	for len(patch) > 0 {
		off, n := binary.Uvarint(patch)
		if n <= 0 {
			return m.broken(errCorrupt)
		}
		isDisk := off&1 != 0
		off >>= 1
		patch = patch[n:]
		count, n := binary.Uvarint(patch)
		if n <= 0 {
			return m.broken(errCorrupt)
		}
		patch = patch[n:]
		//fmt.Printf("AT %#x+%#x\n", off, count)
		if count > uint64(len(patch)) || off+count < off || int(off+count) < 0 {
			return m.broken(errCorrupt)
		}
		if isDisk {
			// disk patch
			if _, err := m.disk.WriteAt(patch[:count], m.diskOff+int64(off)); err != nil {
				return m.broken(err)
			}
		} else {
			// memory patch
			if off+count > uint64(len(m.mem)) {
				mem, err := m.span.Expand(int(off + count))
				if err != nil {
					return m.broken(err)
				}
				m.mem = mem
			}
			copy(m.mem[off:off+count], patch[:count])
			m.patched = max(m.patched, int(off+count))
		}
		patch = patch[count:]
	}
	return nil
}

// Data returns the current memory.
// Changes to the memory must be made only using [Mem.Mutate],
// never using direct writes. Changes made by direct write will not
// be visible when the memory is reloaded by a future [Open].
func (m *Mem) Data() []byte {
	return m.mem
}

// Expand extends the length of the current memory
// to be at least n bytes and returns the extended slice.
func (m *Mem) Expand(n int) ([]byte, error) {
	if m.err != nil {
		return nil, m.err
	}
	if n <= len(m.mem) {
		return m.mem, nil
	}
	mem, err := m.span.Expand(n)
	if err != nil {
		// Do not use m.broken - nothing is broken yet.
		// Caller might recover gracefully from being unable
		// to expand the memory.
		return nil, err
	}
	m.mem = mem
	return m.mem, nil
}

// Offset returns the starting offset of b within the memory.
// If b is not a subslice of m.Data(), Offset returns 0, false.
func (m *Mem) Offset(b []byte) (offset int, ok bool) {
	off, ok := slicemath.Offset(m.mem, b)
	return int(off), ok
}

// BeginGroup starts an atomic mutation group.
// Expand and Mutate calls between Begin and [Mem.EndGroup]
// are guaranteed to be observed as an atomic unit
// upon reloading the memory: either they will all be
// present or none of them will be.
// Calls to BeginGroup must be followed eventually by a call to EndGroup
// and cannot be nested: it is an error to call BeginGroup twice
// without an intervening EndGroup.
//
// A group is limited to mutation of at most MaxGroupBytes bytes of mutated data.
func (m *Mem) BeginGroup() error {
	if m.err != nil {
		return m.err
	}
	if m.group >= 0 {
		return fmt.Errorf("atomic mutation group already begun")
	}

	// Patch buffer always has room to add an empty mutation
	// at the end of the memory, to represent the most recent Expand.
	// If the group grows too large, we will flush up to but not
	// including the group, so add the empty mutation now.
	if err := m.addMemLenPatch(); err != nil {
		return err
	}

	m.group = len(m.patch)
	m.groupData = 0
	return nil
}

// Mutate is like copy(dst, src), where dst must be inside m.Data()
// and src and dst must have the same length,
// but it also arranges to record the change on disk,
// so that it will be visible when the memory is reloaded.
func (m *Mem) Mutate(dst, src []byte) error {
	if m.err != nil {
		return m.err
	}
	if len(dst) != len(src) {
		return fmt.Errorf("mismatched dst, src len in mutation")
	}
	if len(dst) == 0 {
		return fmt.Errorf("empty mutation")
	}
	off, ok := m.Offset(dst)
	if !ok {
		return fmt.Errorf("invalid dst for mutation")
	}
	return m.mutate(uint64(off)<<1, src, func() error {
		copy(dst, src)
		m.patched = max(m.patched, off+len(src))
		return nil
	})
}

// WriteDisk writes src to the disk-only file at offset off.
// It guarantees that on recovery after a crash,
// all disk writes that occurred before the latest recovered Mutate
// will be available for reading.
// (Disk writes that happened after that Mutate may or may not
// be available for reading as well.)
func (m *Mem) WriteDisk(src []byte, off int64) error {
	if m.err != nil {
		return m.err
	}
	if len(src) == 0 {
		return nil
	}
	return m.mutate(uint64(off)<<1|1, src, func() error {
		_, err := m.disk.WriteAt(src, m.diskOff+off)
		if err != nil {
			return m.broken(err)
		}
		return nil
	})
}

// ReadDisk reads into dst from the disk-only file at offset off.
func (m *Mem) ReadDisk(dst []byte, off int64) error {
	if m.err != nil {
		return m.err
	}
	_, err := m.disk.ReadAt(dst, m.diskOff+off)
	if err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return m.broken(err)
	}
	return nil
}

// mutate logs a write to the patch block, starting a new patch block if necessary.
// It calls commit to apply the actual write once it has checked a few
// error conditions.
//
// The offset off is the one recorded in the patch block.
// For mutations of the memory at offset o, off should be o<<1.
// For mutations of the disk-only file at offset o, off should be o<<1|1.
func (m *Mem) mutate(off uint64, src []byte, commit func() error) error {
	// Note: it is tempting to return early if bytes.Equal(dst, src) is true,
	// but there are two problems with that. One is that some callers
	// may modify dst in place and then call m.Mutate(dst, dst).
	// The other is that Mem.End depends on len(dst)==0 emitting
	// a mutation, and of course all zero-length slices are equal.

	if m.group >= 0 {
		if m.groupData+len(src) > MaxGroupBytes {
			return fmt.Errorf("mutation group too large")
		}
	}
	p := m.ptmp[:0]
	//fmt.Printf("PATCH @%d %#x+%#x\n", len(m.patch), off, len(src))
	p = binary.AppendUvarint(p, off)
	p = binary.AppendUvarint(p, uint64(len(src)))
	if len(m.patch)+len(p)+len(src)+maxVarint+1 > maxPatch {
		if err := m.flushPatch(true); err != nil {
			return err
		}
	}
	m.patch = append(m.patch, p...)
	m.patch = append(m.patch, src...)
	if m.group >= 0 {
		m.groupData += len(src)
	}
	if commit != nil {
		if err := commit(); err != nil {
			return err
		}
	}
	if m.mutateHook != nil && m.group < 0 {
		m.mutateHook()
	}
	if m.group < 0 && m.constantFlushing {
		if err := m.flushPatch(true); err != nil {
			return err
		}
	}
	return nil
}

// flushPatch flushes the current patch buffer to disk.
// If there is an active mutation group, only the buffer before
// that group is written.
func (m *Mem) flushPatch(needSpace bool) error {
	var p []byte
	if m.group >= 0 {
		// Can only write up to m.group, but final mem len is already there.
		if m.group == 0 {
			if needSpace {
				return m.broken(fmt.Errorf("pmem: internal error: group overflow"))
			}
			return nil
		}
		p = m.patch[:m.group]
	} else {
		if err := m.addMemLenPatch(); err != nil {
			return err
		}
		p = m.patch
	}
	if len(p) == 0 {
		return nil
	}
	//fmt.Printf("FLUSH\n%s\n", hex.Dump(p))

	if err := m.writeFrame(m.current, p); err != nil {
		return err
	}
	if m.next.seq != 0 {
		if err := m.writeFrame(m.next, p); err != nil {
			return err
		}
	}
	m.patch = m.patch[:copy(m.patch, m.patch[len(p):])] // slide rest down
	if m.group >= 0 {
		m.group = 0
	}
	//fmt.Printf("REMAIN\n%s\n", hex.Dump(m.patch))
	return m.maybeCompact(2 * len(p))
}

// EndGroup finishes an atomic mutation group,
// which must have been started by [Mem.BeginGroup].
func (m *Mem) EndGroup() error {
	if m.err != nil {
		return m.err
	}
	if m.group < 0 {
		return fmt.Errorf("no atomic mutation group to end")
	}
	if m.patched != len(m.mem) {
		// Before closing group, append an empty mutation if an Expand happened.
		// Not using m.addMemLenPatch because we need to preserve the invariant
		// that there will be room for _another_ when the eventual flush happens.
		m.mutate(uint64(len(m.mem))<<1, nil, nil)
		m.patched = len(m.mem)
	}
	m.group = -1

	if m.next.seq > 0 && m.compact.off == m.compact.end && len(m.patch) > 0 {
		m.flushPatch(false)
	}
	return nil
}

// addMemLenPatch adds a final “memory length” patch to m.patch.
// Mutate ensures that there is always room for this final patch,
// so the error return should never happen.
func (m *Mem) addMemLenPatch() error {
	if m.patched == len(m.mem) {
		return nil
	}
	if len(m.patch)+maxVarint+1 > maxPatch {
		return m.broken(fmt.Errorf("pmem internal patch overflow"))
	}
	m.patch = binary.AppendUvarint(m.patch, uint64(len(m.mem))<<1)
	m.patch = binary.AppendUvarint(m.patch, 0)
	m.patched = len(m.mem)
	return nil
}

// writeFrame writes a frame containing data to w.
func (m *Mem) writeFrame(w *writer, data []byte) error {
	f := m.tmp[:frameSize]
	copy(f[frameID:], m.id[:])
	binary.BigEndian.PutUint64(f[frameSeq:], w.seq)
	binary.BigEndian.PutUint64(f[frameLen:], uint64(len(data)))

	w.hash.Reset()
	w.hash.Write(f)
	if err := w.write(f); err != nil {
		return m.broken(err)
	}

	w.hash.Write(data)
	if err := w.write(data); err != nil {
		return m.broken(err)
	}

	sum := w.hash.Sum(w.tmp[:0])
	if err := w.write(sum); err != nil {
		return m.broken(err)
	}

	return nil
}

// maybeCompact runs a bit of compaction if needed,
// limiting I/O to writing at most n data bytes plus some framing.
func (m *Mem) maybeCompact(n int) error {
	//println("MAYBE", m.next.seq, m.current.off, 2*int64(len(m.mem)))
	if m.next.seq == 0 && m.current.off < 2*int64(len(m.mem)) {
		// Current disk file is less than twice the tree memory.
		// Not worth compacting yem.
		return nil
	}

	c := &m.compact
	if m.next.seq == 0 {
		if verboseIO {
			log.Print("compact start")
		}
		// Start a new compaction.
		// Record current tree size (but not content),
		// so we know where patches should be written.
		c.end = len(m.mem)
		c.off = 0
		c.hash.Reset()

		m.next.off = int64(len(m.magic) + frameSize + c.end + 32)
		m.next.seq = m.current.seq + 1

		// Hash the correct frame header.
		var frame [frameSize]byte
		copy(frame[frameID:], m.id[:])
		binary.BigEndian.PutUint64(frame[frameSeq:], m.next.seq)
		binary.BigEndian.PutUint64(frame[frameLen:], uint64(c.end))
		c.hash.Write(frame[:])

		// But write seq=0 to disk for now, so that if we crash before finishing,
		// the next Open will not try to use this file.
		// We will write the correct sequence number once everything is on disk.
		binary.BigEndian.PutUint64(frame[frameSeq:], 0)
		if err := m.next.writeAt(frame[:], int64(len(m.magic))); err != nil {
			return m.broken(err)
		}
	}

	// Write at most n bytes of data, both to c.hash and to m.next.file.
	//
	// Note: If compaction were running in parallel with writes,
	// we could copy from c.mem racily into a buffer and then write
	// the buffer to both the hash and the file. As long as they are
	// consistent, any racy reads would not matter, since the writes
	// we are racing against would be written in patch form, even if
	// we didn't see them here. However, since we run compaction
	// interleaved with other work, there should be no writes to c.mem,
	// and we can read from it twice.
	if c.off < c.end && n > 0 {
		n := min(n, c.end-c.off)
		c.hash.Write(m.mem[c.off : c.off+n])
		if err := m.next.writeAt(m.mem[c.off:c.off+n], int64(len(m.magic)+frameSize+c.off)); err != nil {
			return m.broken(err)
		}
		c.off += n
	}

	if c.off < c.end || m.group >= 0 || len(m.patch) > 0 {
		// Not finished. Wait for next call.
		// Note that if c.off == c.end but m.group >= 0,
		// then there is an active mutation group, and the memory image
		// we wrote may include writes from that group.
		// Similarly, if len(m.patch) > 0, the group may have ended
		// but the patches have not yet been flushed
		// (EndGroup will flush them for us but hasn't yet).
		// We cannot complete the image until the group is flushed.
		return nil
	}

	// Wrote entire tree image. Finish and switch.
	sum := c.hash.Sum(nil)
	if err := m.next.writeAt(sum[:], int64(len(m.magic)+frameSize+c.off)); err != nil {
		return m.broken(err)
	}

	// Sync m.disk to disk, because we are about to abandon
	// all the disk patches in m.current.
	if m.disk != nil {
		if err := m.disk.Sync(); err != nil {
			return m.broken(err)
		}
	}

	// Open will start using the tree when the bigger sequence number hits the disk,
	// so we want to make sure that happens last.
	// Sync entire tree to disk, then update sequence number, then sync again.
	if err := m.sync(m.next); err != nil {
		return err
	}
	if err := m.writeFrameSeq(m.next, int64(len(m.magic)), m.next.seq); err != nil {
		return err
	}
	if err := m.sync(m.next); err != nil {
		return err
	}

	if verboseIO {
		log.Print("compact switch")
	}
	// Switch current and next.
	m.current, m.next = m.next, m.current
	setCurrent(m.current.file, true, int(m.current.off))
	setCurrent(m.next.file, false, int(m.next.off))
	m.next.seq = 0
	return nil
}

// writeFrameSeq updates a frame header at the given offset,
// replacing the sequence number with seq and leaving the
// rest of the frame header unmodified.
func (m *Mem) writeFrameSeq(w *writer, off int64, seq uint64) error {
	binary.BigEndian.PutUint64(m.tmp[:], seq)
	if err := w.writeAt(m.tmp[:8], off+frameSeq); err != nil {
		return m.broken(err)
	}
	return nil
}

func setCurrent(f File, b bool, off int) {
	if f, ok := f.(interface{ setCurrent(bool, int) }); ok {
		f.setCurrent(b, off)
	}
}

// Sync flushes and syncs all memory changes to the underlying files.
//
// As changes are made with Mutate, they are flushed to disk
// incrementally, so that the in-memory footprint of a Mem
// is only a limited amount more than its memory data.
// Sync makes sure that all mutations have been written
// to the files and then calls [File.Sync] to sync those writes.
func (m *Mem) Sync() error {
	if m.err != nil {
		return m.err
	}
	if err := m.flushPatch(false); err != nil {
		return err
	}
	if m.syncHook != nil {
		m.syncHook()
	}
	if err := m.sync(m.current); err != nil {
		return err
	}
	if m.next.seq != 0 {
		if err := m.sync(m.next); err != nil {
			return err
		}
	}
	return nil
}

func (m *Mem) sync(w *writer) error {
	if w.wrote {
		start := time.Now()
		err := w.file.Sync()
		if verboseIO {
			log.Printf("sync %.6fs\n", time.Since(start).Seconds())
		}
		if err != nil {
			return m.broken(err)
		}
		w.wrote = false
	}
	return nil
}

// Release syncs the memory and makes the in-memory data unreadable.
// Future accesses to the slice data will fault, causing the program to crash,
// unless [runtime/debug.SetPanicOnFault] has changed the fault behavior.
//
// Release releases the Mem's physical memory back to the operating system,
// so that it can be used for other purposes.
// However, to make Release a safe operation, Release preserves the virtual
// address space reservation, which only costs a few kilobytes to maintain
// until the process exits.
// To release the virtual address space, see [Mem.UnsafeUnmap],
// but read and understand the warnings in its doc comment before using it.
func (m *Mem) Release() error {
	// Collect errors as we go, but make sure to reach end. No early returns.
	m.Sync()
	if m.mem != nil {
		if err := m.span.Release(); err != nil {
			m.broken(err)
		}
		m.mem = nil
	}
	m.closed = true
	if m.err != nil {
		return m.err
	}
	m.err = errors.New("mem already closed") // for next time
	return nil
}

// UnsafeUnmap unmaps the virtual address space used by a closed memory.
// If m has not been closed using [Mem.Close], UnsafeUnmap does nothing
// but return an error.
//
// Normally, calling [Mem.Close] is sufficient to release the Mem's resources,
// and UnsafeUnmap need not be used. The only reason to use UnsafeUnmap
// is because the program opens and closes hundreds of thousands of Mems
// and must unmap old ones to avoid running out of virtual address space.
// Programs that use only tens of thousands of Mems, or just a few,
// need not use UnsafeUnmap.
//
// After Close, future accesses to the slice data previously returned by [Mem.Data]
// are guaranteed to fault. After UnsafeUnmap, accesses may still fault,
// but if the operating system has reused the virtual address space for other
// purposes the accesses may succeed and read unrelated memory.
// This is why the method is considered unsafe.
// (Writes to the slice data would write that unrelated memory as well,
// but in a correct program there should not be any such writes,
// since writes should only be done using [Mem.Mutate].)
func (m *Mem) UnsafeUnmap() error {
	if !m.closed {
		return fmt.Errorf("mem not closed; cannot unmap")
	}
	m.span.UnsafeUnmap()
	return nil
}

// hash returns a short hash of the current memory content,
// useful for debugging and testing.
func (m *Mem) hash() string {
	h := sha256.Sum256(m.mem)
	s := base64.StdEncoding.EncodeToString(h[:])
	return fmt.Sprintf("%s/%#x", s[:7], len(m.mem))
}
