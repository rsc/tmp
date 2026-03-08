// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type rules struct {
	text  strings.Builder
	files map[string]filePerm
}

type filePerm uint

const (
	denyAll filePerm = 1 << iota
	allowStat
	allowRead
	allowWrite
)

func newRules() *rules {
	r := &rules{
		files: make(map[string]filePerm),
	}
	r.text.WriteString(rulesHeader)
	return r
}

func (r *rules) addFiles() {
	// Would prefer to only do statFile here,
	// but C programs seem to abort at startup if they cannot read /.
	r.readFile("/")

	// Devices
	r.readFile("/dev/autofs_nowait")
	r.writeFile("/dev/dtracehelper")
	r.writeFile("/dev/null")
	r.writeFile("/dev/ptmx")
	r.writeFile("/dev/random")
	r.writeFile("/dev/tty")
	r.writeFile("/dev/zero")
	r.writeTree("/dev/fd")
	// Allow access to tty by actual name.
	// TODO: Limit to current tty file, but need to find out how to find name.
	// TODO: If we do limit, how do we deal with posix_openpt?
	r.text.WriteString(`(allow file-read* file-write* file-ioctl (regex #"^/dev/tty[^/]*$"))` + "\n")

	// macOS applications and system files
	r.readTree("/AppleInternal") // does not exist but many programs check
	r.readTree("/Applications")
	r.readTree("/BinaryCache")  // does not exist but many programs check
	r.readTree("/BuildSupport") // does not exist but many programs check
	r.readTree("/Library")
	r.readTree("/System")

	// Unix applications and system files
	r.readTree("/bin")
	r.readTree("/usr")
	r.readTree("/sbin")

	// Common additional programs
	r.readTree("/opt/homebrew")

	// System files
	r.readTree("/etc")
	r.readTree("/var/select")
	r.readTree("/var/db/timezone")

	// Temporary directories.
	r.statFile("/tmp")
	r.writeTree("/private/tmp")
	if tmpdir := os.Getenv("TMPDIR"); tmpdir != "" {
		// Cut /T/ because clang somehow finds /C/ and uses it for caches.
		tmpdir = strings.TrimSuffix(tmpdir, "/T/")
		r.writeTree(tmpdir)
	}

	// Allow access to directories listed in $PATH.
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if strings.HasPrefix(dir, "/") {
			r.readTree(dir)
		}
	}

	// Allow using locally built Go root.
	if goroot := os.Getenv("GOROOT"); goroot != "" {
		r.readTree(goroot)
	}

	// Allow access to GOPATH.
	if gopath := os.Getenv("GOPATH"); gopath != "" {
		r.readTree(filepath.Join(gopath, "bin"))  // installed programs
		r.writeTree(filepath.Join(gopath, "pkg")) // module cache
	}

	// Allow write access to $BOXROOT.
	if boxroot := os.Getenv("BOXROOT"); boxroot != "" {
		r.writeTree(boxroot)
	}

	// Allow access to parts of HOME.
	if home := os.Getenv("HOME"); home != "" {
		// Deny is the default but avoid mistakes.
		r.denyFile(filepath.Join(home, ".env"))
		r.denyFile(filepath.Join(home, ".gitcookies"))
		r.denyFile(filepath.Join(home, ".netrc"))
		r.denyTree(filepath.Join(home, ".ssh"))
		// Every C program reads this file.
		r.readFile(filepath.Join(home, ".CFUserTextEncoding"))
		// Allow .gitconfig so git will start up.
		r.readFile(filepath.Join(home, ".gitconfig"))
		// Allow read-only access to files like go env.
		r.readTree(filepath.Join(home, "Library/Application Support"))
		// Allow writing Go telemetry data.
		r.writeTree(filepath.Join(home, "Library/Application Support/go/telemetry"))
		// Allow writing to build caches for Go, Bazel. Maybe a little dangerous.
		r.writeTree(filepath.Join(home, "Library/Caches"))
		// Allow using downloaded Go toolchains.
		r.readTree(filepath.Join(home, "sdk"))
	}
}

func (r *rules) allow(path string, perm filePerm) {
	path = filepath.Clean(path)
	if r.files[path]&perm == perm {
		return
	}
	r.files[path] |= perm

	// Allow same permissions on target of symlink.
	if target, err := filepath.EvalSymlinks(strings.TrimSuffix(path, "/**")); err == nil {
		if strings.HasSuffix(path, "/**") {
			target += "/**"
		}
		r.allow(target, perm)
	}

	if perm&denyAll != 0 {
		return
	}

	// Allow stat of files up to root.
	dir := filepath.Dir(path)
	for {
		r.files[dir] |= allowStat
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
}

func (r *rules) denyFile(path string)  { r.allow(path, denyAll) }
func (r *rules) readFile(path string)  { r.allow(path, allowRead|allowStat) }
func (r *rules) writeFile(path string) { r.allow(path, allowWrite|allowRead|allowStat) }
func (r *rules) statFile(path string)  { r.allow(path, allowStat) }
func (r *rules) denyTree(path string)  { r.denyFile(path + "/**") }
func (r *rules) readTree(path string)  { r.readFile(path + "/**") }
func (r *rules) writeTree(path string) { r.writeFile(path + "/**") }

func (r *rules) emitFileRules() {
	for _, path := range slices.Sorted(maps.Keys(r.files)) {
		perm := r.files[path]
		if perm&denyAll != 0 {
			fmt.Fprintf(&r.text, "(deny file-read* file-write* file-ioctl ")
		} else {
			fmt.Fprintf(&r.text, "(allow ")
			if perm&allowStat != 0 {
				fmt.Fprintf(&r.text, "file-read-metadata ")
			}
			if perm&allowRead != 0 {
				fmt.Fprintf(&r.text, "file-read* ")
			}
			if perm&allowWrite != 0 {
				fmt.Fprintf(&r.text, "file-write* file-ioctl ")
			}
		}
		if strings.HasSuffix(path, "/**") {
			fmt.Fprintf(&r.text, "(subpath %q)", strings.TrimSuffix(path, "/**"))
		} else {
			fmt.Fprintf(&r.text, "(literal %q)", path)
		}
		fmt.Fprintf(&r.text, ")\n")
	}
}

var rulesHeader = `
; macOS Seatbelt sandbox profile
;
; To watch rejected operations, use:
;	log stream --style compact --predicate 'sender=="Sandbox"'
; Not everything shows up there.
; For example Java needs (allow dynamic-code-generation)
; but if that is omitted, there is no log indicating that's the problem.
;
; For more examples, see /System/Library/Sandbox/Profiles on a Mac and also:
; https://source.chromium.org/chromium/chromium/src/+/main:sandbox/policy/mac/
; https://source.chromium.org/chromium/chromium/src/+/main:sandbox/policy/mac/common.sb
; https://github.com/openai/codex/blob/main/codex-rs/core/src/seatbelt_base_policy.sbpl
; https://github.com/openai/codex/blob/main/codex-rs/core/src/seatbelt_network_policy.sbpl
; https://github.com/openai/codex/blob/main/codex-rs/core/src/seatbelt_platform_defaults.sbpl
; https://github.com/anthropic-experimental/sandbox-runtime/blob/main/src/sandbox/macos-sandbox-utils.ts
;
; https://bdash.net.nz/posts/tcc-and-the-platform-sandbox-policy/
; https://bdash.net.nz/posts/sandboxing-on-macos/
; https://www.romab.com/ironsuite/SBPL.html

; Sandbox profile language version, not the version of this sandbox rule set.
(version 1)

; Deny everything by default.
; For debugging, can try:
;	(allow (with report) default)
(deny default)

; System operations.

(allow distributed-notification-post)
(allow dynamic-code-generation)
(allow iokit-get-properties)
(allow iokit-open)
(allow ipc-posix-sem)
(allow ipc-posix-shm)
(allow mach-lookup)
(allow mach-priv-task-port (target same-sandbox))
(allow process-exec)
(allow process-fork)
(allow process-info* (target same-sandbox))
(allow signal (target same-sandbox))
(allow sysctl-read)
(allow user-preference-read)

; Network

(allow network-bind)
(allow network-inbound)
(allow network-outbound)
(allow system-socket)
(allow socket-option-get)
(allow socket-option-set)
(allow socket-ioctl)
(allow iokit-open-user-client)
(allow necp-client-open)
(allow system-necp-client-action)

`
