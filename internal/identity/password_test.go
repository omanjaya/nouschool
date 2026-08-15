package identity

import (
	"strings"
	"testing"
)

func TestHashPasswordAndVerify(t *testing.T) {
	hash, err := HashPassword("rahasia123")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$m=65536,t=1,p=4$") {
		t.Fatalf("format hash tidak sesuai: %s", hash)
	}

	ok, err := VerifyPassword(hash, "rahasia123")
	if err != nil {
		t.Fatalf("VerifyPassword (benar): %v", err)
	}
	if !ok {
		t.Fatal("password benar seharusnya valid")
	}

	ok, err = VerifyPassword(hash, "salah")
	if err != nil {
		t.Fatalf("VerifyPassword (salah): %v", err)
	}
	if ok {
		t.Fatal("password salah seharusnya tidak valid")
	}
}

func TestHashPasswordUniqueSalt(t *testing.T) {
	h1, err := HashPassword("sama-sama")
	if err != nil {
		t.Fatal(err)
	}
	h2, err := HashPassword("sama-sama")
	if err != nil {
		t.Fatal(err)
	}
	if h1 == h2 {
		t.Fatal("dua hash dari password sama harus berbeda (salt acak)")
	}
}

func TestVerifyPasswordInvalidHashFormat(t *testing.T) {
	_, err := VerifyPassword("bukan-hash-valid", "apapun")
	if err == nil {
		t.Fatal("expected error untuk format hash tidak valid")
	}
}
