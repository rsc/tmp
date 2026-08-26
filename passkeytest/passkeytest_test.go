package passkeytest_test

import (
	"errors"
	"testing"
	"time"

	"filippo.io/passkey"
	"rsc.io/tmp/passkeytest"
)

const (
	rpID   = "example.com"
	origin = "https://example.com"
	userID = "6ANEIYIVMOOJDMOBHNQ3QCJVGY" // as crypto/rand.Text would make one
)

func newRP(t *testing.T, opts *passkey.Options) *passkey.RelyingParty {
	t.Helper()
	rp, err := passkey.NewRelyingParty(opts)
	if err != nil {
		t.Fatal(err)
	}
	return rp
}

// register runs a registration ceremony and returns the passkey record,
// which is what an application stores.
func register(t *testing.T, rp *passkey.RelyingParty, a *passkeytest.Authenticator, have []string) (string, error) {
	t.Helper()
	optionsJSON, err := rp.NewRegistration(passkey.User{ID: userID, Name: "gopher"}, have)
	if err != nil {
		t.Fatal(err)
	}
	responseJSON, err := a.Create(optionsJSON)
	if err != nil {
		return "", err
	}
	return rp.Register(responseJSON)
}

// login runs a login ceremony against the given records.
func login(t *testing.T, rp *passkey.RelyingParty, a *passkeytest.Authenticator, records []string) (*passkey.LoginResult, error) {
	t.Helper()
	request, optionsJSON, err := rp.NewLogin()
	if err != nil {
		t.Fatal(err)
	}
	responseJSON, err := a.Get(optionsJSON, userID)
	if err != nil {
		return nil, err
	}
	response, err := passkey.ParseResponse(responseJSON)
	if err != nil {
		return nil, err
	}
	if got, want := response.RequestID(), passkey.RequestID(request); got != want {
		t.Errorf("Response.RequestID() = %q, want %q", got, want)
	}
	if got := response.UnauthenticatedUserID(); got != userID {
		t.Errorf("UnauthenticatedUserID() = %q, want %q", got, userID)
	}
	return rp.Login(response, request, records)
}

// TestRoundTrip registers a passkey and signs in with it, which is the
// whole of what an application does.
func TestRoundTrip(t *testing.T) {
	rp := newRP(t, &passkey.Options{RPID: rpID, Origin: origin})
	a := passkeytest.New(&passkeytest.Options{Origin: origin})

	record, err := register(t, rp, a, nil)
	if err != nil {
		t.Fatalf("Register() = %v", err)
	}
	if ok, err := passkey.UserVerificationAvailable(record); err != nil || !ok {
		t.Fatalf("UserVerificationAvailable() = %v, %v, want true", ok, err)
	}

	result, err := login(t, rp, a, []string{record})
	if err != nil {
		t.Fatalf("Login() = %v", err)
	}
	if result.Matched != 0 || !result.UserVerified || !result.BackedUp {
		t.Errorf("LoginResult = %+v, want matched, verified, backed up", result)
	}
}

// TestMatching gives an account several passkeys and checks that each
// login is verified against the right one, which is what the storage
// model asks of Login.
func TestMatching(t *testing.T) {
	rp := newRP(t, &passkey.Options{RPID: rpID, Origin: origin})

	var records []string
	var auths []*passkeytest.Authenticator
	for i := range 3 {
		a := passkeytest.New(&passkeytest.Options{Origin: origin})
		record, err := register(t, rp, a, records)
		if err != nil {
			t.Fatalf("Register() %d = %v", i, err)
		}
		records = append(records, record)
		auths = append(auths, a)
	}

	for i, a := range auths {
		result, err := login(t, rp, a, records)
		if err != nil {
			t.Fatalf("Login() %d = %v", i, err)
		}
		if result.Matched != i {
			t.Errorf("Login() %d matched record %d", i, result.Matched)
		}
	}

	// An authenticator that already holds one of the excluded
	// credentials refuses to create another, so that a second
	// registration on the same device does not leave a duplicate.
	if _, err := register(t, rp, auths[0], records); err == nil {
		t.Error("registration excluding the authenticator's own credential succeeded")
	}
}

// TestUnknownCredential covers a passkey deleted from the account
// between the credential picker and the POST.
func TestUnknownCredential(t *testing.T) {
	rp := newRP(t, &passkey.Options{RPID: rpID, Origin: origin})
	a := passkeytest.New(&passkeytest.Options{Origin: origin})
	if _, err := register(t, rp, a, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := login(t, rp, a, nil); !errors.Is(err, passkey.ErrUnknownCredential) {
		t.Errorf("Login() = %v, want ErrUnknownCredential", err)
	}
}

// TestRequestExpired covers a conditional UI prompt left sitting until
// the request aged out, which the application retries.
func TestRequestExpired(t *testing.T) {
	rp := newRP(t, &passkey.Options{RPID: rpID, Origin: origin})
	a := passkeytest.New(&passkeytest.Options{Origin: origin})
	record, err := register(t, rp, a, nil)
	if err != nil {
		t.Fatal(err)
	}

	// The request records its creation time to the second, so any
	// positive timeout this short has already passed.
	expiring := newRP(t, &passkey.Options{RPID: rpID, Origin: origin, Timeout: time.Nanosecond})
	if _, err := login(t, expiring, a, []string{record}); !errors.Is(err, passkey.ErrRequestExpired) {
		t.Errorf("Login() = %v, want ErrRequestExpired", err)
	}
}

// TestUserVerificationUnavailable covers a security key with no PIN:
// neither the UV nor the BE flag, so it could never log in, and
// registration says so rather than storing a record that cannot be used.
func TestUserVerificationUnavailable(t *testing.T) {
	rp := newRP(t, &passkey.Options{RPID: rpID, Origin: origin})
	a := passkeytest.New(&passkeytest.Options{
		Origin:             origin,
		NoUserVerification: true,
		NotBackupEligible:  true,
	})
	if _, err := register(t, rp, a, nil); !errors.Is(err, passkey.ErrUserVerificationUnavailable) {
		t.Errorf("Register() = %v, want ErrUserVerificationUnavailable", err)
	}

	// With the requirement off, the same key registers and logs in,
	// reporting that nothing was verified.
	optional := newRP(t, &passkey.Options{RPID: rpID, Origin: origin, OptionalUserVerification: true})
	record, err := register(t, optional, a, nil)
	if err != nil {
		t.Fatalf("Register() with OptionalUserVerification = %v", err)
	}
	result, err := login(t, optional, a, []string{record})
	if err != nil {
		t.Fatalf("Login() with OptionalUserVerification = %v", err)
	}
	if result.UserVerified || result.BackedUp {
		t.Errorf("LoginResult = %+v, want neither verified nor backed up", result)
	}
}

// TestDeviceBound covers a passkey that cannot be synced: verified, so
// it is allowed, but never backed up.
func TestDeviceBound(t *testing.T) {
	rp := newRP(t, &passkey.Options{RPID: rpID, Origin: origin})
	a := passkeytest.New(&passkeytest.Options{Origin: origin, NotBackupEligible: true})
	record, err := register(t, rp, a, nil)
	if err != nil {
		t.Fatal(err)
	}
	result, err := login(t, rp, a, []string{record})
	if err != nil {
		t.Fatalf("Login() = %v", err)
	}
	if !result.UserVerified || result.BackedUp {
		t.Errorf("LoginResult = %+v, want verified and not backed up", result)
	}
}

// TestNotBackedUp covers a syncable passkey that is not in the cloud yet.
func TestNotBackedUp(t *testing.T) {
	rp := newRP(t, &passkey.Options{RPID: rpID, Origin: origin})
	a := passkeytest.New(&passkeytest.Options{Origin: origin, NotBackedUp: true})
	record, err := register(t, rp, a, nil)
	if err != nil {
		t.Fatal(err)
	}
	result, err := login(t, rp, a, []string{record})
	if err != nil {
		t.Fatalf("Login() = %v", err)
	}
	if !result.UserVerified || result.BackedUp {
		t.Errorf("LoginResult = %+v, want verified and not backed up", result)
	}
}

// TestNoUserPresence covers a response no real client produces, which is
// refused before anything else is looked at.
func TestNoUserPresence(t *testing.T) {
	rp := newRP(t, &passkey.Options{RPID: rpID, Origin: origin})
	a := passkeytest.New(&passkeytest.Options{Origin: origin, NoUserPresence: true})
	record, err := register(t, rp, a, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := login(t, rp, a, []string{record}); err == nil {
		t.Error("Login() with no user presence succeeded")
	}
}

// TestWrongOrigin covers a ceremony performed somewhere else, which is
// the check that keeps a passkey from being used by a phishing page.
func TestWrongOrigin(t *testing.T) {
	rp := newRP(t, &passkey.Options{RPID: rpID, Origin: origin})
	good := passkeytest.New(&passkeytest.Options{Origin: origin})
	record, err := register(t, rp, good, nil)
	if err != nil {
		t.Fatal(err)
	}

	elsewhere := passkeytest.New(&passkeytest.Options{Origin: "https://evil.example"})
	if _, err := register(t, rp, elsewhere, nil); err == nil {
		t.Error("Register() from another origin succeeded")
	}
	if _, err := login(t, rp, elsewhere, []string{record}); err == nil {
		t.Error("Login() from another origin succeeded")
	}
}

// TestUserScoped covers a ceremony for a user the application has
// already identified, as in a re-authentication prompt: the client is
// offered only that user's credentials, and the response carries no user
// handle because nobody needs to be looked up.
func TestUserScoped(t *testing.T) {
	rp := newRP(t, &passkey.Options{RPID: rpID, Origin: origin})
	a := passkeytest.New(&passkeytest.Options{Origin: origin})
	record, err := register(t, rp, a, nil)
	if err != nil {
		t.Fatal(err)
	}

	records := []string{record}
	request, optionsJSON, err := rp.NewLoginWithOptions(&passkey.LoginOptions{AllowCredentials: records})
	if err != nil {
		t.Fatal(err)
	}
	responseJSON, err := a.Get(optionsJSON, "")
	if err != nil {
		t.Fatalf("Get() = %v", err)
	}
	response, err := passkey.ParseResponse(responseJSON)
	if err != nil {
		t.Fatal(err)
	}
	if got := response.UnauthenticatedUserID(); got != "" {
		t.Errorf("UnauthenticatedUserID() = %q, want empty", got)
	}
	if _, err := rp.Login(response, request, records); err != nil {
		t.Fatalf("Login() = %v", err)
	}

	// A second authenticator is not offered the credential, and a client
	// that holds none of the allowed ones does not sign anything.
	other := passkeytest.New(&passkeytest.Options{Origin: origin})
	if _, err := register(t, rp, other, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := other.Get(optionsJSON, ""); err == nil {
		t.Error("Get() with no allowed credential succeeded")
	}
}

// TestCredentialID checks the identifier the application may keep for a
// passkey management UI.
func TestCredentialID(t *testing.T) {
	rp := newRP(t, &passkey.Options{RPID: rpID, Origin: origin})
	a := passkeytest.New(&passkeytest.Options{Origin: origin})
	if a.CredentialID() != nil {
		t.Error("CredentialID() is set before Create")
	}
	if _, err := register(t, rp, a, nil); err != nil {
		t.Fatal(err)
	}
	if len(a.CredentialID()) == 0 {
		t.Error("CredentialID() is empty after Create")
	}
	if _, err := a.Get(nil, userID); err == nil {
		t.Error("Get() with malformed options succeeded")
	}
}

// TestAAGUID checks that the AAGUID reaches the record, where an
// application reads it to name the passkey provider.
func TestAAGUID(t *testing.T) {
	rp := newRP(t, &passkey.Options{RPID: rpID, Origin: origin})
	want := [16]byte{0xad, 0xce, 0x00, 0x02, 0x35, 0xbc, 0xc6, 0x0a}
	a := passkeytest.New(&passkeytest.Options{Origin: origin, AAGUID: want})
	record, err := register(t, rp, a, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := passkey.AAGUID(record)
	if err != nil || got != want {
		t.Errorf("AAGUID() = %x, %v, want %x", got, err, want)
	}
}

// TestNoCredential checks that a login before any registration says so
// rather than signing with a nil key.
func TestNoCredential(t *testing.T) {
	rp := newRP(t, &passkey.Options{RPID: rpID, Origin: origin})
	_, optionsJSON, err := rp.NewLogin()
	if err != nil {
		t.Fatal(err)
	}
	a := passkeytest.New(&passkeytest.Options{Origin: origin})
	if _, err := a.Get(optionsJSON, userID); err == nil {
		t.Error("Get() before Create succeeded")
	}
}
