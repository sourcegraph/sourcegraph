package maps

import "fmt"

func ExampleMerge() {
	m := Merge(map[string]int{"a": 1, "b": 2}, map[string]int{"b": 3, "c": 4})
	fmt.Printf("%#v\n", m)

	// Output:
	// map[string]int{"a":1, "b":3, "c":4}
}

func ExampleMergePreservingExistingKeys() {
	m := MergePreservingExistingKeys(map[string]int{"a": 1, "b": 2}, map[string]int{"b": 3, "c": 4})
	fmt.Printf("%#v\n", m)

	// Output:
	// map[string]int{"a":1, "b":2, "c":4}
}
