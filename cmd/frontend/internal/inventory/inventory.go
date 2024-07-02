// Package inventory scans a directory tree to identify the
// programming languages, etc., in use.
package inventory

import (
	"bytes"
	"context"
	"fmt"
	"github.com/go-enry/go-enry/v2"      //nolint:depguard - FIXME: replace this usage of enry with languages package
	"github.com/go-enry/go-enry/v2/data" //nolint:depguard - FIXME: replace this usage of enry with languages package
	"github.com/sourcegraph/log"
	"io"
	"io/fs"

	"github.com/sourcegraph/sourcegraph/lib/codeintel/languages"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// fileReadBufferSize is the size of the buffer we'll use while reading file contents
const fileReadBufferSize = 4 * 1024

// Inventory summarizes a tree's contents (e.g., which programming
// languages are used).
type Inventory struct {
	// Languages are the programming languages used in the tree.
	Languages []Lang `json:"Languages,omitempty"`
}

// Lang represents a programming language used in a directory tree.
type Lang struct {
	// Name is the name of a programming language (e.g., "Go" or
	// "Java").
	Name string `json:"Name,omitempty"`
	// TotalBytes is the total number of bytes of code written in the
	// programming language.
	TotalBytes uint64 `json:"TotalBytes,omitempty"`
	// TotalLines is the total number of lines of code written in the
	// programming language.
	TotalLines uint64 `json:"TotalLines,omitempty"`
}

var newLine = []byte{'\n'}

func getLang(ctx context.Context, file fs.FileInfo, getFileReader func(ctx context.Context, path string) (io.ReadCloser, error), skipEnhancedLanguageDetection bool) (Lang, error) {
	if file == nil {
		return Lang{}, nil
	}
	if !file.Mode().IsRegular() || enry.IsVendor(file.Name()) {
		return Lang{}, nil
	}

	var lang Lang
	// In many cases, GetLanguageByFilename can detect the language conclusively just from the
	// filename. If not, we pass a subset of the file contents for analysis.
	matchedLang, safe := GetLanguageByFilename(file.Name())

	if skipEnhancedLanguageDetection {
		lang.Name = matchedLang
		lang.TotalBytes = uint64(file.Size())
		return lang, nil
	}

	rc, err := getFileReader(ctx, file.Name())
	if err != nil {
		return Lang{}, errors.Wrap(err, "Failed to create a file reader.")
	}
	if rc != nil {
		defer rc.Close()
	}

	buf := make([]byte, fileReadBufferSize)

	if !safe {
		// Detect language from content
		n, err := io.ReadFull(rc, buf)
		if err == io.EOF {
			// No bytes read, indicating an empty file
			return Lang{}, nil
		}
		if err != nil && err != io.ErrUnexpectedEOF {
			return lang, errors.Wrap(err, "reading initial file data")
		}

		// GetLanguages can return multiple matches for ambiguous languages. If there are multiple
		// we will take the first one.
		languages, _ := languages.GetLanguages(file.Name(), func() ([]byte, error) { return buf[:n], nil })
		if len(languages) > 0 {
			matchedLang = languages[0]

			lang.TotalBytes += uint64(n)
			lang.TotalLines += uint64(bytes.Count(buf[:n], newLine))
			lang.Name = matchedLang
			if err == io.ErrUnexpectedEOF {
				// File is smaller than buf, we can exit early
				if !bytes.HasSuffix(buf[:n], newLine) {
					// Add final line
					lang.TotalLines++
				}
				return lang, nil
			}
		}
	}
	lang.Name = matchedLang

	lineCount, byteCount, err := countLines(rc, buf)
	if err != nil {
		return lang, err
	}
	lang.TotalLines += uint64(lineCount)
	lang.TotalBytes += uint64(byteCount)
	return lang, nil
}

// countLines counts the number of lines in the supplied reader
// it uses buf as a temporary buffer
func countLines(r io.Reader, buf []byte) (lineCount int, byteCount int, err error) {
	var trailingNewLine bool
	for {
		n, err := r.Read(buf)
		lineCount += bytes.Count(buf[:n], newLine)
		byteCount += n
		// We need this check because the last read will often
		// return (0, io.EOF) and we want to look at the last
		// valid read to determine if there was a trailing newline
		if n > 0 {
			trailingNewLine = bytes.HasSuffix(buf[:n], newLine)
		}
		if err == io.EOF {
			if !trailingNewLine && byteCount > 0 {
				// Add final line
				lineCount++
			}
			break
		}
		if err != nil {
			return 0, 0, errors.Wrap(err, "counting lines")
		}
	}
	return lineCount, byteCount, nil
}

// GetLanguageByFilename returns the guessed language for the named file (and
// safe == true if this is very likely to be correct).
func GetLanguageByFilename(name string) (language string, safe bool) {
	return enry.GetLanguageByExtension(name)
}

func init() {
	// Treat .tsx and .jsx as TypeScript and JavaScript, respectively, instead of distinct languages
	// called "TSX" and "JSX". This is more consistent with user expectations.
	data.ExtensionsByLanguage["TypeScript"] = append(data.ExtensionsByLanguage["TypeScript"], ".tsx")
	data.LanguagesByExtension[".tsx"] = []string{"TypeScript"}
	data.ExtensionsByLanguage["JavaScript"] = append(data.ExtensionsByLanguage["JavaScript"], ".jsx")
	data.LanguagesByExtension[".jsx"] = []string{"JavaScript"}

	// Prefer more popular languages which share extensions
	preferLanguage("Markdown", ".md") // instead of GCC Machine Description
	preferLanguage("Rust", ".rs")     // instead of RenderScript
}

// preferLanguage updates LanguagesByExtension to have lang listed first for
// ext.
func preferLanguage(lang, ext string) {
	langs := data.LanguagesByExtension[ext]
	for i := range langs {
		if langs[i] == lang {
			// swap to front
			for ; i > 0; i-- {
				langs[i-1], langs[i] = langs[i], langs[i-1]
			}
			return
		}
	}

	log.Scoped("inventory").Fatal(fmt.Sprintf("%q not in %q: %q", lang, ext, langs))
}
