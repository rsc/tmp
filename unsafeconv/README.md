
	$ go build
	$ ./unsafeconv std \
		github.com/boltdb/bolt... \
		github.com/google/gopacket... \
		golang.org/x/net... \
		golang.org/x/sys... \
		golang.org/x/tools... >output.txt

	$ awk '$2~/convert/ {print $2, $NF}' output.txt | sort | uniq -c
	   4 array-convert addr-of-index-of-non-slice
	  33 array-convert addr-of-non-index
	  58 array-convert non-addr-of
	   8 array-convert valid
	  25 slice-convert slice-elem-mismatch
	  10 slice-convert valid

