package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"math/big"

	"golang.org/x/crypto/argon2"
)

// Key hierarchy
//
//	user password --argon2id(auth_salt)--> auth_hash     stored; verifies login only
//	user password --argon2id(kek_salt)---> KEK           never stored
//	KEK          --AES-GCM--> wrapped_dek                stored
//	escrow pub   --X25519+AES-GCM--> escrow_dek          stored; lets the master key recover the DEK
//	DEK          --AES-GCM--> every folder name and password record
//
//	master key   --argon2id(master_salt)--> MK           never stored
//	MK           --AES-GCM--> escrow private key         stored
//
// The database alone can't decrypt anything. Recovering a user's DEK
// requires either their password or the master key.

const (
	argonTime    = 3
	argonMemory  = 64 * 1024 // KiB
	argonThreads = 2
	keyLen       = 32
	saltLen      = 16
)

func randomBytes(n int) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand never fails on supported platforms
	}
	return b
}

func deriveKey(secret string, salt []byte) []byte {
	return argon2.IDKey([]byte(secret), salt, argonTime, argonMemory, argonThreads, keyLen)
}

func checkPassword(password string, salt, hash []byte) bool {
	return subtle.ConstantTimeCompare(deriveKey(password, salt), hash) == 1
}

func subKey(secret []byte, info string) []byte {
	k, err := hkdf.Key(sha256.New, secret, nil, info, keyLen)
	if err != nil {
		panic(err)
	}
	return k
}

// seal encrypts with AES-256-GCM; the random nonce is prepended to the output.
// aad binds a ciphertext to the row it belongs to so rows can't be swapped.
func seal(key, plaintext, aad []byte) []byte {
	gcm := newGCM(key)
	nonce := randomBytes(gcm.NonceSize())
	return gcm.Seal(nonce, nonce, plaintext, aad)
}

func open(key, data, aad []byte) ([]byte, error) {
	gcm := newGCM(key)
	if len(data) < gcm.NonceSize() {
		return nil, errors.New("ciphertext too short")
	}
	return gcm.Open(nil, data[:gcm.NonceSize()], data[gcm.NonceSize():], aad)
}

func newGCM(key []byte) cipher.AEAD {
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err) // only fails on a wrong key length, which is a programming error
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		panic(err)
	}
	return gcm
}

// escrowSeal encrypts data to the escrow public key (ephemeral X25519 + AES-GCM),
// so the server can escrow a new user's DEK without knowing the master key.
func escrowSeal(pub *ecdh.PublicKey, data, aad []byte) ([]byte, error) {
	eph, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	shared, err := eph.ECDH(pub)
	if err != nil {
		return nil, err
	}
	ephPub := eph.PublicKey().Bytes()
	key := subKey(append(shared, ephPub...), "passman escrow")
	return append(ephPub, seal(key, data, aad)...), nil
}

func escrowOpen(priv *ecdh.PrivateKey, data, aad []byte) ([]byte, error) {
	if len(data) < 32 {
		return nil, errors.New("escrow data too short")
	}
	ephPub, err := ecdh.X25519().NewPublicKey(data[:32])
	if err != nil {
		return nil, err
	}
	shared, err := priv.ECDH(ephPub)
	if err != nil {
		return nil, err
	}
	key := subKey(append(shared, data[:32]...), "passman escrow")
	return open(key, data[32:], aad)
}

func fingerprint(dek []byte, password string) []byte {
	m := hmac.New(sha256.New, subKey(dek, "passman fingerprint"))
	m.Write([]byte(password))
	return m.Sum(nil)
}

// Master key alphabet: alphanumeric without look-alikes (0/O/o, 1/l/I) so it
// can be copied by hand. 20 characters ≈ 116 bits.
const masterAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnpqrstuvwxyz23456789"

func randomString(n int, alphabet string) string {
	out := make([]byte, n)
	max := big.NewInt(int64(len(alphabet)))
	for i := range out {
		v, err := rand.Int(rand.Reader, max)
		if err != nil {
			panic(err)
		}
		out[i] = alphabet[v.Int64()]
	}
	return string(out)
}

func randomIntn(n int) int {
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		panic(err)
	}
	return int(v.Int64())
}

func newUUID() string {
	b := randomBytes(16)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
