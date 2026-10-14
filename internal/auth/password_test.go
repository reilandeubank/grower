package auth

import (
	"errors"
	"strings"
	"testing"
)

// cheapParams keep tests fast; VerifyPassword reads parameters from the hash.
var cheapParams = HashParams{MemoryKiB: 64, Iterations: 1, Parallelism: 1, SaltLen: 16, KeyLen: 32}

func TestHashAndVerify(t *testing.T) {
	hash, err := HashPassword("correct horse battery", cheapParams)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$m=64,t=1,p=1$") {
		t.Errorf("unexpected PHC prefix: %s", hash)
	}
	other, _ := HashPassword("correct horse battery", cheapParams)
	if other == hash {
		t.Error("two hashes of the same password must differ (random salt)")
	}

	tests := []struct {
		name     string
		password string
		hash     string
		want     bool
		wantErr  error
	}{
		{"match", "correct horse battery", hash, true, nil},
		{"wrong", "correct horse batterY", hash, false, nil},
		{"empty", "", hash, false, nil},
		{"not phc", "x", "plaintext", false, ErrMalformedHash},
		{"wrong algo", "x", "$argon2i$v=19$m=64,t=1,p=1$c2FsdA$a2V5", false, ErrMalformedHash},
		{"bad params", "x", "$argon2id$v=19$m=0,t=1,p=1$c2FsdA$a2V5", false, ErrMalformedHash},
		{"bad version", "x", "$argon2id$v=16$m=64,t=1,p=1$c2FsdA$a2V5", false, ErrMalformedHash},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := VerifyPassword(tt.password, tt.hash)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("VerifyPassword = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDefaultParamsVerify(t *testing.T) {
	if testing.Short() {
		t.Skip("uses production argon2 cost")
	}
	hash, err := HashPassword("a long enough password", DefaultHashParams())
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := VerifyPassword("a long enough password", hash); !ok || err != nil {
		t.Fatalf("verify = %v, %v", ok, err)
	}
}
