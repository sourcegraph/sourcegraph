package embeddings

import (
	"math/rand"
	"testing"
	"testing/quick"
)

func TestDot(t *testing.T) {
	t.Run("edge cases", func(t *testing.T) {
		repeat := func(n int8, size int) []int8 {
			res := make([]int8, size)
			for i := range size {
				res[i] = n
			}
			return res
		}

		cases := []struct {
			a    []int8
			b    []int8
			want int32
		}{
			{[]int8{}, []int8{}, 0},
			{[]int8{1}, []int8{1}, 1},
			{append(repeat(0, 16), 1), append(repeat(0, 16), 2), 2},
			{
				[]int8{10, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
				[]int8{10, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2},
				102,
			},
			{repeat(0, 64), repeat(0, 64), 0},
			{repeat(0, 64), repeat(1, 64), 0},
			{repeat(1, 64), repeat(1, 64), 64},
			{repeat(1, 64), repeat(2, 64), 128},
			{repeat(-1, 64), repeat(1, 64), -64},
			{repeat(1, 65), repeat(1, 65), 65},

			// A couple of large ones to ensure no weird behavior at scale
			{repeat(1, 1000000), repeat(1, 1000000), 1000000},
			{repeat(1, 1000000), repeat(2, 1000000), 2000000},

			// This will come very close to overflowing an int32.
			// Make sure nothing crashes.
			{repeat(127, 133000), repeat(127, 133000), 2145157000},

			// This will overflow an int32 and return garbage.
			// Just make sure nothing crashes.
			{repeat(127, 134000), repeat(127, 134000), -2133681296},

			// These will overflow if we don't multiply into larger ints
			{repeat(127, 100), repeat(127, 100), 1612900},
			{repeat(-128, 100), repeat(-128, 100), 1638400},
			{repeat(-128, 100), repeat(127, 100), -1625600},
		}

		for _, tc := range cases {
			t.Run("dot", func(t *testing.T) {
				got := Dot(tc.a, tc.b)
				if tc.want != got {
					t.Fatalf("want: %d, got: %d", tc.want, got)
				}
			})

			t.Run("naive", func(t *testing.T) {
				got := dotNaive(tc.a, tc.b)
				if tc.want != got {
					t.Fatalf("want: %d, got: %d", tc.want, got)
				}
			})
		}
	})

	t.Run("quick", func(t *testing.T) {
		err := quick.Check(func(a, b []int8) bool {
			if len(a) > len(b) {
				a = a[:len(b)]
			} else {
				b = b[:len(a)]
			}

			want := dotNaive(a, b)
			got := Dot(a, b)

			if want != got {
				t.Fatalf("a: %#v\nb: %#v\ngot: %d\nwant: %d", a, b, got, want)
			}
			return want == got
		}, nil)
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("random", func(t *testing.T) {
		for range 1000 {
			size := rand.Int() % 1000
			a, b := make([]int8, size), make([]int8, size)

			randBytes := make([]byte, size)
			rand.Read(randBytes)
			for i, randByte := range randBytes {
				a[i] = int8(randByte)
			}
			rand.Read(randBytes)
			for i, randByte := range randBytes {
				b[i] = int8(randByte)
			}

			want := dotNaive(a, b)
			got := Dot(a, b)

			if want != got {
				t.Fatalf("a: %#v\nb: %#v\ngot: %d\nwant: %d", a, b, got, want)
			}
		}
	})
}

func dotNaive(a, b []int8) int32 {
	sum := int32(0)
	for i := range len(a) {
		sum += int32(a[i]) * int32(b[i])
	}
	return sum
}
