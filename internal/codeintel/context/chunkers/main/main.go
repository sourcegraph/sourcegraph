package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/context/chunkers"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"runtime/pprof"
	"strings"
)

var chunkerType = flag.String("chunker", "treesitter", "chunker type: treesitter or classic")
var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")

func main() {
	flag.Parse()
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		defer pprof.StopCPUProfile()
	}

	if len(flag.Args()) < 2 {
		log.Fatal("usage: main -chunker=<chunker type> <command> <file>")
	}

	opts := &chunkers.ChunkOptions{
		ChunkTokensThreshold:           256,
		NoSplitTokensThreshold:         384,
		ChunkEarlySplitTokensThreshold: 224,
		CoalesceThreshold:              100,
	}

	command := flag.Args()[0]
	if command == "chunk" {
		filename := flag.Args()[1]
		file, err := os.Open(filename)
		if err != nil {
			log.Fatal(err)
		}

		content, err := io.ReadAll(file)
		if err := file.Close(); err != nil {
			log.Fatal(err)
		}
		var chunker chunkers.Chunker

		if *chunkerType == "classic" {
			chunker = chunkers.NewClassicChunker(opts)
		} else {
			chunker = chunkers.NewTreeSitterChunker(opts)
		}

		chunks, err := chunker.Chunk(string(content), filename)
		if err != nil {
			log.Fatal(err)
		}

		for _, chunk := range chunks {
			//fmt.Println("######################################")
			fmt.Printf("chunk: L%d - L%d (%d lines, %d bytes)\n", chunk.StartLine, chunk.EndLine, chunk.EndLine-chunk.StartLine, len(chunk.Content))
			//fmt.Printf("chunk: L%d - L%d (%d lines, %d bytes): ```%s```\n", chunk.StartLine, chunk.EndLine, chunk.EndLine-chunk.StartLine, len(chunk.Content), chunk.Content)
		}

	} else if command == "print" {
		filename := flag.Args()[1]
		file, err := os.Open(filename)
		if err != nil {
			log.Fatal(err)
		}

		content, err := io.ReadAll(file)
		if err := file.Close(); err != nil {
			log.Fatal(err)
		}
		parse(string(content))
	} else if command == "chunk-repo" {
		var chunker chunkers.Chunker
		classicChunker := chunkers.NewClassicChunker(opts)

		if *chunkerType == "classic" {
			chunker = chunkers.NewClassicChunker(opts)
		} else {
			chunker = chunkers.NewTreeSitterChunker(opts)
		}

		cmd := exec.Command("git", "-C", flag.Args()[1], "ls-files")
		// Get a pipe to read from the command's stdout
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			fmt.Println("Error creating StdoutPipe:", err)
			return
		}

		// Start the command
		if err := cmd.Start(); err != nil {
			fmt.Println("Error starting command:", err)
			return
		}

		// Create a scanner to read the output line by line
		scanner := bufio.NewScanner(stdout)

		// Slice to store the output lines
		var files []string

		// Read lines and append to the slice
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			files = append(files, path.Join(flag.Args()[1], line))
		}

		// Check for errors during scanning
		if err := scanner.Err(); err != nil {
			fmt.Println("Error reading output:", err)
			return
		}

		// Wait for the command to finish
		if err := cmd.Wait(); err != nil {
			fmt.Println("Command finished with error:", err)
			return
		}

		total := 0
		for _, f := range files {
			file, err := os.Open(f)
			if err != nil {
				log.Println(err)
				continue
			}
			content, err := io.ReadAll(file)
			if err := file.Close(); err != nil {
				log.Fatal(err)
			}
			chunks, err := chunker.Chunk(string(content), f)
			if err != nil {
				chunks, err = classicChunker.Chunk(string(content), f)
				if err != nil {
					log.Fatal(err)
				}
			}
			total += len(chunks)
		}
		fmt.Println("total", total)

	} else {
		log.Fatalf("unknown command: %s", command)
	}

}

func parse(content string) {
	parser := sitter.NewParser()
	parser.SetLanguage(typescript.GetLanguage())
	tree, err := parser.ParseCtx(context.Background(), nil, []byte(content))
	if err != nil {
		log.Fatal(err)
	}
	printTree(tree.RootNode(), content, "")
}

func printTree(node *sitter.Node, content, indent string) {
	if node.ChildCount() == 0 {
		fmt.Printf("%2d", len(indent))
		fmt.Println(indent, node.String(), content[node.StartByte():node.EndByte()])
	}
	for i := 0; uint32(i) < node.ChildCount(); i++ {
		printTree(node.Child(i), content, indent+"  ")
	}
}
