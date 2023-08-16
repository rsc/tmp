// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"debug/elf"
	"encoding/binary"
	"flag"
	"log"
	"os"
	"slices"
)

var le = binary.LittleEndian

func main() {
	flag.Parse()
	for _, arg := range flag.Args() {
		strip(arg)
	}
}

func strip(file string) {
	data, err := os.ReadFile(file)
	if err != nil {
		log.Fatal(err)
	}

	if len(data) < 16 || string(data[:4]) != elf.ELFMAG {
		log.Fatalf("not an elf file")
	}
	id := data[:16]
	if id[elf.EI_CLASS] != byte(elf.ELFCLASS64) {
		log.Fatalf("not a 64-bit elf file")
	}
	if id[elf.EI_DATA] != byte(elf.ELFDATA2LSB) {
		log.Fatalf("not a little-endian elf file")
	}
	if id[elf.EI_VERSION] != byte(elf.EV_CURRENT) {
		log.Fatalf("unknown elf version")
	}
	var hdr elf.Header64
	if err := binary.Read(bytes.NewReader(data), le, &hdr); err != nil {
		log.Fatalf("decoding header: %v", err)
	}
	if hdr.Phentsize != 56 || hdr.Shentsize != 64 {
		log.Fatalf("invalid sizes in elf header")
	}
	slice := func(start, size uint64) []byte {
		if start >= uint64(len(data)) || uint64(len(data))-start < size {
			log.Fatalf("%s: elf offsets out of range %d %d %d", file, start, size, len(data))
		}
		return data[start:][:size]
	}

	progs := make([]elf.Prog64, hdr.Phnum)
	sections := make([]elf.Section64, hdr.Shnum)
	decode(slice(hdr.Phoff, uint64(hdr.Phnum)*uint64(hdr.Phentsize)), progs)
	decode(slice(hdr.Shoff, uint64(hdr.Shnum)*uint64(hdr.Shentsize)), sections)

	// Zero old sections.
	clear(slice(hdr.Shoff, uint64(hdr.Shnum)*uint64(hdr.Shentsize)))

	// Parser for old string table.
	if int(hdr.Shstrndx) >= len(sections) {
		log.Fatalf("missing section header string table")
	}
	raw := slice(sections[hdr.Shstrndx].Off, sections[hdr.Shstrndx].Size)
	oldStr := slices.Clone(raw)
	clear(raw)
	nameAt := func(off uint32) string {
		if uint64(off) >= uint64(len(oldStr)) {
			log.Fatalf("invalid offset for section string name")
		}
		name := oldStr[off:]
		if i := bytes.IndexByte(name, 0); i >= 0 {
			name = name[:i]
		}
		return string(name)
	}

	// Trim prog file size to max of contained sections.
	fileMax := uint64(0)
	for i := range progs {
		p := &progs[i]
		if p.Type != uint32(elf.PT_LOAD) {
			continue
		}
		maxOff := uint64(0)
		for j := range sections {
			s := &sections[j]
			if s.Type != uint32(elf.SHT_NULL) && s.Type != uint32(elf.SHT_NOBITS) && p.Vaddr <= s.Addr && s.Addr < p.Vaddr+p.Filesz {
				o := s.Addr + s.Size - p.Vaddr
				if maxOff < o {
					maxOff = o
				}
			}
		}
		if p.Filesz > maxOff {
			p.Filesz = maxOff
		}
		if o := p.Off + p.Filesz; fileMax < o {
			fileMax = o
		}
	}
	data = data[:fileMax]

	// Write progs back.
	copy(data[hdr.Phoff:], encode(progs))

	// Build new section list and string table.
	str := "\x00.shstrtab\x00"
	var newSections []elf.Section64
	for j := range sections {
		s := sections[j]
		keep := s.Type == uint32(elf.SHT_NULL)
		if !keep {
			for i := range progs {
				p := &progs[i]
				if p.Vaddr <= s.Addr && s.Addr < p.Vaddr+p.Filesz {
					keep = true
					break
				}
			}
		}
		if !keep && s.Type == uint32(elf.SHT_NOBITS) && s.Flags&uint64(elf.SHF_ALLOC) != 0 {
			keep = true
			s.Off = fileMax
		}
		if !keep {
			continue
		}
		if s.Name == 0 {
			// do nothing
		} else {
			name := nameAt(s.Name)
			s.Name = uint32(len(str))
			str += name + "\x00"
		}
		newSections = append(newSections, s)
	}
	newSections = append(newSections, elf.Section64{
		Name:      1, // offset for .shstrtab
		Type:      uint32(elf.SHT_STRTAB),
		Off:       fileMax,
		Size:      uint64(len(str)),
		Addralign: 1,
	})

	// Add string table to end of file, pad to 8-byte boundary.
	data = append(data, str...)
	for len(data)&7 != 0 {
		data = append(data, 0)
	}

	// Write new sections.
	hdr.Shoff = uint64(len(data))
	hdr.Shnum = uint16(len(newSections))
	hdr.Shstrndx = hdr.Shnum - 1
	data = append(data, encode(newSections)...)

	// Write new header.
	copy(data, encode(&hdr))

	if err := os.WriteFile(file, data, 0666); err != nil {
		log.Fatal(err)
	}
}

func decode(buf []byte, data any) {
	err := binary.Read(bytes.NewReader(buf), le, data)
	if err != nil {
		log.Fatalf("decoding elf data: %v", err)
	}
}

func encode(data any) []byte {
	var buf bytes.Buffer
	err := binary.Write(&buf, le, data)
	if err != nil {
		log.Fatalf("encoding elf data: %v", err)
	}
	return buf.Bytes()
}
