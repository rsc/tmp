This is a copy of github.com/google/brotli revision 509d4419bd (Dec 22 2022)
adjusted to build as a single Go package (without pre-installed C libraries).
All the files from brotli/c/{enc,dec,common}/*.[ch] and go/*.go were copied
to one directory and renamed to avoid collisions, with #includes updated.

cmd/brotli is a simple compression program for testing.
