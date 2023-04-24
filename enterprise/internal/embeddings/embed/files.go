package embed

import (
	"bytes"
	"path/filepath"
	"strings"

	"github.com/sourcegraph/sourcegraph/enterprise/internal/paths"
	"github.com/sourcegraph/sourcegraph/internal/binary"
)

const MIN_EMBEDDABLE_FILE_SIZE = 32
const MAX_LINE_LENGTH = 2048

var autogeneratedFileHeaders = [][]byte{
	[]byte("autogenerated file"),
	[]byte("lockfile"),
	[]byte("generated by"),
	[]byte("do not edit"),
}

var textFileExtensions = map[string]struct{}{
	"md":       {},
	"markdown": {},
	"rst":      {},
	"txt":      {},
}

var defaultExcludedFilePathPatterns = []string{
	".*ignore", // Files like .gitignore, .eslintignore
	".gitattributes",
	".mailmap",
	"*.csv",
	"*.sql",
	"*.svg",
	"*.json",
	"*.jsonc",
	"*.jsonl",
	"*.xml",
	"*.yml",
	"*.yaml",
	"__fixtures__/",
	"node_modules/",
	"testdata/",
	"mocks/",
	"vendor/",
}

func GetDefaultExcludedFilePathPatterns() []*paths.GlobPattern {
	return CompileGlobPatterns(defaultExcludedFilePathPatterns)
}

func CompileGlobPatterns(patterns []string) []*paths.GlobPattern {
	globPatterns := make([]*paths.GlobPattern, 0, len(patterns))
	for _, pattern := range patterns {
		globPattern, err := paths.Compile(pattern)
		if err != nil {
			continue
		}
		globPatterns = append(globPatterns, globPattern)
	}
	return globPatterns
}

func isExcludedFilePath(filePath string, excludedFilePathPatterns []*paths.GlobPattern) bool {
	for _, excludedFilePathPattern := range excludedFilePathPatterns {
		if excludedFilePathPattern.Match(filePath) {
			return true
		}
	}
	return false
}

type SkipReason int8

const (
	// File is binary
	SkipReasonBinary SkipReason = iota + 1

	// File is too small to provide useful embeddings
	SkipReasonSmall

	// File is larger than the max file size
	SkipReasonLarge

	// File is autogenerated
	SkipReasonAutogenerated

	// File has a line that is too long
	SkipReasonLongLine

	// File was excluded by configuration rules
	SkipReasonExcluded

	// File was excluded because we hit the max embedding limit for the repo
	SkipReasonMaxEmbeddings
)

func (s SkipReason) String() string {
	switch s {
	case SkipReasonBinary:
		return "binary"
	case SkipReasonSmall:
		return "small"
	case SkipReasonLarge:
		return "large"
	case SkipReasonAutogenerated:
		return "autogenerated"
	case SkipReasonLongLine:
		return "long_line"
	case SkipReasonExcluded:
		return "excluded"
	case SkipReasonMaxEmbeddings:
		return "max_embeddings"
	default:
		return "unknown"
	}
}

type SkipStats struct {
	reasons    map[SkipReason]int
	byteCounts map[SkipReason]int
}

func (s *SkipStats) Add(r SkipReason, byteCount int) {
	if s.reasons == nil {
		s.reasons = make(map[SkipReason]int)
	}
	s.reasons[r] += 1

	if s.byteCounts == nil {
		s.byteCounts = make(map[SkipReason]int)
	}
	s.byteCounts[r] += byteCount
}

func (s *SkipStats) Counts() map[string]int {
	m := make(map[string]int, len(s.reasons))
	for k, v := range s.reasons {
		m[k.String()] = v
	}
	return m
}

func (s *SkipStats) ByteCounts() map[string]int {
	m := make(map[string]int, len(s.byteCounts))
	for k, v := range s.byteCounts {
		m[k.String()] = v
	}
	return m
}

func isEmbeddableFileContent(content []byte) (embeddable bool, reason SkipReason) {
	if binary.IsBinary(content) {
		return false, SkipReasonBinary
	}

	if len(bytes.TrimSpace(content)) < MIN_EMBEDDABLE_FILE_SIZE {
		return false, SkipReasonSmall
	}

	lines := bytes.Split(content, []byte("\n"))

	fileHeader := bytes.ToLower(bytes.Join(lines[0:min(5, len(lines))], []byte("\n")))
	for _, header := range autogeneratedFileHeaders {
		if bytes.Contains(fileHeader, header) {
			return false, SkipReasonAutogenerated
		}
	}

	for _, line := range lines {
		if len(line) > MAX_LINE_LENGTH {
			return false, SkipReasonLongLine
		}
	}

	return true, 0
}

func isValidTextFile(fileName string) bool {
	ext := strings.TrimPrefix(filepath.Ext(fileName), ".")
	_, ok := textFileExtensions[strings.ToLower(ext)]
	if ok {
		return true
	}
	basename := strings.ToLower(filepath.Base(fileName))
	return strings.HasPrefix(basename, "license")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
