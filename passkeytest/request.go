package passkeytest

import (
	"encoding/binary"
	"errors"
	"fmt"
	"slices"
	"time"

	"filippo.io/passkey"
)

// requestLen is the length of a login request: a version byte, the
// 32-byte challenge, and the creation time as eight big-endian bytes.
//
// Not a format this package is entitled to know, which is why
// SetRequestCreation checks its work with passkey.RequestCreation on the
// way in and on the way out. A version byte the passkey package no
// longer recognizes, or a request that has grown a field, fails there
// rather than quietly writing eight bytes over something else.
const requestLen = 1 + 32 + 8

// timeOffset is where the creation time begins.
const timeOffset = 1 + 32

// SetRequestCreation returns a copy of a login request, as returned by
// [passkey.RelyingParty.NewLogin], with its creation time set to t.
//
// A request records when it was made, and Login refuses one older than
// the relying party's timeout. That is what makes a recorded ceremony --
// a request, and the response a real browser gave to it -- stop
// replaying the day after it was recorded, and no timeout is long enough
// to wait out: the package refuses one over about seven weeks.
// Restamping the request is what keeps a recording usable, and setting
// the stamp far enough back is how a test reaches
// [passkey.ErrRequestExpired] with a response a real client produced.
//
// The request is not authenticated. An application is expected to
// protect it wherever it stores it, so rewriting one here is only doing
// what that storage is trusted to prevent anyone else from doing.
func SetRequestCreation(request []byte, t time.Time) ([]byte, error) {
	if passkey.RequestCreation(request).IsZero() {
		return nil, errors.New("passkeytest: not a valid login request")
	}
	if len(request) != requestLen {
		return nil, fmt.Errorf("passkeytest: login request is %d bytes, want %d", len(request), requestLen)
	}
	if t.Unix() < 0 {
		return nil, fmt.Errorf("passkeytest: creation time %v is before 1970", t)
	}

	out := slices.Clone(request)
	binary.BigEndian.PutUint64(out[timeOffset:], uint64(t.Unix()))

	// Read it back the way the passkey package will. The time is kept to
	// the second, so this compares seconds rather than instants.
	if got := passkey.RequestCreation(out); got.Unix() != t.Unix() {
		return nil, fmt.Errorf("passkeytest: set creation time to %v, but the request reports %v", t, got)
	}
	return out, nil
}
