package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/sourcegraph/sourcegraph/enterprise/lib/codeintel/lsif/conversion"
	"github.com/sourcegraph/sourcegraph/enterprise/lib/codeintel/pathexistence"
	"github.com/sourcegraph/sourcegraph/enterprise/lib/codeintel/semantic"
)

const helpMsg string = `
?                                                     help
count                                                 list dumps
paths <dump id>                                       list some paths from a dump
query <dump id> <path> <line> <column>                query dumps
index <path to repo root with dump.lsif>              index new dump
`

const helpMsgTODO string = `
patch <path to repo root with dump.lsif> <base ID>    patch dump
docRefs <dump id> <paths file>                        replace list of paths in paths file with list of docs referencing in dump
eq <dump id> <dump id>                                test two dumps for equality
cd <path>                                             change directory
`

func main() {
	var bundles []*conversion.GroupedBundleDataMaps

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
			break
		case "query":
			if len(fields) != 5 {
				fmt.Println("wrong number of args to q")
				break
			}
			dumpID, err := strconv.Atoi(fields[1])
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

			err = queryBundle(bundles[dumpID], path, line, column)
			if err != nil {
				fmt.Println(helpMsg)
				break
			}
			break

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
			if len(fields) != 2 {
				fmt.Println("expected 1 argument to index")
				break
			}
			root := fields[1]

			bundle, err := readBundle(len(bundles), root)
			if err != nil {
			}
			bundles = append(bundles, bundle)
			fmt.Println("indexing finished")
			break

		case "patch":
			// TODO
			if len(fields) != 3 {
				fmt.Println("expected 2 arguments to patch")
				break
			}
			root := fields[1]
			baseID, err := strconv.Atoi(fields[2])
			if err != nil {
				fmt.Println("second argument should be int")
			}

			_, err = readBundle(baseID, root)
			if err != nil {
				fmt.Println(helpMsg)
				break
			}

			break

		case "?", "h", "help":
			fmt.Println(helpMsg)
		default:
			fmt.Println(helpMsg)
		}

		fmt.Printf("\n> ")
	}
}

func queryBundle(bundle *conversion.GroupedBundleDataMaps, path string, line, character int) error {
	results, err := conversion.Query(bundle, path, line, character)
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
			fmt.Printf("Moniker:      %v:%v:%v\n", moniker.Scheme, moniker.Identifier, moniker.Kind)
		}

		fmt.Printf("Hover data:\n\n%v\n\n", result.Hover)

	}
	return nil
}

func makeExistenceFunc(path string) pathexistence.GetChildrenFunc {
	return func(ctx context.Context, dirnames []string) (map[string][]string, error) {
		cmd := exec.Command("git", append([]string{"ls-tree", "--name-only", "HEAD"}, cleanDirectoriesForLsTree(dirnames)...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Printf("git ls-tree failed with output %v\n", string(out))
			fmt.Printf("args were %v\n", strings.Join(dirnames, ", "))
		}

		return parseDirectoryChildren(dirnames, strings.Split(string(out), "\n")), nil
	}
}

func readBundle(dumpID int, root string) (*conversion.GroupedBundleDataMaps, error) {
	dumpPath := path.Join(root, "dump.lsif")
	getChildrenFunc := makeExistenceFunc(root)
	file, err := os.Open(dumpPath)
	if err != nil {
		fmt.Printf("Couldn't open file %v\n", dumpPath)
		return nil, err
	}
	defer file.Close()

	bundle, err := conversion.Correlate(context.Background(), file, dumpID, "", getChildrenFunc)
	if err != nil {
		fmt.Println("Conversion failed")
		return nil, err
	}

	return conversion.GroupedBundleDataChansToMaps(context.Background(), bundle), nil
}

// finds all documents which have definition edges pointing into the argument list
func documentsReferencing(bundle *conversion.GroupedBundleDataMaps, paths []string) (_ []string, err error) {
	pathMap := map[string]struct{}{}
	for _, path := range paths {
		pathMap[path] = struct{}{}
	}

	resultIDs := map[semantic.ID]struct{}{}
	for _, resultChunk := range bundle.ResultChunks {
		for resultID, documentIDRangeIDs := range resultChunk.DocumentIDRangeIDs {
			for _, documentIDRangeID := range documentIDRangeIDs {
				// Skip results that do not point into one of the given documents
				if _, ok := pathMap[resultChunk.DocumentPaths[documentIDRangeID.DocumentID]]; !ok {
					continue
				}

				resultIDs[resultID] = struct{}{}
			}
		}
	}

	var pathsReferencing []string
	for path, document := range bundle.Documents {
		for _, r := range document.Ranges {
			if _, ok := resultIDs[r.DefinitionResultID]; ok {
				pathsReferencing = append(pathsReferencing, path)
				break
			}
		}
	}

	return pathsReferencing, nil
}

// cleanDirectoriesForLsTree sanitizes the input dirnames to a git ls-tree command. There are a
// few peculiarities handled here:
//
//   1. The root of the tree must be indicated with `.`, and
//   2. In order for git ls-tree to return a directory's contents, the name must end in a slash.
func cleanDirectoriesForLsTree(dirnames []string) []string {
	var args []string
	for _, dir := range dirnames {
		if dir == "" {
			args = append(args, ".")
		} else {
			if !strings.HasSuffix(dir, "/") {
				dir += "/"
			}
			args = append(args, dir)
		}
	}

	return args
}

// parseDirectoryChildren converts the flat list of files from git ls-tree into a map. The keys of the
// resulting map are the input (unsanitized) dirnames, and the value of that key are the files nested
// under that directory. If dirnames contains a directory that encloses another, then the paths will
// be placed into the key sharing the longest path prefix.
func parseDirectoryChildren(dirnames, paths []string) map[string][]string {
	childrenMap := map[string][]string{}

	// Ensure each directory has an entry, even if it has no children
	// listed in the gitserver output.
	for _, dirname := range dirnames {
		childrenMap[dirname] = nil
	}

	// Order directory names by length (biggest first) so that we assign
	// paths to the most specific enclosing directory in the following loop.
	sort.Slice(dirnames, func(i, j int) bool {
		return len(dirnames[i]) > len(dirnames[j])
	})

	for _, path := range paths {
		if strings.Contains(path, "/") {
			for _, dirname := range dirnames {
				if strings.HasPrefix(path, dirname) {
					childrenMap[dirname] = append(childrenMap[dirname], path)
					break
				}
			}
		} else {
			// No need to loop here. If we have a root input directory it
			// will necessarily be the last element due to the previous
			// sorting step.
			if len(dirnames) > 0 && dirnames[len(dirnames)-1] == "" {
				childrenMap[""] = append(childrenMap[""], path)
			}
		}
	}

	return childrenMap
}
