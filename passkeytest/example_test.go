package passkeytest_test

import (
	"crypto/rand"
	"fmt"
	"log"

	"filippo.io/passkey"
	"rsc.io/tmp/passkeytest"
)

// This example runs a registration and a login the way a sign-in
// server's tests would: the application's own code makes the options and
// verifies the responses, and the authenticator stands in for the
// browser and the passkey provider in between.
func Example() {
	const (
		rpID   = "example.com"
		origin = "https://example.com"
	)

	// User IDs are opaque and hold no personal information: they are
	// stored inside the authenticator and returned in every login, and
	// they can never change. An application makes one per account at
	// sign-up and maps it back to the account in its database.
	userID := rand.Text()

	rp, err := passkey.NewRelyingParty(&passkey.Options{RPID: rpID, Origin: origin})
	if err != nil {
		log.Fatal(err)
	}

	// A synced passkey on a phone or laptop, which is what almost every
	// real login is. The Options fields describe the ones that are not:
	// a security key with no PIN, a credential that cannot be synced.
	a := passkeytest.New(&passkeytest.Options{Origin: origin})

	// Registration. The server sends the options to the browser, which
	// passes them to navigator.credentials.create() and posts the
	// result back.
	creationOptions, err := rp.NewRegistration(passkey.User{ID: userID, Name: "gopher"}, nil)
	if err != nil {
		log.Fatal(err)
	}
	responseJSON, err := a.Create(creationOptions)
	if err != nil {
		log.Fatal(err)
	}
	record, err := rp.Register(responseJSON)
	if err != nil {
		log.Fatal(err)
	}
	// The record is stored for the user, handled like a password hash.
	fmt.Println("registered:", record[:len("$webauthn$v=1$")])

	// Login. The request is stored by the application, keyed by
	// passkey.RequestID, and deleted once it has been used.
	request, requestOptions, err := rp.NewLogin()
	if err != nil {
		log.Fatal(err)
	}
	responseJSON, err = a.Get(requestOptions, userID)
	if err != nil {
		log.Fatal(err)
	}
	response, err := passkey.ParseResponse(responseJSON)
	if err != nil {
		log.Fatal(err)
	}

	// The asserted user ID is attacker-controlled until Login succeeds.
	// It is good for looking up the user's records and nothing else.
	if response.UnauthenticatedUserID() != userID {
		log.Fatal("no account has that passkey user ID")
	}

	result, err := rp.Login(response, request, []string{record})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("matched record:", result.Matched)
	fmt.Println("user verified:", result.UserVerified)
	fmt.Println("backed up:", result.BackedUp)

	// Output:
	// registered: $webauthn$v=1$
	// matched record: 0
	// user verified: true
	// backed up: true
}
