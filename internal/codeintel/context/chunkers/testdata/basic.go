// Package main docstring
package main

import "fmt"

// globalVarAlone docstring
var globalVarAlone = "bar"

var (
	// globalVarInGroup docstring
	globalVarInGroup = "foo"
	globalVarInGroup2 = "foo"
)

// globalConstAlone docstring
const globalConstAlone = 0

const (
	// globalConstInGroup docstring
	globalConstInGroup = 1
	globalConstInGroup2 = 1
)

// Foo docstring
type Foo struct {
	// Foo.Field docstring
	Field string
}

// Foo.Do() docstring
func (f *Foo) Do() {
	fmt.Println("Do")
}

// Foo.DoNonPointer() docstring
func (f Foo) DoNonPointer() {
	fmt.Println("DoNonPointer")
}

type Foo2 struct {
	Field2 string
}

func (f *Foo2) Do() {
	fmt.Println("Do")
}

func (f Foo2) DoNonPointer() {
	fmt.Println("DoNonPointer")
}

// Bar docstring
type Bar interface {
	// Baz docstring
	Baz()
	Baz2()
}

type Bar2 interface{}

type GenericBarContainer[T Bar2] struct {
	Bars T[]
}

func (b *GenericBarContainer[T]) barContainerDo() {
}

// main docstring
func main() {
	// inline comment
	fmt.Println("Hello world")
}

func main2() {}
