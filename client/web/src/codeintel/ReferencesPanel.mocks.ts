import { useState } from 'react'

import type { MockedResponse } from '@apollo/client/testing'
import { of } from 'rxjs'

import { logger } from '@sourcegraph/common'
import { getDocumentNode, dataOrThrowErrors, useQuery } from '@sourcegraph/http-client'
import { noOpTelemetryRecorder } from '@sourcegraph/shared/src/telemetry'
import { NOOP_TELEMETRY_SERVICE } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { NOOP_PLATFORM_CONTEXT } from '@sourcegraph/shared/src/testing/searchTestHelpers'

import type { ConnectionQueryArguments } from '../components/FilteredConnection'
import { asGraphQLResult } from '../components/FilteredConnection/utils'
import {
    type UsePreciseCodeIntelForPositionResult,
    type UsePreciseCodeIntelForPositionVariables,
    HighlightResponseFormat,
    type LocationFields,
    type ReferencesPanelHighlightedBlobVariables,
    type ResolveRepoAndRevisionVariables,
} from '../graphql-operations'

import { buildPreciseLocation, LocationsGroup } from './location'
import type { ReferencesPanelProps } from './ReferencesPanel'
import {
    USE_PRECISE_CODE_INTEL_FOR_POSITION_QUERY,
    RESOLVE_REPO_REVISION_BLOB_QUERY,
    FETCH_HIGHLIGHTED_BLOB,
} from './ReferencesPanelQueries'
import type { UseCodeIntelParameters, UseCodeIntelResult } from './useCodeIntel'

const goDiffFileContent =
    'package main\n\nimport (\n\t"flag"\n\t"fmt"\n\t"io"\n\t"log"\n\t"os"\n\n\t"github.com/sourcegraph/go-diff/diff"\n)\n\n// A diagnostic program to aid in debugging diff parsing or printing\n// errors.\n\nconst stdin = "\u003Cstdin\u003E"\n\nvar (\n\tdiffPath = flag.String("f", stdin, "filename of diff (default: stdin)")\n\tfileIdx  = flag.Int("i", -1, "if \u003E= 0, only print and report errors from the i\'th file (0-indexed)")\n)\n\nfunc main() {\n\tlog.SetFlags(0)\n\tflag.Parse()\n\n\tvar diffFile *os.File\n\tif *diffPath == stdin {\n\t\tdiffFile = os.Stdin\n\t} else {\n\t\tvar err error\n\t\tdiffFile, err = os.Open(*diffPath)\n\t\tif err != nil {\n\t\t\tlog.Fatal(err)\n\t\t}\n\t}\n\tdefer diffFile.Close()\n\n\tr := diff.NewMultiFileDiffReader(diffFile)\n\tfor i := 0; ; i++ {\n\t\treport := (*fileIdx == -1) || i == *fileIdx // true if -i==-1 or if this is the i\'th file\n\n\t\tlabel := fmt.Sprintf("file(%d)", i)\n\t\tfdiff, err := r.ReadFile()\n\t\tif fdiff != nil {\n\t\t\tlabel = fmt.Sprintf("orig(%s) new(%s)", fdiff.OrigName, fdiff.NewName)\n\t\t}\n\t\tif err == io.EOF {\n\t\t\tbreak\n\t\t}\n\t\tif err != nil {\n\t\t\tif report {\n\t\t\t\tlog.Fatalf("err read %s: %s", label, err)\n\t\t\t} else {\n\t\t\t\tcontinue\n\t\t\t}\n\t\t}\n\n\t\tif report {\n\t\t\tlog.Printf("ok read: %s", label)\n\t\t}\n\n\t\tout, err := diff.PrintFileDiff(fdiff)\n\t\tif err != nil {\n\t\t\tif report {\n\t\t\t\tlog.Fatalf("err print %s: %s", label, err)\n\t\t\t} else {\n\t\t\t\tcontinue\n\t\t\t}\n\t\t}\n\t\tif report {\n\t\t\tif _, err := os.Stdout.Write(out); err != nil {\n\t\t\t\tlog.Fatal(err)\n\t\t\t}\n\t\t}\n\t}\n}\n'
const diffFileContent =
    "package diff\n\nimport (\n\t\"bytes\"\n\t\"time\"\n)\n\n// A FileDiff represents a unified diff for a single file.\n//\n// A file unified diff has a header that resembles the following:\n//\n//   --- oldname\t2009-10-11 15:12:20.000000000 -0700\n//   +++ newname\t2009-10-11 15:12:30.000000000 -0700\ntype FileDiff struct {\n\t// the original name of the file\n\tOrigName string\n\t// the original timestamp (nil if not present)\n\tOrigTime *time.Time\n\t// the new name of the file (often same as OrigName)\n\tNewName string\n\t// the new timestamp (nil if not present)\n\tNewTime *time.Time\n\t// extended header lines (e.g., git's \"new mode \u003Cmode\u003E\", \"rename from \u003Cpath\u003E\", etc.)\n\tExtended []string\n\t// hunks that were changed from orig to new\n\tHunks []*Hunk\n}\n\n// A Hunk represents a series of changes (additions or deletions) in a file's\n// unified diff.\ntype Hunk struct {\n\t// starting line number in original file\n\tOrigStartLine int32\n\t// number of lines the hunk applies to in the original file\n\tOrigLines int32\n\t// if \u003E 0, then the original file had a 'No newline at end of file' mark at this offset\n\tOrigNoNewlineAt int32\n\t// starting line number in new file\n\tNewStartLine int32\n\t// number of lines the hunk applies to in the new file\n\tNewLines int32\n\t// optional section heading\n\tSection string\n\t// 0-indexed line offset in unified file diff (including section headers); this is\n\t// only set when Hunks are read from entire file diff (i.e., when ReadAllHunks is\n\t// called) This accounts for hunk headers, too, so the StartPosition of the first\n\t// hunk will be 1.\n\tStartPosition int32\n\t// hunk body (lines prefixed with '-', '+', or ' ')\n\tBody []byte\n}\n\n// A Stat is a diff stat that represents the number of lines added/changed/deleted.\ntype Stat struct {\n\t// number of lines added\n\tAdded int32\n\t// number of lines changed\n\tChanged int32\n\t// number of lines deleted\n\tDeleted int32\n}\n\n// Stat computes the number of lines added/changed/deleted in all\n// hunks in this file's diff.\nfunc (d *FileDiff) Stat() Stat {\n\ttotal := Stat{}\n\tfor _, h := range d.Hunks {\n\t\ttotal.add(h.Stat())\n\t}\n\treturn total\n}\n\n// Stat computes the number of lines added/changed/deleted in this\n// hunk.\nfunc (h *Hunk) Stat() Stat {\n\tlines := bytes.Split(h.Body, []byte{'\\n'})\n\tvar last byte\n\tst := Stat{}\n\tfor _, line := range lines {\n\t\tif len(line) == 0 {\n\t\t\tlast = 0\n\t\t\tcontinue\n\t\t}\n\t\tswitch line[0] {\n\t\tcase '-':\n\t\t\tif last == '+' {\n\t\t\t\tst.Added--\n\t\t\t\tst.Changed++\n\t\t\t\tlast = 0 // next line can't change this one since this is already a change\n\t\t\t} else {\n\t\t\t\tst.Deleted++\n\t\t\t\tlast = line[0]\n\t\t\t}\n\t\tcase '+':\n\t\t\tif last == '-' {\n\t\t\t\tst.Deleted--\n\t\t\t\tst.Changed++\n\t\t\t\tlast = 0 // next line can't change this one since this is already a change\n\t\t\t} else {\n\t\t\t\tst.Added++\n\t\t\t\tlast = line[0]\n\t\t\t}\n\t\tdefault:\n\t\t\tlast = 0\n\t\t}\n\t}\n\treturn st\n}\n\nvar (\n\thunkPrefix          = []byte(\"@@ \")\n\tonlyInMessagePrefix = []byte(\"Only in \")\n)\n\nconst hunkHeader = \"@@ -%d,%d +%d,%d @@\"\nconst onlyInMessage = \"Only in %s: %s\\n\"\n\n// diffTimeParseLayout is the layout used to parse the time in unified diff file\n// header timestamps.\n// See https://www.gnu.org/software/diffutils/manual/html_node/Detailed-Unified.html.\nconst diffTimeParseLayout = \"2006-01-02 15:04:05 -0700\"\n\n// diffTimeFormatLayout is the layout used to format (i.e., print) the time in unified diff file\n// header timestamps.\n// See https://www.gnu.org/software/diffutils/manual/html_node/Detailed-Unified.html.\nconst diffTimeFormatLayout = \"2006-01-02 15:04:05.000000000 -0700\"\n\nfunc (s *Stat) add(o Stat) {\n\ts.Added += o.Added\n\ts.Changed += o.Changed\n\ts.Deleted += o.Deleted\n}\n"
const printFileContent =
    'package diff\n\nimport (\n\t"bytes"\n\t"fmt"\n\t"io"\n\t"path/filepath"\n\t"time"\n)\n\n// PrintMultiFileDiff prints a multi-file diff in unified diff format.\nfunc PrintMultiFileDiff(ds []*FileDiff) ([]byte, error) {\n\tvar buf bytes.Buffer\n\tfor _, d := range ds {\n\t\tdiff, err := PrintFileDiff(d)\n\t\tif err != nil {\n\t\t\treturn nil, err\n\t\t}\n\t\tif _, err := buf.Write(diff); err != nil {\n\t\t\treturn nil, err\n\t\t}\n\t}\n\treturn buf.Bytes(), nil\n}\n\n// PrintFileDiff prints a FileDiff in unified diff format.\n//\n// TODO(sqs): handle escaping whitespace/etc. chars in filenames\nfunc PrintFileDiff(d *FileDiff) ([]byte, error) {\n\tvar buf bytes.Buffer\n\n\tfor _, xheader := range d.Extended {\n\t\tif _, err := fmt.Fprintln(\u0026buf, xheader); err != nil {\n\t\t\treturn nil, err\n\t\t}\n\t}\n\n\t// FileDiff is added/deleted file\n\t// No further hunks printing needed\n\tif d.NewName == "" {\n\t\t_, err := fmt.Fprintf(\u0026buf, onlyInMessage, filepath.Dir(d.OrigName), filepath.Base(d.OrigName))\n\t\tif err != nil {\n\t\t\treturn nil, err\n\t\t}\n\t\treturn buf.Bytes(), nil\n\t}\n\n\tif d.Hunks == nil {\n\t\treturn buf.Bytes(), nil\n\t}\n\n\tif err := printFileHeader(\u0026buf, "--- ", d.OrigName, d.OrigTime); err != nil {\n\t\treturn nil, err\n\t}\n\tif err := printFileHeader(\u0026buf, "+++ ", d.NewName, d.NewTime); err != nil {\n\t\treturn nil, err\n\t}\n\n\tph, err := PrintHunks(d.Hunks)\n\tif err != nil {\n\t\treturn nil, err\n\t}\n\n\tif _, err := buf.Write(ph); err != nil {\n\t\treturn nil, err\n\t}\n\treturn buf.Bytes(), nil\n}\n\nfunc printFileHeader(w io.Writer, prefix string, filename string, timestamp *time.Time) error {\n\tif _, err := fmt.Fprint(w, prefix, filename); err != nil {\n\t\treturn err\n\t}\n\tif timestamp != nil {\n\t\tif _, err := fmt.Fprint(w, "\\t", timestamp.Format(diffTimeFormatLayout)); err != nil {\n\t\t\treturn err\n\t\t}\n\t}\n\tif _, err := fmt.Fprintln(w); err != nil {\n\t\treturn err\n\t}\n\treturn nil\n}\n\n// PrintHunks prints diff hunks in unified diff format.\nfunc PrintHunks(hunks []*Hunk) ([]byte, error) {\n\tvar buf bytes.Buffer\n\tfor _, hunk := range hunks {\n\t\t_, err := fmt.Fprintf(\u0026buf,\n\t\t\t"@@ -%d,%d +%d,%d @@", hunk.OrigStartLine, hunk.OrigLines, hunk.NewStartLine, hunk.NewLines,\n\t\t)\n\t\tif err != nil {\n\t\t\treturn nil, err\n\t\t}\n\t\tif hunk.Section != "" {\n\t\t\t_, err := fmt.Fprint(\u0026buf, " ", hunk.Section)\n\t\t\tif err != nil {\n\t\t\t\treturn nil, err\n\t\t\t}\n\t\t}\n\t\tif _, err := fmt.Fprintln(\u0026buf); err != nil {\n\t\t\treturn nil, err\n\t\t}\n\n\t\tif hunk.OrigNoNewlineAt == 0 {\n\t\t\tif _, err := buf.Write(hunk.Body); err != nil {\n\t\t\t\treturn nil, err\n\t\t\t}\n\t\t} else {\n\t\t\tif _, err := buf.Write(hunk.Body[:hunk.OrigNoNewlineAt]); err != nil {\n\t\t\t\treturn nil, err\n\t\t\t}\n\t\t\tif err := printNoNewlineMessage(\u0026buf); err != nil {\n\t\t\t\treturn nil, err\n\t\t\t}\n\t\t\tif _, err := buf.Write(hunk.Body[hunk.OrigNoNewlineAt:]); err != nil {\n\t\t\t\treturn nil, err\n\t\t\t}\n\t\t}\n\n\t\tif !bytes.HasSuffix(hunk.Body, []byte{\'\\n\'}) {\n\t\t\tif _, err := fmt.Fprintln(\u0026buf); err != nil {\n\t\t\t\treturn nil, err\n\t\t\t}\n\t\t\tif err := printNoNewlineMessage(\u0026buf); err != nil {\n\t\t\t\treturn nil, err\n\t\t\t}\n\t\t}\n\t}\n\treturn buf.Bytes(), nil\n}\n\nfunc printNoNewlineMessage(w io.Writer) error {\n\tif _, err := w.Write([]byte(noNewlineMessage)); err != nil {\n\t\treturn err\n\t}\n\tif _, err := fmt.Fprintln(w); err != nil {\n\t\treturn err\n\t}\n\treturn nil\n}\n'

interface ReferencePanelMock {
    url: string
    requestMocks: readonly MockedResponse[]
}

export function buildReferencePanelMocks(): ReferencePanelMock {
    const repoName = 'github.com/sourcegraph/go-diff'
    const commit = '9d1f353a285b3094bc33bdae277a19aedabe8b71'
    const path = 'diff/diff.go'
    const line = 16
    const character = 2

    const usePreciseCodeIntelVariables: UsePreciseCodeIntelForPositionVariables = {
        repository: repoName,
        commit,
        path,
        line: line - 1,
        character: character - 1,
        filter: null,
        firstReferences: 100,
        afterReferences: null,
        firstImplementations: 100,
        afterImplementations: null,
        firstPrototypes: 100,
        afterPrototypes: null,
    }

    const resolveRepoRevisionBlobVariables: ResolveRepoAndRevisionVariables = {
        repoName,
        filePath: path,
        revision: commit,
    }

    const fetchHighlightedBlobVariables: ReferencesPanelHighlightedBlobVariables = {
        commit,
        path,
        repository: repoName,
        format: HighlightResponseFormat.JSON_SCIP,
    }

    return {
        url: `/${repoName}@${commit}/-/blob/${path}?L${line}:${character}#tab=references`,
        requestMocks: [
            {
                request: {
                    query: getDocumentNode(USE_PRECISE_CODE_INTEL_FOR_POSITION_QUERY),
                    variables: usePreciseCodeIntelVariables,
                },
                result: { data: USE_PRECISE_CODE_INTEL_MOCK },
            },
            {
                request: {
                    query: getDocumentNode(RESOLVE_REPO_REVISION_BLOB_QUERY),
                    variables: resolveRepoRevisionBlobVariables,
                },
                result: RESOLVE_REPO_REVISION_BLOB_MOCK,
            },
            {
                request: {
                    query: getDocumentNode(FETCH_HIGHLIGHTED_BLOB),
                    variables: fetchHighlightedBlobVariables,
                },
                result: HIGHLIGHTED_FILE_MOCK,
            },
        ],
    }
}

function buildMockLocation({
    repo,
    commit,
    path,
    content,
    start,
    end,
}: {
    repo: string
    commit: string
    path: string
    content: string
    start: { line: number; character: number }
    end: { line: number; character: number }
}): LocationFields {
    const url = [
        `/${repo}`,
        `@${commit}`,
        `/-/blob/${path}`,
        `?L${start.line + 1}:${start.character + 1}-${end.line + 1}:${end.character + 1}`,
    ].join('')

    return {
        url,
        resource: {
            path,
            content,
            repository: {
                name: repo,
                __typename: 'CodeIntelRepository',
            },
            commit: {
                oid: commit,
                __typename: 'CodeIntelCommit',
            },
            __typename: 'CodeIntelGitBlob',
        },
        range: {
            start: { ...start, __typename: 'Position' },
            end: { ...end, __typename: 'Position' },
            __typename: 'Range',
        },
        __typename: 'Location',
    }
}

// Fake highlighting for lines 16 and 52 in diff/diff.go
export const highlightedLinesDiffGo = [
    ['<tr><td class="line" data-line="16"></td><td class="code">line 16</td></tr>'],
    ['<tr><td class="line" data-line="52"></td><td class="code">line 52</td></tr>'],
]

// Fake highlighting for line 46 in cmd/go-diff/go-diff.go
export const highlightedLinesGoDiffGo = [
    ['<tr><td class="line" data-line="46"></td><td class="code">line 46</td></tr>'],
]

const MOCK_DEFINITIONS: LocationFields[] = [
    buildMockLocation({
        repo: 'github.com/sourcegraph/go-diff',
        commit: '9d1f353a285b3094bc33bdae277a19aedabe8b71',
        path: 'diff/diff.go',
        content: diffFileContent,
        start: { line: 15, character: 1 },
        end: { line: 15, character: 9 },
    }),
]

const MOCK_REFERENCES: LocationFields[] = [
    buildMockLocation({
        repo: 'github.com/sourcegraph/go-diff',
        commit: '9d1f353a285b3094bc33bdae277a19aedabe8b71',
        path: 'cmd/go-diff/go-diff.go',
        content: goDiffFileContent,
        start: { line: 45, character: 49 },
        end: { line: 45, character: 57 },
    }),
    buildMockLocation({
        repo: 'github.com/sourcegraph/go-diff',
        commit: '9d1f353a285b3094bc33bdae277a19aedabe8b71',
        path: 'diff/diff.go',
        content: diffFileContent,
        start: { line: 15, character: 1 },
        end: { line: 15, character: 9 },
    }),
    buildMockLocation({
        repo: 'github.com/sourcegraph/go-diff',
        commit: '9d1f353a285b3094bc33bdae277a19aedabe8b71',
        path: 'diff/diff.go',
        content: printFileContent,
        start: { line: 51, character: 43 },
        end: { line: 51, character: 51 },
    }),
]

const USE_PRECISE_CODE_INTEL_MOCK: UsePreciseCodeIntelForPositionResult = {
    repository: {
        id: 'UmVwb3NpdG9yeToz',
        commit: {
            id: 'R2l0Q29tbWl0OnsiciI6IlVtVndiM05wZEc5eWVUb3oiLCJjIjoiOWQxZjM1M2EyODViMzA5NGJjMzNiZGFlMjc3YTE5YWVkYWJlOGI3MSJ9',
            blob: {
                lsif: {
                    references: {
                        nodes: MOCK_REFERENCES,
                        pageInfo: { endCursor: null, __typename: 'PageInfo' },
                        __typename: 'LocationConnection',
                    },
                    implementations: {
                        nodes: [],
                        pageInfo: {
                            endCursor: null,
                            __typename: 'PageInfo',
                        },
                        __typename: 'LocationConnection',
                    },
                    prototypes: {
                        nodes: [],
                        pageInfo: {
                            endCursor: null,
                            __typename: 'PageInfo',
                        },
                        __typename: 'LocationConnection',
                    },
                    definitions: {
                        nodes: MOCK_DEFINITIONS,
                        pageInfo: {
                            endCursor: null,
                            __typename: 'PageInfo',
                        },
                        __typename: 'LocationConnection',
                    },
                    __typename: 'GitBlobLSIFData',
                },
                __typename: 'GitBlob',
            },
            __typename: 'GitCommit',
        },
        __typename: 'Repository',
    },
}

const RESOLVE_REPO_REVISION_BLOB_MOCK = {
    data: {
        repositoryRedirect: {
            __typename: 'Repository',
            id: 'UmVwb3NpdG9yeToz',
            name: 'github.com/sourcegraph/go-diff',
            url: '/github.com/sourcegraph/go-diff',
            isFork: false,
            isArchived: false,
            commit: {
                oid: '9d1f353a285b3094bc33bdae277a19aedabe8b71',
                file: {
                    content: diffFileContent,
                    __typename: 'GitBlob',
                },
                __typename: 'GitCommit',
            },
            defaultBranch: {
                abbrevName: 'master',
                __typename: 'GitRef',
            },
        },
    },
}

const HIGHLIGHTED_FILE_MOCK = {
    data: {
        repository: {
            id: 'UmVwb3NpdG9yeToyMDQ=',
            commit: {
                id: 'R2l0Q29tbWl0OnsiciI6IlVtVndiM05wZEc5eWVUb3lNRFE9IiwiYyI6IjlkMWYzNTNhMjg1YjMwOTRiYzMzYmRhZTI3N2ExOWFlZGFiZThiNzEifQ==',
                blob: {
                    content: `package diff

                    import (
                        "bytes"
                        "time"
                    )

                    // A FileDiff represents a unified diff for a single file.
                    //
                    // A file unified diff has a header that resembles the following:
                    //
                    //   --- oldname	2009-10-11 15:12:20.000000000 -0700
                    //   +++ newname	2009-10-11 15:12:30.000000000 -0700
                    type FileDiff struct {
                        // the original name of the file
                        OrigName string
                        // the original timestamp (nil if not present)
                        OrigTime *time.Time
                        // the new name of the file (often same as OrigName)
                        NewName string
                        // the new timestamp (nil if not present)
                        NewTime *time.Time
                        // extended header lines (e.g., git's "new mode <mode>", "rename from <path>", etc.)
                        Extended []string
                        // hunks that were changed from orig to new
                        Hunks []*Hunk
                    }

                    // A Hunk represents a series of changes (additions or deletions) in a file's
                    // unified diff.
                    type Hunk struct {
                        // starting line number in original file
                        OrigStartLine int32
                        // number of lines the hunk applies to in the original file
                        OrigLines int32
                        // if > 0, then the original file had a 'No newline at end of file' mark at this offset
                        OrigNoNewlineAt int32
                        // starting line number in new file
                        NewStartLine int32
                        // number of lines the hunk applies to in the new file
                        NewLines int32
                        // optional section heading
                        Section string
                        // 0-indexed line offset in unified file diff (including section headers); this is
                        // only set when Hunks are read from entire file diff (i.e., when ReadAllHunks is
                        // called) This accounts for hunk headers, too, so the StartPosition of the first
                        // hunk will be 1.
                        StartPosition int32
                        // hunk body (lines prefixed with '-', '+', or ' ')
                        Body []byte
                    }

                    // A Stat is a diff stat that represents the number of lines added/changed/deleted.
                    type Stat struct {
                        // number of lines added
                        Added int32
                        // number of lines changed
                        Changed int32
                        // number of lines deleted
                        Deleted int32
                    }

                    // Stat computes the number of lines added/changed/deleted in all
                    // hunks in this file's diff.
                    func (d *FileDiff) Stat() Stat {
                        total := Stat{}
                        for _, h := range d.Hunks {
                            total.add(h.Stat())
                        }
                        return total
                    }

                    // Stat computes the number of lines added/changed/deleted in this
                    // hunk.
                    func (h *Hunk) Stat() Stat {
                        lines := bytes.Split(h.Body, []byte{'\n'})
                        var last byte
                        st := Stat{}
                        for _, line := range lines {
                            if len(line) == 0 {
                                last = 0
                                continue
                            }
                            switch line[0] {
                            case '-':
                                if last == '+' {
                                    st.Added--
                                    st.Changed++
                                    last = 0 // next line can't change this one since this is already a change
                                } else {
                                    st.Deleted++
                                    last = line[0]
                                }
                            case '+':
                                if last == '-' {
                                    st.Deleted--
                                    st.Changed++
                                    last = 0 // next line can't change this one since this is already a change
                                } else {
                                    st.Added++
                                    last = line[0]
                                }
                            default:
                                last = 0
                            }
                        }
                        return st
                    }

                    var (
                        hunkPrefix          = []byte("@@ ")
                        onlyInMessagePrefix = []byte("Only in ")
                    )

                    const hunkHeader = "@@ -%d,%d +%d,%d @@"
                    const onlyInMessage = "Only in %s: %s\n"

                    // diffTimeParseLayout is the layout used to parse the time in unified diff file
                    // header timestamps.
                    // See https://www.gnu.org/software/diffutils/manual/html_node/Detailed-Unified.html.
                    const diffTimeParseLayout = "2006-01-02 15:04:05 -0700"

                    // diffTimeFormatLayout is the layout used to format (i.e., print) the time in unified diff file
                    // header timestamps.
                    // See https://www.gnu.org/software/diffutils/manual/html_node/Detailed-Unified.html.
                    const diffTimeFormatLayout = "2006-01-02 15:04:05.000000000 -0700"

                    func (s *Stat) add(o Stat) {
                        s.Added += o.Added
                        s.Changed += o.Changed
                        s.Deleted += o.Deleted
                    }`,
                    highlight: {
                        aborted: false,
                        lsif: JSON.stringify({
                            occurrences: [
                                { range: [0, 0, 7], syntaxKind: 4 },
                                { range: [0, 8, 12], syntaxKind: 14 },
                                { range: [2, 0, 6], syntaxKind: 4 },
                                { range: [3, 1, 8], syntaxKind: 27 },
                                { range: [4, 1, 7], syntaxKind: 27 },
                                { range: [7, 0, 58], syntaxKind: 1 },
                                { range: [8, 0, 2], syntaxKind: 1 },
                                { range: [9, 0, 65], syntaxKind: 1 },
                                { range: [10, 0, 2], syntaxKind: 1 },
                                { range: [11, 0, 52], syntaxKind: 1 },
                                { range: [12, 0, 52], syntaxKind: 1 },
                                { range: [13, 0, 4], syntaxKind: 4 },
                                { range: [13, 5, 13], syntaxKind: 19 },
                                { range: [13, 14, 20], syntaxKind: 4 },
                                { range: [14, 1, 33], syntaxKind: 1 },
                                { range: [15, 1, 9], syntaxKind: 6 },
                                { range: [15, 10, 16], syntaxKind: 20 },
                                { range: [16, 1, 47], syntaxKind: 1 },
                                { range: [17, 1, 9], syntaxKind: 6 },
                                { range: [17, 10, 11], syntaxKind: 5 },
                                { range: [17, 11, 15], syntaxKind: 14 },
                                { range: [17, 16, 20], syntaxKind: 19 },
                                { range: [18, 1, 53], syntaxKind: 1 },
                                { range: [19, 1, 8], syntaxKind: 6 },
                                { range: [19, 9, 15], syntaxKind: 20 },
                                { range: [20, 1, 42], syntaxKind: 1 },
                                { range: [21, 1, 8], syntaxKind: 6 },
                                { range: [21, 9, 10], syntaxKind: 5 },
                                { range: [21, 10, 14], syntaxKind: 14 },
                                { range: [21, 15, 19], syntaxKind: 19 },
                                { range: [22, 1, 85], syntaxKind: 1 },
                                { range: [23, 1, 9], syntaxKind: 6 },
                                { range: [23, 12, 18], syntaxKind: 20 },
                                { range: [24, 1, 44], syntaxKind: 1 },
                                { range: [25, 1, 6], syntaxKind: 6 },
                                { range: [25, 9, 10], syntaxKind: 5 },
                                { range: [25, 10, 14], syntaxKind: 19 },
                                { range: [28, 0, 77], syntaxKind: 1 },
                                { range: [29, 0, 16], syntaxKind: 1 },
                                { range: [30, 0, 4], syntaxKind: 4 },
                                { range: [30, 5, 9], syntaxKind: 19 },
                                { range: [30, 10, 16], syntaxKind: 4 },
                                { range: [31, 1, 41], syntaxKind: 1 },
                                { range: [32, 1, 14], syntaxKind: 6 },
                                { range: [32, 15, 20], syntaxKind: 20 },
                                { range: [33, 1, 60], syntaxKind: 1 },
                                { range: [34, 1, 10], syntaxKind: 6 },
                                { range: [34, 11, 16], syntaxKind: 20 },
                                { range: [35, 1, 88], syntaxKind: 1 },
                                { range: [36, 1, 16], syntaxKind: 6 },
                                { range: [36, 17, 22], syntaxKind: 20 },
                                { range: [37, 1, 36], syntaxKind: 1 },
                                { range: [38, 1, 13], syntaxKind: 6 },
                                { range: [38, 14, 19], syntaxKind: 20 },
                                { range: [39, 1, 55], syntaxKind: 1 },
                                { range: [40, 1, 9], syntaxKind: 6 },
                                { range: [40, 10, 15], syntaxKind: 20 },
                                { range: [41, 1, 28], syntaxKind: 1 },
                                { range: [42, 1, 8], syntaxKind: 6 },
                                { range: [42, 9, 15], syntaxKind: 20 },
                                { range: [43, 1, 83], syntaxKind: 1 },
                                { range: [44, 1, 82], syntaxKind: 1 },
                                { range: [45, 1, 82], syntaxKind: 1 },
                                { range: [46, 1, 19], syntaxKind: 1 },
                                { range: [47, 1, 14], syntaxKind: 6 },
                                { range: [47, 15, 20], syntaxKind: 20 },
                                { range: [48, 1, 52], syntaxKind: 1 },
                                { range: [49, 1, 5], syntaxKind: 6 },
                                { range: [49, 8, 12], syntaxKind: 20 },
                                { range: [52, 0, 83], syntaxKind: 1 },
                                { range: [53, 0, 4], syntaxKind: 4 },
                                { range: [53, 5, 9], syntaxKind: 19 },
                                { range: [53, 10, 16], syntaxKind: 4 },
                                { range: [54, 1, 25], syntaxKind: 1 },
                                { range: [55, 1, 6], syntaxKind: 6 },
                                { range: [55, 7, 12], syntaxKind: 20 },
                                { range: [56, 1, 27], syntaxKind: 1 },
                                { range: [57, 1, 8], syntaxKind: 6 },
                                { range: [57, 9, 14], syntaxKind: 20 },
                                { range: [58, 1, 27], syntaxKind: 1 },
                                { range: [59, 1, 8], syntaxKind: 6 },
                                { range: [59, 9, 14], syntaxKind: 20 },
                                { range: [62, 0, 65], syntaxKind: 1 },
                                { range: [63, 0, 29], syntaxKind: 1 },
                                { range: [64, 0, 4], syntaxKind: 4 },
                                { range: [64, 6, 7], syntaxKind: 11 },
                                { range: [64, 8, 9], syntaxKind: 5 },
                                { range: [64, 9, 17], syntaxKind: 19 },
                                { range: [64, 19, 23], syntaxKind: 15 },
                                { range: [64, 26, 30], syntaxKind: 19 },
                                { range: [65, 1, 6], syntaxKind: 6 },
                                { range: [65, 7, 9], syntaxKind: 5 },
                                { range: [65, 10, 14], syntaxKind: 19 },
                                { range: [66, 1, 4], syntaxKind: 4 },
                                { range: [66, 5, 6], syntaxKind: 9 },
                                { range: [66, 8, 9], syntaxKind: 6 },
                                { range: [66, 10, 12], syntaxKind: 5 },
                                { range: [66, 13, 18], syntaxKind: 4 },
                                { range: [66, 19, 20], syntaxKind: 6 },
                                { range: [66, 21, 26], syntaxKind: 6 },
                                { range: [67, 2, 7], syntaxKind: 6 },
                                { range: [67, 8, 11], syntaxKind: 15 },
                                { range: [67, 12, 13], syntaxKind: 6 },
                                { range: [67, 14, 18], syntaxKind: 15 },
                                { range: [69, 1, 7], syntaxKind: 4 },
                                { range: [69, 8, 13], syntaxKind: 6 },
                                { range: [72, 0, 66], syntaxKind: 1 },
                                { range: [73, 0, 8], syntaxKind: 1 },
                                { range: [74, 0, 4], syntaxKind: 4 },
                                { range: [74, 6, 7], syntaxKind: 11 },
                                { range: [74, 8, 9], syntaxKind: 5 },
                                { range: [74, 9, 13], syntaxKind: 19 },
                                { range: [74, 15, 19], syntaxKind: 15 },
                                { range: [74, 22, 26], syntaxKind: 19 },
                                { range: [75, 1, 6], syntaxKind: 6 },
                                { range: [75, 7, 9], syntaxKind: 5 },
                                { range: [75, 10, 15], syntaxKind: 6 },
                                { range: [75, 16, 21], syntaxKind: 15 },
                                { range: [75, 22, 23], syntaxKind: 6 },
                                { range: [75, 24, 28], syntaxKind: 6 },
                                { range: [75, 32, 36], syntaxKind: 20 },
                                { range: [75, 37, 41], syntaxKind: 27 },
                                { range: [76, 1, 4], syntaxKind: 4 },
                                { range: [76, 5, 9], syntaxKind: 6 },
                                { range: [76, 10, 14], syntaxKind: 20 },
                                { range: [77, 1, 3], syntaxKind: 6 },
                                { range: [77, 4, 6], syntaxKind: 5 },
                                { range: [77, 7, 11], syntaxKind: 19 },
                                { range: [78, 1, 4], syntaxKind: 4 },
                                { range: [78, 5, 6], syntaxKind: 9 },
                                { range: [78, 8, 12], syntaxKind: 6 },
                                { range: [78, 13, 15], syntaxKind: 5 },
                                { range: [78, 16, 21], syntaxKind: 4 },
                                { range: [78, 22, 27], syntaxKind: 6 },
                                { range: [79, 2, 4], syntaxKind: 4 },
                                { range: [79, 5, 8], syntaxKind: 7 },
                                { range: [79, 9, 13], syntaxKind: 6 },
                                { range: [79, 15, 17], syntaxKind: 5 },
                                { range: [79, 18, 19], syntaxKind: 32 },
                                { range: [80, 3, 7], syntaxKind: 6 },
                                { range: [80, 8, 9], syntaxKind: 5 },
                                { range: [80, 10, 11], syntaxKind: 32 },
                                { range: [81, 3, 11], syntaxKind: 4 },
                                { range: [83, 2, 8], syntaxKind: 4 },
                                { range: [83, 9, 13], syntaxKind: 6 },
                                { range: [83, 14, 15], syntaxKind: 32 },
                                { range: [84, 2, 6], syntaxKind: 4 },
                                { range: [84, 7, 10], syntaxKind: 27 },
                                { range: [85, 3, 5], syntaxKind: 4 },
                                { range: [85, 6, 10], syntaxKind: 6 },
                                { range: [85, 11, 13], syntaxKind: 5 },
                                { range: [85, 14, 17], syntaxKind: 27 },
                                { range: [86, 4, 6], syntaxKind: 6 },
                                { range: [86, 7, 12], syntaxKind: 6 },
                                { range: [86, 12, 14], syntaxKind: 5 },
                                { range: [87, 4, 6], syntaxKind: 6 },
                                { range: [87, 7, 14], syntaxKind: 6 },
                                { range: [87, 14, 16], syntaxKind: 5 },
                                { range: [88, 4, 8], syntaxKind: 6 },
                                { range: [88, 9, 10], syntaxKind: 5 },
                                { range: [88, 11, 12], syntaxKind: 32 },
                                { range: [88, 13, 78], syntaxKind: 1 },
                                { range: [89, 5, 9], syntaxKind: 4 },
                                { range: [90, 4, 6], syntaxKind: 6 },
                                { range: [90, 7, 14], syntaxKind: 6 },
                                { range: [90, 14, 16], syntaxKind: 5 },
                                { range: [91, 4, 8], syntaxKind: 6 },
                                { range: [91, 9, 10], syntaxKind: 5 },
                                { range: [91, 11, 15], syntaxKind: 6 },
                                { range: [91, 16, 17], syntaxKind: 32 },
                                { range: [93, 2, 6], syntaxKind: 4 },
                                { range: [93, 7, 10], syntaxKind: 27 },
                                { range: [94, 3, 5], syntaxKind: 4 },
                                { range: [94, 6, 10], syntaxKind: 6 },
                                { range: [94, 11, 13], syntaxKind: 5 },
                                { range: [94, 14, 17], syntaxKind: 27 },
                                { range: [95, 4, 6], syntaxKind: 6 },
                                { range: [95, 7, 14], syntaxKind: 6 },
                                { range: [95, 14, 16], syntaxKind: 5 },
                                { range: [96, 4, 6], syntaxKind: 6 },
                                { range: [96, 7, 14], syntaxKind: 6 },
                                { range: [96, 14, 16], syntaxKind: 5 },
                                { range: [97, 4, 8], syntaxKind: 6 },
                                { range: [97, 9, 10], syntaxKind: 5 },
                                { range: [97, 11, 12], syntaxKind: 32 },
                                { range: [97, 13, 78], syntaxKind: 1 },
                                { range: [98, 5, 9], syntaxKind: 4 },
                                { range: [99, 4, 6], syntaxKind: 6 },
                                { range: [99, 7, 12], syntaxKind: 6 },
                                { range: [99, 12, 14], syntaxKind: 5 },
                                { range: [100, 4, 8], syntaxKind: 6 },
                                { range: [100, 9, 10], syntaxKind: 5 },
                                { range: [100, 11, 15], syntaxKind: 6 },
                                { range: [100, 16, 17], syntaxKind: 32 },
                                { range: [102, 2, 9], syntaxKind: 4 },
                                { range: [103, 3, 7], syntaxKind: 6 },
                                { range: [103, 8, 9], syntaxKind: 5 },
                                { range: [103, 10, 11], syntaxKind: 32 },
                                { range: [106, 1, 7], syntaxKind: 4 },
                                { range: [106, 8, 10], syntaxKind: 6 },
                                { range: [109, 0, 3], syntaxKind: 4 },
                                { range: [110, 1, 11], syntaxKind: 6 },
                                { range: [110, 21, 22], syntaxKind: 5 },
                                { range: [110, 25, 29], syntaxKind: 20 },
                                { range: [110, 30, 35], syntaxKind: 27 },
                                { range: [111, 1, 20], syntaxKind: 6 },
                                { range: [111, 21, 22], syntaxKind: 5 },
                                { range: [111, 25, 29], syntaxKind: 20 },
                                { range: [111, 30, 40], syntaxKind: 27 },
                                { range: [114, 0, 5], syntaxKind: 4 },
                                { range: [114, 6, 16], syntaxKind: 9 },
                                { range: [114, 17, 18], syntaxKind: 5 },
                                { range: [114, 19, 40], syntaxKind: 27 },
                                { range: [115, 0, 5], syntaxKind: 4 },
                                { range: [115, 6, 19], syntaxKind: 9 },
                                { range: [115, 20, 21], syntaxKind: 5 },
                                { range: [115, 22, 37], syntaxKind: 27 },
                                { range: [115, 37, 39], syntaxKind: 28 },
                                { range: [115, 39, 40], syntaxKind: 27 },
                                { range: [117, 0, 80], syntaxKind: 1 },
                                { range: [118, 0, 21], syntaxKind: 1 },
                                { range: [119, 0, 85], syntaxKind: 1 },
                                { range: [120, 0, 5], syntaxKind: 4 },
                                { range: [120, 6, 25], syntaxKind: 9 },
                                { range: [120, 26, 27], syntaxKind: 5 },
                                { range: [120, 28, 55], syntaxKind: 27 },
                                { range: [122, 0, 96], syntaxKind: 1 },
                                { range: [123, 0, 21], syntaxKind: 1 },
                                { range: [124, 0, 85], syntaxKind: 1 },
                                { range: [125, 0, 5], syntaxKind: 4 },
                                { range: [125, 6, 26], syntaxKind: 9 },
                                { range: [125, 27, 28], syntaxKind: 5 },
                                { range: [125, 29, 66], syntaxKind: 27 },
                                { range: [127, 0, 4], syntaxKind: 4 },
                                { range: [127, 6, 7], syntaxKind: 11 },
                                { range: [127, 8, 9], syntaxKind: 5 },
                                { range: [127, 9, 13], syntaxKind: 19 },
                                { range: [127, 15, 18], syntaxKind: 15 },
                                { range: [127, 19, 20], syntaxKind: 11 },
                                { range: [127, 21, 25], syntaxKind: 19 },
                                { range: [128, 1, 2], syntaxKind: 6 },
                                { range: [128, 3, 8], syntaxKind: 6 },
                                { range: [128, 9, 11], syntaxKind: 5 },
                                { range: [128, 12, 13], syntaxKind: 6 },
                                { range: [128, 14, 19], syntaxKind: 6 },
                                { range: [129, 1, 2], syntaxKind: 6 },
                                { range: [129, 3, 10], syntaxKind: 6 },
                                { range: [129, 11, 13], syntaxKind: 5 },
                                { range: [129, 14, 15], syntaxKind: 6 },
                                { range: [129, 16, 23], syntaxKind: 6 },
                                { range: [130, 1, 2], syntaxKind: 6 },
                                { range: [130, 3, 10], syntaxKind: 6 },
                                { range: [130, 11, 13], syntaxKind: 5 },
                                { range: [130, 14, 15], syntaxKind: 6 },
                                { range: [130, 16, 23], syntaxKind: 6 },
                            ],
                        }),
                        __typename: 'HighlightedFile',
                    },
                    __typename: 'GitBlob',
                },
                __typename: 'GitCommit',
            },
            __typename: 'Repository',
        },
    },
}

export const defaultProps: ReferencesPanelProps = {
    telemetryService: NOOP_TELEMETRY_SERVICE,
    telemetryRecorder: noOpTelemetryRecorder,
    settingsCascade: {
        subjects: null,
        final: null,
    },
    platformContext: NOOP_PLATFORM_CONTEXT as any,
    fetchHighlightedFileLineRanges: args => {
        if (args.filePath === 'cmd/go-diff/go-diff.go') {
            return of(highlightedLinesGoDiffGo)
        }
        if (args.filePath === 'diff/diff.go') {
            return of(highlightedLinesDiffGo)
        }
        logger.error('attempt to fetch highlighted lines for file without mocks', args.filePath)
        return of([])
    },
    useCodeIntel: ({ variables }: UseCodeIntelParameters): UseCodeIntelResult => {
        const [result, setResult] = useState<UseCodeIntelResult>({
            data: {
                implementations: { endCursor: '', nodes: LocationsGroup.empty },
                prototypes: { endCursor: '', nodes: LocationsGroup.empty },
                references: { endCursor: '', nodes: LocationsGroup.empty },
                definitions: { endCursor: '', nodes: LocationsGroup.empty },
            },
            loading: true,
            referencesHasNextPage: false,
            fetchMoreReferences: () => {},
            fetchMoreReferencesLoading: false,
            implementationsHasNextPage: false,
            fetchMoreImplementationsLoading: false,
            fetchMoreImplementations: () => {},
            prototypesHasNextPage: false,
            fetchMorePrototypesLoading: false,
            fetchMorePrototypes: () => {},
        })
        useQuery<
            UsePreciseCodeIntelForPositionResult,
            UsePreciseCodeIntelForPositionVariables & ConnectionQueryArguments
        >(USE_PRECISE_CODE_INTEL_FOR_POSITION_QUERY, {
            variables,
            notifyOnNetworkStatusChange: false,
            fetchPolicy: 'no-cache',
            skip: !result.loading,
            onCompleted: result => {
                const data = dataOrThrowErrors(asGraphQLResult({ data: result, errors: [] }))
                if (!data?.repository?.commit?.blob?.lsif) {
                    return
                }
                const lsif = data.repository.commit.blob.lsif
                setResult(prevResult => ({
                    ...prevResult,
                    loading: false,
                    data: {
                        implementations: {
                            endCursor: lsif.implementations.pageInfo.endCursor,
                            nodes: new LocationsGroup(lsif.implementations.nodes.map(buildPreciseLocation)),
                        },
                        prototypes: {
                            endCursor: lsif.prototypes.pageInfo.endCursor,
                            nodes: new LocationsGroup(lsif.prototypes.nodes.map(buildPreciseLocation)),
                        },

                        references: {
                            endCursor: lsif.references.pageInfo.endCursor,
                            nodes: new LocationsGroup(lsif.references.nodes.map(buildPreciseLocation)),
                        },
                        definitions: {
                            endCursor: lsif.definitions.pageInfo.endCursor,
                            nodes: new LocationsGroup(lsif.definitions.nodes.map(buildPreciseLocation)),
                        },
                    },
                }))
            },
        })
        return result
    },
}
