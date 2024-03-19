package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sourcegraph/sourcegraph/lib/codeintel/lsif/conversion"
	"github.com/sourcegraph/sourcegraph/lib/codeintel/precise"
	"github.com/sourcegraph/sourcegraph/lib/codeintel/precise/diff"
)

const helpMsg string = `
?                                                     help
count                                                 list dumps
paths <dump id>                                       list some paths from a dump
query <dump id> <path> <line> <column>                query dumps
index <path to repo root with dump.lsif>              index new dump
`

func main() {
	var bundles []*precise.GroupedBundleDataMaps

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Printf("\n> ")
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		switch fields[0] {
		case "count":
			fmt.Println(len(bundles))
		case "query":
			if len(fields) != 5 {
				fmt.Println("wrong number of args to q")
				break
			}
			uploadID, err := strconv.Atoi(fields[1])
			if err != nil {
				fmt.Println("first arg should be an integer")
				break
			}
			path := fields[2]
			line, err := strconv.Atoi(fields[3])
			if err != nil {
				fmt.Println("third arg should be an integer")
				break
			}
			column, err := strconv.Atoi(fields[4])
			if err != nil {
				fmt.Println("fourth arg should be an integer")
				break
			}

			err = queryBundle(bundles[uploadID], path, line, column)
			if err != nil {
				fmt.Printf("%s\n", helpMsg)
				break
			}

		case "paths":
			if len(fields) != 2 {
				fmt.Println("expected 1 argument to index")
				break
			}
			bundleID, err := strconv.Atoi(fields[1])
			if err != nil {
				fmt.Println("first argument should be int")
				break
			}

			idx := 0
			for path := range bundles[bundleID].Documents {
				fmt.Println(path)
				idx++
				if idx > 4 {
					break
				}
			}

		case "index":
			if len(fields) != 2 && len(fields) != 3 {
				fmt.Println("expected 1 or 2 arguments to index")
				break
			}
			dumpPath := fields[1]
			projectRoot := filepath.Dir(dumpPath)
			if len(fields) == 3 {
				projectRoot = fields[2]
			}

			bundle, err := conversion.CorrelateLocalGit(context.Background(), dumpPath, projectRoot)
			if err != nil {
				fmt.Printf("CorrelateLocalGit failed: %s", err)
				break
			}
			bundles = append(bundles, precise.GroupedBundleDataChansToMaps(bundle))
			fmt.Printf("finished indexing dump %v\n", len(bundles)-1)

		case "diff":
			if len(fields) != 3 {
				fmt.Printf("expected 2 arguments to diff, got %v\n", len(fields))
				break
			}
			gotID, err := strconv.Atoi(fields[1])
			if err != nil || gotID >= len(bundles) {
				fmt.Println("first argument should be bundle ID")
				break
			}
			wantID, err := strconv.Atoi(fields[2])
			if err != nil || wantID >= len(bundles) {
				fmt.Println("second argument should be bundle ID")
				break
			}

			fmt.Println(diff.Diff(bundles[gotID], bundles[wantID]))

		case "patch":
			// TODO
			// if len(fields) != 3 {
			// 	fmt.Println("expected 2 arguments to patch")
			// 	break
			// }
			// root := fields[1]
			// baseID, err := strconv.Atoi(fields[2])
			// if err != nil {
			// 	fmt.Println("second argument should be int")
			// }

			// _, err = readBundle(root)
			// if err != nil {
			// 	fmt.Println(helpMsg)
			// 	break
			// }

			// break

		default:
			fmt.Printf("%s\n", helpMsg)
		}

		fmt.Printf("\n> ")
	}
}

func queryBundle(bundle *precise.GroupedBundleDataMaps, path string, line, character int) error {
	results, err := precise.Query(bundle, path, line, character)
	if err != nil {
		fmt.Printf("No data found at location")
		return err
	}
	for idx, result := range results {
		fmt.Printf("Result %d:\n", idx)
		for idx, definition := range result.Definitions {
			if idx >= 10 {
				fmt.Printf("Abridging definitions...\n")
				break
			}
			fmt.Printf("Definition:   %v:%v:(%v, %v)\n", definition.URI, definition.StartLine, definition.StartCharacter, definition.EndCharacter)
		}

		for idx, reference := range result.References {
			if idx >= 10 {
				fmt.Printf("Abridging references...\n")
				break
			}
			fmt.Printf("Reference:    %v:%v:(%v, %v)\n", reference.URI, reference.StartLine, reference.StartCharacter, reference.EndCharacter)
		}

		for idx, moniker := range result.Monikers {
			if idx >= 10 {
				fmt.Printf("Abridging monikers...\n")
			}
			fmt.Printf("Moniker:      %v:%v:%v:%v\n", moniker.Kind, moniker.Scheme, moniker.Identifier, moniker.Kind)
		}

		fmt.Printf("Hover data:\n\n%v\n\n", result.Hover)

	}
	return nil
}
