package passkeytest_test

import (
	"errors"
	"testing"
	"time"

	"filippo.io/passkey"
	"rsc.io/tmp/passkeytest"
)

// TestSetRequestCreation replays one ceremony twice: once with the
// request restamped as if it had just been made, which is what a
// recorded test does, and once with it restamped into the past, which is
// how a test reaches ErrRequestExpired with a real response.
func TestSetRequestCreation(t *testing.T) {
	rp := newRP(t, &passkey.Options{RPID: rpID, Origin: origin})
	a := passkeytest.New(&passkeytest.Options{Origin: origin})
	record, err := register(t, rp, a, nil)
	if err != nil {
		t.Fatal(err)
	}

	// One ceremony, kept the way a recording keeps it.
	request, optionsJSON, err := rp.NewLogin()
	if err != nil {
		t.Fatal(err)
	}
	responseJSON, err := a.Get(optionsJSON, userID)
	if err != nil {
		t.Fatal(err)
	}
	response, err := passkey.ParseResponse(responseJSON)
	if err != nil {
		t.Fatal(err)
	}

	fresh, err := passkeytest.SetRequestCreation(request, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rp.Login(response, fresh, []string{record}); err != nil {
		t.Errorf("Login() with a restamped request = %v", err)
	}

	stale, err := passkeytest.SetRequestCreation(request, time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rp.Login(response, stale, []string{record}); !errors.Is(err, passkey.ErrRequestExpired) {
		t.Errorf("Login() with a stale request = %v, want ErrRequestExpired", err)
	}

	// The request ID is the challenge, which restamping leaves alone, so
	// a recording can still be looked up by the key it was stored under.
	if got, want := passkey.RequestID(fresh), passkey.RequestID(request); got != want {
		t.Errorf("RequestID() = %q, want %q", got, want)
	}
	if got := passkey.RequestCreation(stale); time.Since(got) < 23*time.Hour {
		t.Errorf("RequestCreation() = %v, want about a day ago", got)
	}
}

func TestSetRequestCreationErrors(t *testing.T) {
	rp := newRP(t, &passkey.Options{RPID: rpID, Origin: origin})
	request, _, err := rp.NewLogin()
	if err != nil {
		t.Fatal(err)
	}

	for _, tt := range []struct {
		name    string
		request []byte
		time    time.Time
	}{
		{"nil", nil, time.Now()},
		{"empty", []byte{}, time.Now()},
		{"truncated", request[:len(request)-1], time.Now()},
		{"unknown version", append([]byte{2}, request[1:]...), time.Now()},
		{"before 1970", request, time.Unix(-1, 0)},
	} {
		if _, err := passkeytest.SetRequestCreation(tt.request, tt.time); err == nil {
			t.Errorf("SetRequestCreation(%s) succeeded, want an error", tt.name)
		}
	}
}
