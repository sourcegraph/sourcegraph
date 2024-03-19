package randstring

import "testing"

func validateChars(t *testing.T, u string, chars []byte) {
	for _, c := range u {
		var present bool
		for _, a := range chars {
			if rune(a) == c {
				present = true
			}
		}
		if !present {
			t.Fatalf("chars not allowed in %q", u)
		}
	}
}

func TestNew_unique(t *testing.T) {
	// Generate 1000 strings and check that they are unique
	ss := make([]string, 1000)
	for i := range ss {
		ss[i] = NewLen(16)
	}
	for i, u := range ss {
		for j, u2 := range ss {
			if i != j && u == u2 {
				t.Fatalf("not unique: %d:%q and %d:%q", i, u, j, u2)
			}
		}
	}
}

func TestNewLen(t *testing.T) {
	for i := range 100 {
		u := NewLen(i)
		if len(u) != i {
			t.Fatalf("request length %d, got %d", i, len(u))
		}
	}
}

func TestNewLenChars(t *testing.T) {
	length := 10
	chars := []byte("01234567")
	u := NewLenChars(length, chars)

	// Check length
	if len(u) != length {
		t.Fatalf("wrong length: expected %d, got %d", length, len(u))
	}
	// Check that only allowed characters are present
	validateChars(t, u, chars)

	// Check that two generated strings are different
	u2 := NewLenChars(length, chars)
	if u == u2 {
		t.Fatalf("not unique: %q and %q", u, u2)
	}
}

func TestNewLenCharsMaxLength(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("didn't panic")
		}
	}()
	chars := make([]byte, 257)
	NewLenChars(32, chars)
}
