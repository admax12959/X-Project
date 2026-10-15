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

func TestFieldSecretsAreBoundToTheirFieldAndRecordTheKeyVersion(t *testing.T) {
	c, _ := newTokenCipher("some-secret-key")
	c.version = 3
	sealed, version, err := c.sealField("ldap", "bindPassword", []byte("pw-123456"))
	if err != nil || version != 3 {
		t.Fatalf("sealField = %v, version %d", err, version)
	}
	if bytes.Contains(sealed, []byte("pw-123456")) {
		t.Fatal("sealed value contains the plaintext")
	}
	if got, err := c.openField("ldap", "bindPassword", sealed, 3); err != nil || string(got) != "pw-123456" {
		t.Fatalf("openField = %q, %v", got, err)
	}
	if _, err := c.openField("smtp", "bindPassword", sealed, 3); err == nil {
		t.Fatal("opened under another section")
	}
	if _, err := c.openField("ldap", "password", sealed, 3); err == nil {
		t.Fatal("opened under another field")
	}
	if _, err := c.openField("ldap", "bindPassword", sealed, 2); err == nil {
		t.Fatal("opened under another key version")
	}
}

func TestTokenCipherNeedsAKey(t *testing.T) {
	if _, err := newTokenCipher(""); err == nil {
		t.Fatal("empty secret key accepted")
	}
}
