package casetransform

import (
	"bytes"
	"testing"
	"testing/quick"
)

func benchBytesToLower(b *testing.B, src []byte) {
	dst := make([]byte, len(src))
	b.ResetTimer()
	for range b.N {
		BytesToLowerASCII(dst, src)
	}
}

func BenchmarkBytesToLowerASCII(b *testing.B) {
	b.Run("short", func(b *testing.B) { benchBytesToLower(b, []byte("a-z@[A-Z")) })
	b.Run("pangram", func(b *testing.B) { benchBytesToLower(b, []byte("\tThe Quick Brown Fox juMPs over the LAZY dog!?")) })
	long := bytes.Repeat([]byte{'A'}, 8*1024)
	b.Run("8k", func(b *testing.B) { benchBytesToLower(b, long) })
	b.Run("8k-misaligned", func(b *testing.B) { benchBytesToLower(b, long[1:]) })
}

func checkBytesToLower(t *testing.T, b []byte) {
	t.Helper()
	want := make([]byte, len(b))
	bytesToLowerASCIIgeneric(want, b)
	got := make([]byte, len(b))
	BytesToLowerASCII(got, b)
	if !bytes.Equal(want, got) {
		t.Errorf("BytesToLowerASCII(%q)=%q want %q", b, got, want)
	}
}

func TestBytesToLowerASCII(t *testing.T) {
	// @ and [ are special: '@'+1=='A' and 'Z'+1=='['
	t.Run("pangram", func(t *testing.T) {
		checkBytesToLower(t, []byte("\t[The Quick Brown Fox juMPs over the LAZY dog!?@"))
	})
	t.Run("short", func(t *testing.T) {
		checkBytesToLower(t, []byte("a-z@[A-Z"))
	})
	t.Run("quick", func(t *testing.T) {
		f := func(b []byte) bool {
			x := make([]byte, len(b))
			bytesToLowerASCIIgeneric(x, b)
			y := make([]byte, len(b))
			BytesToLowerASCII(y, b)
			return bytes.Equal(x, y)
		}
		if err := quick.Check(f, nil); err != nil {
			t.Error(err)
		}
	})
	t.Run("alignment", func(t *testing.T) {
		// The goal of this test is to make sure we don't write to any bytes
		// that don't belong to us.
		b := make([]byte, 96)
		c := make([]byte, 96)
		for i := range len(b) {
			for j := i; j < len(b); j++ {
				// fill b with Ms and c with xs
				for k := range b {
					b[k] = 'M'
					c[k] = 'x'
				}
				// process a subslice of b
				BytesToLowerASCII(c[i:j], b[i:j])
				for k := range b {
					want := byte('m')
					if k < i || k >= j {
						want = 'x'
					}
					if want != c[k] {
						t.Errorf("BytesToLowerASCII bad byte using bounds [%d:%d] (len %d) at index %d, have %c want %c", i, j, len(c[i:j]), k, c[k], want)
					}
				}
			}
		}
	})
}
