## Radix Sort Experiment

This package holds an experiment in using radix sort for string sorting,
following Peter McIlroy, Keith Bostic, and Doug McIlroy's 1993 _Computing Systems_ paper
“[Engineering Radix Sort](https://www.usenix.org/legacy/publications/compsystems/1993/win_mcilroy.pdf)”.
Thanks to Don Caldwell and John P. Linderman for suggesting this experiment,
which was a lot of fun.

In theory, radix sort over _N_ strings of length _L_ runs in at most _O_(_N_ × _L_) time,
while any comparison-based sort must run in at least _O_(_N_ log _N_ × _L_) time.
Asymptotically, radix sort should have the edge over the comparison-based
sort that Go's `slices.Sort` and `sort.Strings` have historically used (and still use today).

Most presentations of radix sort assume allocation of a second copy of the array,
which would make it unsuitable for use in a library function,
but the McIlroy-Bostic-McIlroy paper shows how to work without any
auxiliary storage.
This package implements both algorithms, as `sortWithTmp` and `sortInPlace`.

Despite the asymptotic superiority, the benchmarks in this package show
that radix sort loses to Go's standard library in most real-world cases.
The fundamental problem appears to be that radix sort's memory access
patterns are terrible for modern memory hardware,
so in practice radix sort wins only when _L_ is very small.

Radix sort looks at the first byte of every string once to build a table,
then looks at the first byte of every string again to move the strings
into the right places.
Then the same happens for the second byte of all the strings with the same
first letter, and so on.
The factor of _L_ in the radix sort _O_(_N_ × _L_) is worst case _L_ rounds
of this single-letter sorting,
making poor use of the memory fetches.
In contrast, the factor of _L_ in  _O_(_N_ log _N_ × _L_) is a worst case comparison of _L_
bytes of two strings during `strings.Compare`,
but those bytes are being accessed sequentially, making use of entire cache lines
and accessing memory in a prefetch-friendly order.
That factor of _L_ essentially disappears in real execution,
or at least the associated constant is much smaller than the one
attached to radix sort's _L_.

It is worth noting that although _L_ is bounded by the length of the input strings,
the effective _L_ is only the number of initial bytes needed to distinguish a pair of strings.
For example, if you make 2²⁰ random strings of length 1000 with
completely random bytes, _L_ is more like 1 than 1000,
because any pair of strings has a 255-in-256 chance of
being distinguished based on just their leading bytes.
Some pairs may need more, but the average over all pairs will be near 1.
For random strings, then, radix sort can be faster than `slices.Sort`.
The same turns out to be true for dictionary word lists
or lines in a book,
both of which also exhibit very high entropy in the leading bytes.
But when you consider the kinds of strings that most Go programs
actually sort, such as package names in a build or
even file names in a directory, the effective _L_ ends up large enough
that radix sort loses.

Here is a synthetic benchmark that generates random strings with
a given number of bits of entropy (`e=`) per input byte:

```
$ benchstat -col '/alg@(slices.Sort sortInPlace sortWithTmp)' bench.txt
goos: linux
goarch: amd64
pkg: rsc.io/tmp/rsort
cpu: AMD Ryzen 9 7950X 16-Core Processor
                                   │ slices.Sort  │              sortInPlace              │               sortWithTmp               │
                                   │    sec/op    │    sec/op     vs base                 │    sec/op      vs base                  │
Random/n=1048576/len=512/e=8         277.79m ± 3%   201.11m ± 4%   -27.60% (p=0.000 n=10)     97.40m ± 3%    -64.94% (p=0.000 n=10)
Random/n=1048576/len=512/e=7         284.55m ± 2%   196.81m ± 5%   -30.84% (p=0.000 n=10)     77.57m ± 5%    -72.74% (p=0.000 n=10)
Random/n=1048576/len=512/e=4          289.2m ± 3%    286.3m ± 4%         ~ (p=0.481 n=10)     129.6m ± 2%    -55.19% (p=0.000 n=10)
Random/n=1048576/len=512/e=2          285.4m ± 4%    424.1m ± 3%   +48.59% (p=0.000 n=10)     174.5m ± 1%    -38.88% (p=0.000 n=10)
Random/n=1048576/len=512/e=1          286.0m ± 2%    728.0m ± 4%  +154.59% (p=0.000 n=10)     254.4m ± 5%    -11.05% (p=0.000 n=10)
Random/n=1048576/len=512/e=1_2        306.6m ± 2%    778.6m ± 3%  +154.00% (p=0.000 n=10)     415.3m ± 3%    +35.47% (p=0.000 n=10)
Random/n=1048576/len=512/e=1_4        330.4m ± 3%   1008.7m ± 2%  +205.32% (p=0.000 n=10)     792.0m ± 4%   +139.75% (p=0.000 n=10)
Random/n=1048576/len=512/e=1_8        444.5m ± 2%   1289.5m ± 4%  +190.13% (p=0.000 n=10)    1482.4m ± 4%   +233.54% (p=0.000 n=10)
Random/n=1048576/len=512/e=1_16       577.8m ± 3%   1889.5m ± 3%  +227.03% (p=0.000 n=10)    2866.8m ± 2%   +396.17% (p=0.000 n=10)
```

For the high-entropy cases, radix sort is much faster.
The version that allocates a temporary array is 3X faster at sorting 2²⁰ strings.
But if you drop down to 4 bits of entropy per byte, radix sort drops to 2X faster;
at 1 bit per byte, it has dropped to only 10% faster;
and at 1/2 bit per byte, it falls behind.
The in-place radix sort falls behind even sooner.

Here are some benchmarks using the text of Newton's Opticks
as well as sorting the Project Gutenberg Webster's dictionary word list
and the Plan 9 `/lib/words` file
(all randomly shuffled before the sort).
All have enough leading entropy that radix sort wins:

```
                                   │ slices.Sort  │              sortInPlace              │               sortWithTmp               │
                                   │    sec/op    │    sec/op     vs base                 │    sec/op      vs base                  │
Text/input=opticks                    814.3µ ± 2%    337.1µ ± 1%   -58.61% (p=0.000 n=10)     379.1µ ± 1%    -53.44% (p=0.000 n=10)
Text/input=pgw                       14.004m ± 1%    5.637m ± 1%   -59.75% (p=0.000 n=10)     4.908m ± 1%    -64.95% (p=0.000 n=10)
Text/input=pgwlower                  14.227m ± 3%    5.720m ± 3%   -59.80% (p=0.000 n=10)     4.903m ± 0%    -65.54% (p=0.000 n=10)
Text/input=plan9words                 2.916m ± 2%    1.026m ± 2%   -64.81% (p=0.000 n=10)     1.041m ± 1%    -64.29% (p=0.000 n=10)
```


Real inputs tend to have larger effective _L_, making them run more like the low-entropy random strings.
Here are a few real inputs I collected that real programs might sort:

 - `filelist`, the list of all files in the Go distribution, all with a leading `go/` prefix; 14,305 lines
 - `gofiles`, the list of all files in my /Users/rsc/go, all with a leading `/Users/rsc/go/` prefix; 39,594 lines
 - `gomodcache`, the list of all files in my /Users/rsc/pkg (Go module cache), all with a leading `pkg/` prefix; 201,210 lines
 - `kubedeps`, the list of all Kubernetes dependency packages; 3,025 lines
 - `runtimefiles`, the list of files in /Users/rsc/go/src/runtime, with no common prefix; 739 lines
 - `stdcmd`, approximately `go list std cmd | grep -v vendor`; 488 lines of relatively short package paths

Running shuffled versions of these inputs as benchmarks:

```
                                   │ slices.Sort  │              sortInPlace              │               sortWithTmp               │
                                   │    sec/op    │    sec/op     vs base                 │    sec/op      vs base                  │
Text/input=filelist                   1.570m ± 2%    1.142m ± 1%   -27.27% (p=0.000 n=10)     1.384m ± 0%    -11.85% (p=0.000 n=10)
Text/input=gofiles                    4.804m ± 1%    4.582m ± 3%    -4.61% (p=0.000 n=10)     6.575m ± 2%    +36.88% (p=0.000 n=10)
Text/input=gomodcache                 33.69m ± 1%    40.71m ± 3%   +20.82% (p=0.000 n=10)     45.91m ± 2%    +36.28% (p=0.000 n=10)
Text/input=kubedeps                   239.5µ ± 1%    187.9µ ± 2%   -21.52% (p=0.000 n=10)     253.7µ ± 1%     +5.95% (p=0.000 n=10)
Text/input=runtimefiles               22.66µ ± 2%    17.89µ ± 4%   -21.07% (p=0.000 n=10)     19.76µ ± 1%    -12.82% (p=0.000 n=10)
Text/input=stdcmd                     15.16µ ± 1%    18.07µ ± 2%   +19.20% (p=0.000 n=10)     22.26µ ± 0%    +46.89% (p=0.000 n=10)
```

The radix sort does run slightly faster in a few cases, but never overwhelmingly so. And it is often much slower.

It is also not terribly common to be asked to sort completely shuffled inputs.
The Go `slices.Sort` and `sort.Slices` both use [Pattern-Defeating Quicksort](https://arxiv.org/abs/2106.05123),
which runs even faster when the input is mostly in order.
These variants of the benchmarks start with an input that is mostly sorted
except for 10 random element pairs having been swapped:


```
                                   │ slices.Sort  │              sortInPlace              │               sortWithTmp               │
                                   │    sec/op    │    sec/op     vs base                 │    sec/op      vs base                  │
Text/input=opticks.sortish            209.8µ ± 2%    286.6µ ± 2%   +36.61% (p=0.000 n=10)     275.6µ ± 1%    +31.37% (p=0.000 n=10)
Text/input=pgw.sortish                3.934m ± 2%    4.534m ± 2%   +15.25% (p=0.000 n=10)     3.403m ± 1%    -13.51% (p=0.000 n=10)
Text/input=pgwlower.sortish           4.129m ± 2%    4.468m ± 1%    +8.21% (p=0.000 n=10)     3.421m ± 0%    -17.14% (p=0.000 n=10)
Text/input=plan9words.sortish         894.8µ ± 1%    912.6µ ± 3%         ~ (p=0.075 n=10)     775.8µ ± 1%    -13.29% (p=0.000 n=10)

Text/input=filelist.sortish           285.3µ ± 3%    914.2µ ± 2%  +220.44% (p=0.000 n=10)    1092.9µ ± 1%   +283.06% (p=0.000 n=10)
Text/input=gofiles.sortish            998.2µ ± 2%   3108.3µ ± 2%  +211.38% (p=0.000 n=10)    5240.4µ ± 4%   +424.97% (p=0.000 n=10)
Text/input=gomodcache.sortish         6.493m ± 2%   28.232m ± 3%  +334.82% (p=0.000 n=10)    37.908m ± 1%   +483.84% (p=0.000 n=10)
Text/input=kubedeps.sortish           63.87µ ± 2%   173.29µ ± 2%  +171.31% (p=0.000 n=10)    220.45µ ± 1%   +245.14% (p=0.000 n=10)
Text/input=runtimefiles.sortish       13.93µ ± 2%    16.59µ ± 2%   +19.14% (p=0.000 n=10)     15.82µ ± 0%    +13.59% (p=0.000 n=10)
Text/input=stdcmd.sortish             9.702µ ± 1%   17.086µ ± 2%   +76.11% (p=0.000 n=10)    20.186µ ± 1%   +108.06% (p=0.000 n=10)
```

Radix sort still wins, but not convincingly, on the high-entropy inputs.
And now it loses dramatically on the real-world inputs.

All in all, it seems like Go's `slices.Sort` and `sort.Strings`
should stick with a comparison-based sort like pattern-defeating quicksort.
