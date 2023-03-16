//go:build ignore
// +build ignore

package main

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/sourcegraph/sourcegraph/internal/rbac"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

var (
	outputFile = flag.String("o", "", "output file")
	pkgName    = flag.String("pkg", "main", "Go package name")
)

func main() {
	flag.Parse()

	if *outputFile == "" {
		flag.Usage()
		os.Exit(1)
	}

	output, err := os.Create(*outputFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer output.Close()

	schema := rbac.RBACSchema

	var permissions = []types.Permission{}
	for _, namespace := range schema.Namespaces {
		for _, action := range namespace.Actions {
			permissions = append(permissions, types.Permission{
				Namespace: namespace.Name,
				Action:    action,
			})
		}
	}

	fmt.Fprintln(output, "// Code generated by yamldata. DO NOT EDIT.")
	fmt.Fprintln(output)
	fmt.Fprintf(output, "package %s\n", *pkgName)
	fmt.Fprintln(output)
	for _, permission := range permissions {
		dn := permission.DisplayName()
		fmt.Fprintln(output, fmt.Sprintf("const %sPermission string = \"%s\"", sentencizePermission(dn), dn))
	}
}

var zanzibarPermissionRegex = regexp.MustCompile("([A-Za-z]+)(?:_([A-Za-z]+))?#([A-Za-z]+)")

func sentencizePermission(permission string) string {
	separators := [2]string{"#", "_"}
	// Replace all separators with white spaces
	for _, sep := range separators {
		permission = strings.ReplaceAll(permission, sep, " ")
	}

	return toTitleCase(permission)
}

func toTitleCase(input string) string {
	words := strings.Fields(input)

	formattedWords := make([]string, len(words))

	for i, word := range words {
		formattedWords[i] = strings.Title(strings.ToLower(word))
	}

	return strings.Join(formattedWords, "")
}
