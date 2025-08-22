# Merkle Patricia Tree Storage

This package implements a Merkle Patricia Tree (MPT) stored on disk.

A Merkle Patricia Tree (MPT) is a map that stores key-value pairs, where
each key and value is an opaque 256-bit value (typically a SHA256 hash).
Analogous to a [transparent log](https://research.swtch.com/tlog),
an MPT can cryptographically prove that a given key-value pair exists
(or that a key does not exist) in a given tree root.
By recording the sequence of tree roots in a transparent log,
a server can publish an record of the history of a key-value database,
in such a way that auditors can check that the database was correct
at all times, and clients can be sure the responses they received
came from the recorded database history.

## Motivation and Performance Goals {#perf}

[Certificate Transparency](https://certificate.transparency.dev/) (CT)
records a transparent log of all issued HTTPS certificates,
but it is expensive for a domain owner to
read the entire log looking for certificates for one or a few domains.
It would be useful to provide a server mapping from
domain to certificate lists, backed by an auditable MPT to verify
(after the fact) that the server has been behaving correctly.

CT logs contain about 2 billion entries and are growing by about 75 entries/second.
[Let's Encrypt](https://letsencrypt.org/) currently issues 90-day certificates
but plans to start issuing
[short-lived, six-day certificates](https://letsencrypt.org/2024/12/11/eoy-letter-2024/),
which they estimate could result in 20X as many certificates.
The number of domains does not change,
so the overall MPT size would stay at around 2 billion entries,
but the update rate would grow to perhaps 75×20 = 1,500 updates/second.
It would be good to be able to handle 75×50 = 3,750 updates/second to provide extra headroom.
We'd like to handle this load comfortably on one reasonably configured server.

Let's assume we have a single server with a
decent amount of memory and a fast disk.
For example, a Google Cloud
“m4-megamem-56” server with 56 vCPUs, 744 GiB of memory
costs $3,230/month (before sustained use or negotiated discounts).
Connecting two 1 TiB “balanced hyperdisks” each with
20,000 IOPS and 1200 MiB/s throughput adds $418/month.
([Pricing calculator](https://cloud.google.com/products/calculator?hl=en&dl=CjhDaVJsWVdZME1EVXpaQzFoWW1NMUxUUTFZMlF0T1RBNE15MDRNRFZtT0RFM09UWmhNR0VRQVE9PRAKGiRCQzY2RkQ0NC1CMEZGLTRFN0UtODJBMC0zNkM4NkI0QTQ5RjU))
For people running servers in their own data centers
or behind their own couches, a
[Thelio Astra with 512 GB of memory and 2 4TB NVMe SSDs](https://system76.com/desktops/thelio-astra-a1.1-n1/configure)
can be had for $5,462.

We will aim to be able to store, update, and serve an MPT of
2 billion entries with 4,000 updates/second easily on those servers.

If we are to serve 4,000 updates/second, each update must cost less than 250µs.
And if our budget is 20,000 I/O operations/second,
each update must cost less than 5 I/O operations.
A tree of two billion entries has height at least 30:
we don't have enough I/O budget even to read nodes from disk during updates,
not to mention lookups.
The inevitable conclusion is that we must keep the entire tree in memory,
streaming updates to disk in batches.
So we will do exactly that.

## Merkle Patricia Tree Overview {#mpt}

An MPT starts with the concept of a binary tree of depth 256, where the key-value
pairs are stored in the leaves at depth 256, and a lookup proceeds by walking
left or right according to each of the 256 key bits. The root node represents
the empty key prefix, its children represent key bit prefixes 0 and 1, their
children represent key bit prefixes 00, 01, 10, 11, and so on: at depth d,
the nodes represent key prefixes of d bits.
The original Key Transparency system at Google used exactly this data structure,
a [Merkle-hashed binary radix tree](https://github.com/google/keytransparency/blob/master/docs/overview.md).
Since then, the transparency community has realized that it works better
 to apply Merkle hashing to a Patricia tree,
which adds three optimizations to the binary radix tree.

First, the tree is “path-compressed,” by removing inner nodes with a single child:
a node that would have pointed at a single-child node is replaced by its child,
recursively. Every node is therefore either a leaf or an inner node with two children.
The path compression ensures that there are exactly _N_ inner nodes for a tree with _N_+1 leaf nodes.

Second, unlike in a normal binary tree, an inner node stores only the bit position
that determines whether a lookup should proceed to the left or right child.
A lookup walks inner nodes down to some leaf, checking one bit at each step.
Only upon reaching the leaf does it do a full key comparison.
If it takes _O_(_K_) time to compare two keys, a normal binary tree would
take _O_(_K_ log _N_) time for a walk; this optimization
cuts the time to _O_(_K_ + log _N_).
Furthermore, inner nodes need not store associated keys,
cutting the number of stored keys by a factor of two.

Third, nodes are “joined” by merging one inner node and one leaf node into
a single stored node.
(After joining _N_ inner nodes to _N_ leaf nodes, that leaves one “leaf-only”
node not paired to an inner node,
but the node is still stored using the joined representation.)
Whether a stored node represents an inner node or leaf node depends
on how it is reached
while walking the tree.
This trick is not essential, but it simplifies storage management
to have only one type of stored node.

The path-compression optimization implies that an inner node for key prefix _p_ exists
if and only if the tree contains at least one key with prefix _p_0 and at least one key with prefix _p_1.
That is, the specific inner nodes that exist in a Patricia depend only on which
keys are present in the tree, not on their insertion order.
This implies that we can batch or otherwise reorder insertions of distinct keys
without affecting the final tree structure.

For more about standard Patricia trees, see TODO REFERENCE.

Cominbing the Merkle and Patricia pieces, a Merkle Patricia Tree provides the following operations:

 - Set(key, val): add a new key-value pair to the map.
 - Snap(version): set the tree's version and return the tree root's key prefix and hash.
 - Prove(key): return a proof of the result of looking up a given key in the current snapshot. A separate library function `Verify` verifies a proof and returns the lookup result (whether the key was found and, if so, its associated value).
 - Sync(): flush recent changes to disk.

The recursive hash of an MPT is defined as follows:

 - The hash of a leaf node is the hash of its key and value.
 - The hash of an inner node is the hash of its bit position and its left and right children's hashes.

A proof confirming that a key-value pair exists in an MPT with a given recursive hash
is the value followed by the bit position and sibling hash for every inner node along the path
back to the root.
The key-value pair can be hashed to obtain the hash of the leaf node,
and then the running hash can be hashed with the parent's bit position
and sibling hash to obtain the hash of the next inner node toward the root.
(Whether the running hash is the left or right child is determined by checking
the specified bit of the key.)
Recomputing the root's actual hash proves the lookup.

A proof denying that a target key exists in an MPT is almost identical.
It consists of the “other key” whose leaf would be found by looking up the key in the tree,
followed by the proof that that other key is in the tree.
The verification checks that the other key's proof is valid and also that
the target key and other key agree at every relevant bit position.

An in-memory MPT implementation is in [mem.go](mem.go).
It was useful to write and debug that version before adding the
complexity of attempting to store the tree on disk.
If future algorithmic bugs are found, it may still be helpful to debug them in
that version first.
It may also be useful read and understand that implementation
before proceeding to the disk implementation.

TODO: link to other MPT implementations and contrast with this one?

## Storage Overview {#storage}

Although the working tree is stored in memory,
we of course want to recover from crashes by persisting the tree to a disk file as well.
The disk file consists of a memory image of a tree
followed by a sequence of patches of the form “at offset O, write these N bytes”.
Each update requires only a single disk write to append its patches to the file.
(Multiple updates can also be batched into a single write.)

There is only one problem with this representation: it grows without bound,
and faster than the tree.
A write of an existing key-value pair requires no additional memory at all,
but it requires 30 or so patches to existing nodes,
to update the hashes of the inner nodes along the path back to the root.
A write of a new key-value pair requires only one new allocated node,
but it too requires the same 30 or so patches.
There must be some kind of compaction.

The simplest way to compact one file is to write out a second disk file.
After all, the in-memory copy has all the patches applied already.
Conceptually, we can stop updates,
write the current tree memory to a new file,
delete the old file, and then resume updates,
now writing patches to the new file.
It is worth introducing two complications.
First, we can reuse the old file as the output for the next compaction,
alternating between a pair of files
instead of continually deleting and recreating files.
Second, we can let updates proceed concurrently
with compaction, so that updates aren't blocked
waiting to write a few hundred gigabytes to disk.

## Memory Format {#mem}

The memory format of the tree must be suitable for writing to disk
and then reading back into a different memory location,
so it cannot contain actual Go pointers.
Instead, the memory format is one very large byte array
that is interpreted as higher-level data structures
“on demand,” when accessing or modifying it.
Each actual update happens by writing to the memory
as bytes and also logging the mutation to a patch
that will be written to disk.

The tree memory starts with a header with the form:

	version  [ 8 bytes]
	dirty    [ 1 byte]
	pad      [ 1 byte]
	root     [ 6 bytes]
	hash     [32 bytes]
	nodes    [ 8 bytes]

All numbers are stored in big-endian order
for legibility when reading hex dumps.

 - “version” is a number for clients to use to match the
   tree contents to a position in the underlying transparent log.
 - “root” is a pointer to the tree's root node,
   represented as a 48-bit byte offset within the
   tree memory.
 - “nodes” field counts the number of nodes (leaves)
   stored in the tree.
 - “hash” is the Merkle hash of the tree root.
   When “dirty” is set, the hash is stale and needs to be recomputed.
 - “pad” pads “root” to a 16-bit boundary and “hash” and “nodes”
   to a 64-bit boundary.

The header is immediately followed by a sequence of Patricia nodes,
each with the form:

	key      [32 bytes]
	val      [32 bytes]
	bit      [ 1 byte]
	dirty    [ 1 byte]
	pad      [ 2 bytes]
	left     [ 6 bytes]
	right    [ 6 bytes]
	ihash    [32 bytes]

Remember that each Patricia node represents both one leaf node
and one inner node.

 - “key” and “val” are the key and value for the leaf node.
 - “left” and “right” are pointers to the inner node's
   left and right children;
   “bit” is the bit position to use to decide between them
   during a lookup.
   Since keys are 32 bytes, bit positions 0..255 fit in a single byte.
   The leaf-only node is an exception: it needs a bit position
   set to -1, but it can be identified by having “left” and “right” set to 0
   (nil pointer)
   and treated as a special case when reading “bit”.
 - “ihash” is the Merkle hash of the inner node.
   When “dirty” is set, the hash is stale and needs to be recomputed.
   The Merkle hash of the leaf node is not stored explicitly.
   It is recomputed from “key” and “val” whenever it is needed.

After setting the value associated with a given key,
the “ihash” values in all nodes back to the root
need to be recomputed.
If we are writing _N_ new values between
taking snapshots,
we would end up recomputing the root hash _N_−1 times
unnecessarily,
recomputing the root's two children's hashes _N_/2−1 times
unnecessarily, and so on.
Since everything else is in memory, batched, and cheap,
these SHA256 computations end up being the serving bottleneck.
To avoid the unnecessary hashes,
we don't recompute any hash during a write of a new value.
Instead, we set the “dirty” field on all nodes back
to the root.
The snapshot operation restores each hash by recomputing it
from its children,
restoring those hashes first as needed.
In effect, it rewalks the entire modified area of the tree,
computing all the new hashes then.
Snapshots are still amortized O(1) but not an actual O(1).
If the snapshot operations caused problematic latency hiccups,
this lazy recomputation could be abandoned.

Notice that a Patricia node takes 112 bytes,
so a 2-billion node tree requires about 224 GB of memory,
well within the 512 GB we allotted ourselves on our
“reasonably configured server”.
The actual memory for the tree is obtained directly
using the operating system, not from the Go heap.
Using _mmap_(2), we can reserve a very large amount
of space but then only map the memory we need as the tree grows.
This allows extending the tree without having to move it.
It also has the side benefit of not skewing the Go
garbage collector's pacing with one extremely large allocation.

The file format allows grouping memory updates into atomic units,
so that partially applied updates are never observed
when loading a tree from disk.

## File Format {#format}

A file consists of the magic string `"mpt tree\n\x00\x00\x00\x00\x00\x00\x00"`
followed by a sequence of variable-length frames.
Each frame has the form:

	treeID   [16 bytes]
	treeSeq  [ 8 bytes]
	N        [ 8 bytes]
    data     [ N bytes]
    checksum [32 bytes]

The “treeID” is randomly chosen when a tree is first created,
and the “treeSeq” is a sequence number incremented each time
a new tree file is written.
Both ensure that when files are reused
(either for a new tree or a new version of the same tree),
old frames are not misinterpreted as new ones.

The “checksum” is a SHA256 checksum of the preceding fields.
Verifying the checksum detects corruption but also provides
atomicity of frame writes: either the whole frame is written
to disk and the checksum matches, or none of it is used.

The file starts with one very large frame containing a snapshot
of the tree memory.
As we will see, concurrent compaction means that
this memory snapshot may not itself be a valid tree:
the patches in the rest of the file must be applied
not just to obtain the latest tree, but also to obtain
a valid tree.

The second and subsequent frames in the file each hold
a patch block, which holds one or more mutations of the form:

	offset   [varint]
	N        [varint]
	data     [N bytes]

That mutation says to write `data` of length `N` at
`offset` in the tree memory.

There is no guarantee that a file ends after a valid patch block frame.
If a frame was only partially written before a process or system crash,
we still want to read the tree before that point.
We do this by reading as many valid (checksum-matching) frames
as possible from the file and stopping at EOF or when we reach
a frame that is truncated or does not have a valid checksum.

## Compaction {#compaction}

When the current disk file holding a tree has grown too large,
compaction writes and then switches to a new smaller file,
at which point the old one can be abandoned.
In practice, the implementation reuses the old one for the
next compaction.

Logically, compaction is shortening the old file by writing
the patches directly to the tree memory.
However, it is not reading the old file: the tree
can be written directly from memory instead of consulting
the old file.
So technically compaction may be a misnomer:
the old file not being compacted so much as it is
being obsoleted.

One approach would be to pause all tree updates,
write the tree to the new file, and then continue
updates, writing patches to the new file.
If the tree is 224 GB,
then even if we can write at a relatively fast 10 GB/s,
that would be a 22-second pause.
Instead, we can allow tree updates to proceed
concurrently with compaction.

The concurrent compaction algorithm works as follows:

 - Record the current tree memory size _M_.
 - Arrange to write future patch frames
   to both the _current_ tree file and the _next_ tree file.
   In the _next_ tree, the patches start at the offset where
   a tree of size _M_ would end.
 - Each time a new megabyte (or other chosen chunk size) of patches
   is written, write the next megabyte of tree memory to the
   _next_ tree file as well.
 - Once all the tree memory has been written to the _next_ file,
   sync it to disk and write its incremented tree sequence number
   so that a future open will choose that file.
 - Now the old _next_ file has become the new _current_ one, and the
   old _current_ one will be reused for as _next_ for the next
   compaction.

Note that the compaction is concurrent but not parallel:
the compaction writes are interleaved with non-compaction writes,
not run in a separate goroutine.

Suppose we start a compaction when the in-memory tree is _M_ bytes
and the _current_ tree file is 2*M* bytes long.
Compaction will finish when another _M_ bytes of patches have been written,
meaning the _current_ file will be 3*M* bytes when it is retired,
and the _next_ tree file will be 2*M* bytes long.
After installing _next_ as the new _current_, it will be time for
a new compaction.
The result is continuous, just-in-time compaction: each one finishes
exactly as the next needs to begin.

For the actual implementation,
it seemed safer to write two megabytes of tree for
each megabyte of patches, rather than dance on that knife's edge.
If writing a megabyte of patches to two different trees
(_current_ and _next_) allows writing two megabytes of tree memory as well,
then the implementation writes the same amount of tree bytes and patch bytes to disk
when compaction is running.
At worst, compaction is always running, so that the number of tree
bytes written equals the number of patch bytes written.

If each memory change is written to two patch files,
and those writes justify writing the same amount of tree bytes,
then the [write amplification](https://en.wikipedia.org/wiki/Write_amplification)
factor is no worse than 4.
This constant factor is much better than the _O_(log _N_) amplification in
[log-structured merge trees](https://en.wikipedia.org/wiki/Log-structured_merge-tree).
The improvement is possible because we keep all the data in memory at all times.

## Speed {#speed}

On my circa-2023 home server with 128 GB of RAM
and an NVMe disk using LVM encryption, storing 834 million hashes
takes about 250 minutes, or about 55,000 Set operations per second.
This is with constant disk compaction, and I suspect something in my
kernel stack of slowing disk I/O.

A “lazy hash” optimization that delays recomputing all inner node hashes
is delayed until the Sync operation can avoid spending time
computing hashes that will be overwritten by a subsequent Set,
but it dramatically increases the latency of Sync.
More important than not computing the hashes is not writing
them to disk, especially for the somewhat special case of writing all new entries
when populating a new tree.
In that case, the lazy hash's lower disk usage
also avoids any compaction: only new data is
being written, so the disk file never reaches twice the memory size.
In that case, the 834 million hashes can be written in 45 minutes,
followed by a 7 minute sync, or about 260,000 Set operations second.

A limited lazy hash that is lazy only up to a fixed number of
Set operations may be the best of both worlds.

Prove operations run in microseconds.

Snap is effectively free.

## Recovery {#recovery}

On crash and restart, the persistent disk implementation guarantees
to have a state equivalent to some prefix of the operations that had
been executed prior to the crash. That is, some recent operations may
have been lost, but if operation K is observed, then all operations
prior to K will also be observed.

If the client calls Snap with a version number at regular intervals,
then after a crash, if the tree loads with version V, it means that
all of the Set calls before Snap(V) has been retained, and some of the Set
calls between Snap(V) and Snap(V+1) may also have been retained.
It suffices to replay all the Set operations between Snap(V) and Snap(V+1)
and then Snap(V+1) to get a consistent tree.
