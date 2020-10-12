// +build generate

package main

import (
	"fmt"
	"io/ioutil"
	"log"
)

func main() {
	out, err := ioutil.ReadFile("schema.graphql")
	if err != nil {
		log.Fatal(err)
	}

	pre := `// +build !dev

package graphqlbackend

// Code generated by schema_generate.go

// Schema is the raw graqhql schema
`

	out = []byte(fmt.Sprintf("%svar Schema = `%s`\n", pre, string(out)))
	err = ioutil.WriteFile("schema.go", out, 0666)
	if err != nil {
		log.Fatal(err)
	}
}
