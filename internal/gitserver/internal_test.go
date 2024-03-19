package gitserver

import (
	"fmt"
	"testing"
)

func BenchmarkAddrForKey(b *testing.B) {
	for _, count := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("Count-%d", count), func(b *testing.B) {
			var nodes []string
			for i := range count {
				nodes = append(nodes, fmt.Sprintf("Node%d", i))
			}
			b.ResetTimer()
			for range b.N {
				addrForKey("foo", nodes)
			}
		})
	}
}
