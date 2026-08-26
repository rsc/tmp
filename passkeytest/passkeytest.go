// Package passkeytest is a software authenticator, and enough of the
// client around it to run complete WebAuthn ceremonies against
// filippo.io/passkey without a browser.
//
// A browser test is what proves a sign-in page works. This is for the
// server side, where a real client is a slow and awkward way to ask what
// happens when a passkey is deleted between the picker and the POST.
//
// The credential is ES256. A client picks an algorithm out of the
// pubKeyCredParams the server offers, so this is a client that always
// picks that one.
//
// [SetRequestCreation] is for the other way to test a sign-in server:
// replaying ceremonies captured from a real browser, whose requests must
// be restamped to keep them from expiring.
package passkeytest

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
)

// Authenticator data flags, from WebAuthn 6.1.
const (
	flagUP byte = 1 << 0 // user present
	flagUV byte = 1 << 2 // user verified
	flagBE byte = 1 << 3 // backup eligible: a synced passkey
	flagBS byte = 1 << 4 // backed up right now
	flagAT byte = 1 << 6 // attested credential data present
)

// COSE constants, from RFC 9052 and RFC 9053.
const (
	coseKeyTypeEC2 = 2
	coseCurveP256  = 1
	algES256       = -7
)

// Options configures an Authenticator. The zero value describes an
// ordinary synced passkey on a phone or laptop: user verified, backup
// eligible, and backed up, which is what iCloud Keychain and Google
// Password Manager produce and what almost every real login will be.
type Options struct {
	// Origin is the origin the client claims the ceremony happened on.
	// It must match the RelyingParty's, except in the tests that are
	// checking what happens when it does not.
	Origin string

	// AAGUID identifies the passkey provider. Zero for a roaming
	// authenticator, which is what Chrome reports for a security key.
	AAGUID [16]byte

	// Transports are recorded in the passkey record at registration and
	// handed back to clients in credential descriptors.
	Transports []string

	// NoUserVerification clears the UV flag: a security key with no PIN,
	// or a platform authenticator whose screen lock is off.
	NoUserVerification bool

	// NotBackupEligible clears BE and BS: a credential bound to one
	// device, which cannot be synced. A record with neither UV nor BE is
	// the one filippo.io/passkey refuses to register.
	NotBackupEligible bool

	// NotBackedUp clears BS alone: a syncable credential that is not in
	// the cloud yet.
	NotBackedUp bool

	// NoUserPresence clears UP, which no real client does. Logins from
	// such an authenticator are refused before the signature is checked.
	NoUserPresence bool
}

// An Authenticator holds one credential, created on the first call to
// Create. It is not safe for concurrent use.
type Authenticator struct {
	opts Options
	key  *ecdsa.PrivateKey
	id   []byte
}

// New returns an Authenticator that has not yet created a credential.
// opts may be nil, for the defaults described in Options.
func New(opts *Options) *Authenticator {
	a := new(Authenticator)
	if opts != nil {
		a.opts = *opts
	}
	if a.opts.Transports == nil {
		a.opts.Transports = []string{"internal", "hybrid"}
	}
	return a
}

// CredentialID returns the credential ID, which is nil until Create.
func (a *Authenticator) CredentialID() []byte { return a.id }

// flags returns the authenticator data flags for a response.
func (a *Authenticator) flags() byte {
	var f byte
	if !a.opts.NoUserPresence {
		f |= flagUP
	}
	if !a.opts.NoUserVerification {
		f |= flagUV
	}
	if !a.opts.NotBackupEligible {
		f |= flagBE
		if !a.opts.NotBackedUp {
			f |= flagBS
		}
	}
	return f
}

// Create answers a registration ceremony: it takes the options from
// RelyingParty.NewRegistration and returns a registration response, to
// be passed to RelyingParty.Register.
//
// It refuses, as a client would, when the options exclude the credential
// this authenticator already holds.
func (a *Authenticator) Create(optionsJSON []byte) ([]byte, error) {
	var o struct {
		RP struct {
			ID string `json:"id"`
		} `json:"rp"`
		Challenge          string `json:"challenge"`
		ExcludeCredentials []struct {
			ID string `json:"id"`
		} `json:"excludeCredentials"`
	}
	if err := json.Unmarshal(optionsJSON, &o); err != nil {
		return nil, fmt.Errorf("passkeytest: malformed creation options: %w", err)
	}
	for _, c := range o.ExcludeCredentials {
		if id, err := base64.RawURLEncoding.DecodeString(c.ID); err == nil && slices.Equal(id, a.id) {
			return nil, errors.New("passkeytest: credential is excluded (InvalidStateError)")
		}
	}
	if a.key == nil {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return nil, err
		}
		a.key, a.id = key, make([]byte, 32)
		rand.Read(a.id)
	}

	authData := a.authData(o.RP.ID, true)
	return json.Marshal(map[string]any{
		"id":                      base64.RawURLEncoding.EncodeToString(a.id),
		"rawId":                   base64.RawURLEncoding.EncodeToString(a.id),
		"type":                    "public-key",
		"authenticatorAttachment": "platform",
		"response": map[string]any{
			"clientDataJSON":     b64(a.clientDataJSON("webauthn.create", o.Challenge)),
			"authenticatorData":  b64(authData),
			"transports":         a.opts.Transports,
			"publicKeyAlgorithm": algES256,
		},
		"clientExtensionResults": map[string]any{
			"credProps": map[string]any{"rk": true},
		},
	})
}

// Get answers a login ceremony: it takes the options from
// RelyingParty.NewLogin and returns a login response, to be passed to
// RelyingParty.Login.
//
// userID is the passkey user ID the credential was registered for, which
// a real authenticator stores alongside it. It is sent as the user
// handle, and is empty for a response to a user-scoped ceremony.
//
// Get refuses, as a client would, when the options allow only
// credentials this authenticator does not hold.
func (a *Authenticator) Get(optionsJSON []byte, userID string) ([]byte, error) {
	if a.key == nil {
		return nil, errors.New("passkeytest: no credential; call Create first")
	}
	var o struct {
		RPID             string `json:"rpId"`
		Challenge        string `json:"challenge"`
		AllowCredentials []struct {
			ID string `json:"id"`
		} `json:"allowCredentials"`
	}
	if err := json.Unmarshal(optionsJSON, &o); err != nil {
		return nil, fmt.Errorf("passkeytest: malformed request options: %w", err)
	}
	if len(o.AllowCredentials) > 0 {
		ok := false
		for _, c := range o.AllowCredentials {
			if id, err := base64.RawURLEncoding.DecodeString(c.ID); err == nil && slices.Equal(id, a.id) {
				ok = true
			}
		}
		if !ok {
			return nil, errors.New("passkeytest: no allowed credential (NotAllowedError)")
		}
	}

	authData := a.authData(o.RPID, false)
	clientData := a.clientDataJSON("webauthn.get", o.Challenge)
	digest := sha256.Sum256(append(append([]byte{}, authData...), hash(clientData)...))
	sig, err := ecdsa.SignASN1(rand.Reader, a.key, digest[:])
	if err != nil {
		return nil, err
	}
	response := map[string]any{
		"clientDataJSON":    b64(clientData),
		"authenticatorData": b64(authData),
		"signature":         b64(sig),
	}
	if userID != "" {
		response["userHandle"] = b64([]byte(userID))
	}
	return json.Marshal(map[string]any{
		"id":                      base64.RawURLEncoding.EncodeToString(a.id),
		"rawId":                   base64.RawURLEncoding.EncodeToString(a.id),
		"type":                    "public-key",
		"authenticatorAttachment": "platform",
		"response":                response,
		"clientExtensionResults":  map[string]any{},
	})
}

// authData assembles authenticator data. Attested credential data -- the
// AAGUID, credential ID and public key -- is present only at
// registration, which is the one time the server is told the key.
func (a *Authenticator) authData(rpID string, attested bool) []byte {
	rpIDHash := sha256.Sum256([]byte(rpID))
	flags := a.flags()
	if attested {
		flags |= flagAT
	}
	b := append([]byte{}, rpIDHash[:]...)
	b = append(b, flags)
	b = append(b, 0, 0, 0, 0) // signature counter, zero as the real providers leave it
	if attested {
		b = append(b, a.opts.AAGUID[:]...)
		b = append(b, byte(len(a.id)>>8), byte(len(a.id)))
		b = append(b, a.id...)
		b = append(b, a.coseKey()...)
	}
	return b
}

// clientDataJSON serializes the client data. challenge is base64url, as
// it appears in the ceremony options.
func (a *Authenticator) clientDataJSON(ceremony, challenge string) []byte {
	data, err := json.Marshal(map[string]any{
		"type":        ceremony,
		"challenge":   challenge,
		"origin":      a.opts.Origin,
		"crossOrigin": false,
	})
	if err != nil {
		panic(err) // a map of strings and a bool
	}
	return data
}

// coseKey encodes the public key as a COSE_Key, in the CTAP2 canonical
// CBOR that lives inside attested credential data.
func (a *Authenticator) coseKey() []byte {
	point, err := a.key.PublicKey.Bytes()
	if err != nil {
		panic(err) // a P-256 key this package generated
	}
	b := cborArgument(nil, 5, 5) // a five-pair map
	b = cborInt(b, 1)
	b = cborInt(b, coseKeyTypeEC2)
	b = cborInt(b, 3)
	b = cborInt(b, algES256)
	b = cborInt(b, -1)
	b = cborInt(b, coseCurveP256)
	b = cborInt(b, -2)
	b = cborBytes(b, point[1:33])
	b = cborInt(b, -3)
	return cborBytes(b, point[33:65])
}

// cborArgument appends a CBOR head: a major type and its argument.
func cborArgument(b []byte, major byte, arg uint16) []byte {
	switch {
	case arg <= 23:
		return append(b, major<<5|byte(arg))
	case arg <= 0xff:
		return append(b, major<<5|24, byte(arg))
	default:
		return append(b, major<<5|25, byte(arg>>8), byte(arg))
	}
}

func cborInt(b []byte, v int32) []byte {
	if v < 0 {
		return cborArgument(b, 1, uint16(-(v + 1)))
	}
	return cborArgument(b, 0, uint16(v))
}

func cborBytes(b, v []byte) []byte {
	return append(cborArgument(b, 2, uint16(len(v))), v...)
}

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func hash(b []byte) []byte {
	sum := sha256.Sum256(b)
	return sum[:]
}
