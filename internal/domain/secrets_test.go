package domain

import (
	"bytes"
	"testing"
)

func TestTokenCipherRoundTripsAndNeverStoresPlaintext(t *testing.T) {
	c, err := newTokenCipher("some-secret-key")
	if err != nil {
		t.Fatal(err)
	}
	plain := []byte("glpat-abcdefghij1234567890")
	a, err := c.seal(plain)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := c.seal(plain)
	if bytes.Contains(a, plain) || bytes.Contains(a, plain[:10]) {
		t.Fatalf("sealed value contains the plaintext: %x", a)
	}
	if bytes.Equal(a, b) {
		t.Fatal("sealing twice gave identical output; the nonce must be random")
	}
	got, err := c.open(a)
	if err != nil || !bytes.Equal(got, plain) {
		t.Fatalf("open = %q, %v", got, err)
	}
}

func TestTokenCipherRejectsAWrongKeyAndTampering(t *testing.T) {
	c, _ := newTokenCipher("key-one")
	other, _ := newTokenCipher("key-two")
	sealed, _ := c.seal([]byte("token-value-123456"))
	if _, err := other.open(sealed); err == nil {
		t.Fatal("opened with the wrong key")
	}
	sealed[len(sealed)-1] ^= 1
	if _, err := c.open(sealed); err == nil {
		t.Fatal("opened a tampered value")
	}
}

func TestTokenCipherNeedsAKey(t *testing.T) {
	if _, err := newTokenCipher(""); err == nil {
		t.Fatal("empty secret key accepted")
	}
}
