import { useState } from 'react'

import { MockedResponse } from '@apollo/client/testing'
import { of } from 'rxjs'

import { logger } from '@sourcegraph/common'
import { getDocumentNode, dataOrThrowErrors, useQuery } from '@sourcegraph/http-client'
import { NOOP_TELEMETRY_SERVICE } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { NOOP_PLATFORM_CONTEXT } from '@sourcegraph/shared/src/testing/searchTestHelpers'

import { ConnectionQueryArguments } from '../components/FilteredConnection'
import { asGraphQLResult } from '../components/FilteredConnection/utils'
import {
    UsePreciseCodeIntelForPositionResult,
    UsePreciseCodeIntelForPositionVariables,
    HighlightResponseFormat,
    LocationFields,
    ReferencesPanelHighlightedBlobVariables,
    ResolveRepoAndRevisionVariables,
} from '../graphql-operations'

import { buildPreciseLocation } from './location'
import { ReferencesPanelProps } from './ReferencesPanel'
import {
    USE_PRECISE_CODE_INTEL_FOR_POSITION_QUERY,
    RESOLVE_REPO_REVISION_BLOB_QUERY,
    FETCH_HIGHLIGHTED_BLOB,
} from './ReferencesPanelQueries'
import { UseCodeIntelParameters, UseCodeIntelResult } from './useCodeIntel'

const goDiffFileContent =
    'package main\n\nimport (\n\t"flag"\n\t"fmt"\n\t"io"\n\t"log"\n\t"os"\n\n\t"github.com/sourcegraph/go-diff/diff"\n)\n\n// A diagnostic program to aid in debugging diff parsing or printing\n// errors.\n\nconst stdin = "\u003Cstdin\u003E"\n\nvar (\n\tdiffPath = flag.String("f", stdin, "filename of diff (default: stdin)")\n\tfileIdx  = flag.Int("i", -1, "if \u003E= 0, only print and report errors from the i\'th file (0-indexed)")\n)\n\nfunc main() {\n\tlog.SetFlags(0)\n\tflag.Parse()\n\n\tvar diffFile *os.File\n\tif *diffPath == stdin {\n\t\tdiffFile = os.Stdin\n\t} else {\n\t\tvar err error\n\t\tdiffFile, err = os.Open(*diffPath)\n\t\tif err != nil {\n\t\t\tlog.Fatal(err)\n\t\t}\n\t}\n\tdefer diffFile.Close()\n\n\tr := diff.NewMultiFileDiffReader(diffFile)\n\tfor i := 0; ; i++ {\n\t\treport := (*fileIdx == -1) || i == *fileIdx // true if -i==-1 or if this is the i\'th file\n\n\t\tlabel := fmt.Sprintf("file(%d)", i)\n\t\tfdiff, err := r.ReadFile()\n\t\tif fdiff != nil {\n\t\t\tlabel = fmt.Sprintf("orig(%s) new(%s)", fdiff.OrigName, fdiff.NewName)\n\t\t}\n\t\tif err == io.EOF {\n\t\t\tbreak\n\t\t}\n\t\tif err != nil {\n\t\t\tif report {\n\t\t\t\tlog.Fatalf("err read %s: %s", label, err)\n\t\t\t} else {\n\t\t\t\tcontinue\n\t\t\t}\n\t\t}\n\n\t\tif report {\n\t\t\tlog.Printf("ok read: %s", label)\n\t\t}\n\n\t\tout, err := diff.PrintFileDiff(fdiff)\n\t\tif err != nil {\n\t\t\tif report {\n\t\t\t\tlog.Fatalf("err print %s: %s", label, err)\n\t\t\t} else {\n\t\t\t\tcontinue\n\t\t\t}\n\t\t}\n\t\tif report {\n\t\t\tif _, err := os.Stdout.Write(out); err != nil {\n\t\t\t\tlog.Fatal(err)\n\t\t\t}\n\t\t}\n\t}\n}\n'
const diffFileContent =
    "package diff\n\nimport (\n\t\"bytes\"\n\t\"time\"\n)\n\n// A FileDiff represents a unified diff for a single file.\n//\n// A file unified diff has a header that resembles the following:\n//\n//   --- oldname\t2009-10-11 15:12:20.000000000 -0700\n//   +++ newname\t2009-10-11 15:12:30.000000000 -0700\ntype FileDiff struct {\n\t// the original name of the file\n\tOrigName string\n\t// the original timestamp (nil if not present)\n\tOrigTime *time.Time\n\t// the new name of the file (often same as OrigName)\n\tNewName string\n\t// the new timestamp (nil if not present)\n\tNewTime *time.Time\n\t// extended header lines (e.g., git's \"new mode \u003Cmode\u003E\", \"rename from \u003Cpath\u003E\", etc.)\n\tExtended []string\n\t// hunks that were changed from orig to new\n\tHunks []*Hunk\n}\n\n// A Hunk represents a series of changes (additions or deletions) in a file's\n// unified diff.\ntype Hunk struct {\n\t// starting line number in original file\n\tOrigStartLine int32\n\t// number of lines the hunk applies to in the original file\n\tOrigLines int32\n\t// if \u003E 0, then the original file had a 'No newline at end of file' mark at this offset\n\tOrigNoNewlineAt int32\n\t// starting line number in new file\n\tNewStartLine int32\n\t// number of lines the hunk applies to in the new file\n\tNewLines int32\n\t// optional section heading\n\tSection string\n\t// 0-indexed line offset in unified file diff (including section headers); this is\n\t// only set when Hunks are read from entire file diff (i.e., when ReadAllHunks is\n\t// called) This accounts for hunk headers, too, so the StartPosition of the first\n\t// hunk will be 1.\n\tStartPosition int32\n\t// hunk body (lines prefixed with '-', '+', or ' ')\n\tBody []byte\n}\n\n// A Stat is a diff stat that represents the number of lines added/changed/deleted.\ntype Stat struct {\n\t// number of lines added\n\tAdded int32\n\t// number of lines changed\n\tChanged int32\n\t// number of lines deleted\n\tDeleted int32\n}\n\n// Stat computes the number of lines added/changed/deleted in all\n// hunks in this file's diff.\nfunc (d *FileDiff) Stat() Stat {\n\ttotal := Stat{}\n\tfor _, h := range d.Hunks {\n\t\ttotal.add(h.Stat())\n\t}\n\treturn total\n}\n\n// Stat computes the number of lines added/changed/deleted in this\n// hunk.\nfunc (h *Hunk) Stat() Stat {\n\tlines := bytes.Split(h.Body, []byte{'\\n'})\n\tvar last byte\n\tst := Stat{}\n\tfor _, line := range lines {\n\t\tif len(line) == 0 {\n\t\t\tlast = 0\n\t\t\tcontinue\n\t\t}\n\t\tswitch line[0] {\n\t\tcase '-':\n\t\t\tif last == '+' {\n\t\t\t\tst.Added--\n\t\t\t\tst.Changed++\n\t\t\t\tlast = 0 // next line can't change this one since this is already a change\n\t\t\t} else {\n\t\t\t\tst.Deleted++\n\t\t\t\tlast = line[0]\n\t\t\t}\n\t\tcase '+':\n\t\t\tif last == '-' {\n\t\t\t\tst.Deleted--\n\t\t\t\tst.Changed++\n\t\t\t\tlast = 0 // next line can't change this one since this is already a change\n\t\t\t} else {\n\t\t\t\tst.Added++\n\t\t\t\tlast = line[0]\n\t\t\t}\n\t\tdefault:\n\t\t\tlast = 0\n\t\t}\n\t}\n\treturn st\n}\n\nvar (\n\thunkPrefix          = []byte(\"@@ \")\n\tonlyInMessagePrefix = []byte(\"Only in \")\n)\n\nconst hunkHeader = \"@@ -%d,%d +%d,%d @@\"\nconst onlyInMessage = \"Only in %s: %s\\n\"\n\n// diffTimeParseLayout is the layout used to parse the time in unified diff file\n// header timestamps.\n// See https://www.gnu.org/software/diffutils/manual/html_node/Detailed-Unified.html.\nconst diffTimeParseLayout = \"2006-01-02 15:04:05 -0700\"\n\n// diffTimeFormatLayout is the layout used to format (i.e., print) the time in unified diff file\n// header timestamps.\n// See https://www.gnu.org/software/diffutils/manual/html_node/Detailed-Unified.html.\nconst diffTimeFormatLayout = \"2006-01-02 15:04:05.000000000 -0700\"\n\nfunc (s *Stat) add(o Stat) {\n\ts.Added += o.Added\n\ts.Changed += o.Changed\n\ts.Deleted += o.Deleted\n}\n"
const printFileContent =
    'package diff\n\nimport (\n\t"bytes"\n\t"fmt"\n\t"io"\n\t"path/filepath"\n\t"time"\n)\n\n// PrintMultiFileDiff prints a multi-file diff in unified diff format.\nfunc PrintMultiFileDiff(ds []*FileDiff) ([]byte, error) {\n\tvar buf bytes.Buffer\n\tfor _, d := range ds {\n\t\tdiff, err := PrintFileDiff(d)\n\t\tif err != nil {\n\t\t\treturn nil, err\n\t\t}\n\t\tif _, err := buf.Write(diff); err != nil {\n\t\t\treturn nil, err\n\t\t}\n\t}\n\treturn buf.Bytes(), nil\n}\n\n// PrintFileDiff prints a FileDiff in unified diff format.\n//\n// TODO(sqs): handle escaping whitespace/etc. chars in filenames\nfunc PrintFileDiff(d *FileDiff) ([]byte, error) {\n\tvar buf bytes.Buffer\n\n\tfor _, xheader := range d.Extended {\n\t\tif _, err := fmt.Fprintln(\u0026buf, xheader); err != nil {\n\t\t\treturn nil, err\n\t\t}\n\t}\n\n\t// FileDiff is added/deleted file\n\t// No further hunks printing needed\n\tif d.NewName == "" {\n\t\t_, err := fmt.Fprintf(\u0026buf, onlyInMessage, filepath.Dir(d.OrigName), filepath.Base(d.OrigName))\n\t\tif err != nil {\n\t\t\treturn nil, err\n\t\t}\n\t\treturn buf.Bytes(), nil\n\t}\n\n\tif d.Hunks == nil {\n\t\treturn buf.Bytes(), nil\n\t}\n\n\tif err := printFileHeader(\u0026buf, "--- ", d.OrigName, d.OrigTime); err != nil {\n\t\treturn nil, err\n\t}\n\tif err := printFileHeader(\u0026buf, "+++ ", d.NewName, d.NewTime); err != nil {\n\t\treturn nil, err\n\t}\n\n\tph, err := PrintHunks(d.Hunks)\n\tif err != nil {\n\t\treturn nil, err\n\t}\n\n\tif _, err := buf.Write(ph); err != nil {\n\t\treturn nil, err\n\t}\n\treturn buf.Bytes(), nil\n}\n\nfunc printFileHeader(w io.Writer, prefix string, filename string, timestamp *time.Time) error {\n\tif _, err := fmt.Fprint(w, prefix, filename); err != nil {\n\t\treturn err\n\t}\n\tif timestamp != nil {\n\t\tif _, err := fmt.Fprint(w, "\\t", timestamp.Format(diffTimeFormatLayout)); err != nil {\n\t\t\treturn err\n\t\t}\n\t}\n\tif _, err := fmt.Fprintln(w); err != nil {\n\t\treturn err\n\t}\n\treturn nil\n}\n\n// PrintHunks prints diff hunks in unified diff format.\nfunc PrintHunks(hunks []*Hunk) ([]byte, error) {\n\tvar buf bytes.Buffer\n\tfor _, hunk := range hunks {\n\t\t_, err := fmt.Fprintf(\u0026buf,\n\t\t\t"@@ -%d,%d +%d,%d @@", hunk.OrigStartLine, hunk.OrigLines, hunk.NewStartLine, hunk.NewLines,\n\t\t)\n\t\tif err != nil {\n\t\t\treturn nil, err\n\t\t}\n\t\tif hunk.Section != "" {\n\t\t\t_, err := fmt.Fprint(\u0026buf, " ", hunk.Section)\n\t\t\tif err != nil {\n\t\t\t\treturn nil, err\n\t\t\t}\n\t\t}\n\t\tif _, err := fmt.Fprintln(\u0026buf); err != nil {\n\t\t\treturn nil, err\n\t\t}\n\n\t\tif hunk.OrigNoNewlineAt == 0 {\n\t\t\tif _, err := buf.Write(hunk.Body); err != nil {\n\t\t\t\treturn nil, err\n\t\t\t}\n\t\t} else {\n\t\t\tif _, err := buf.Write(hunk.Body[:hunk.OrigNoNewlineAt]); err != nil {\n\t\t\t\treturn nil, err\n\t\t\t}\n\t\t\tif err := printNoNewlineMessage(\u0026buf); err != nil {\n\t\t\t\treturn nil, err\n\t\t\t}\n\t\t\tif _, err := buf.Write(hunk.Body[hunk.OrigNoNewlineAt:]); err != nil {\n\t\t\t\treturn nil, err\n\t\t\t}\n\t\t}\n\n\t\tif !bytes.HasSuffix(hunk.Body, []byte{\'\\n\'}) {\n\t\t\tif _, err := fmt.Fprintln(\u0026buf); err != nil {\n\t\t\t\treturn nil, err\n\t\t\t}\n\t\t\tif err := printNoNewlineMessage(\u0026buf); err != nil {\n\t\t\t\treturn nil, err\n\t\t\t}\n\t\t}\n\t}\n\treturn buf.Bytes(), nil\n}\n\nfunc printNoNewlineMessage(w io.Writer) error {\n\tif _, err := w.Write([]byte(noNewlineMessage)); err != nil {\n\t\treturn err\n\t}\n\tif _, err := fmt.Fprintln(w); err != nil {\n\t\treturn err\n\t}\n\treturn nil\n}\n'

// highlightedDiffFileContent is a shortened version of the highlighted
// contents of diff/diff.go so it's easier to manage here.
const highlightedDiffFileContent = [
    '<table>',
    '<tbody>',
    '<tr><td class="line" data-line="1"/><td class="code"><div><span class="hl-source hl-go"><span class="hl-keyword hl-other hl-package hl-go">package</span> <span class="hl-variable hl-other hl-go">diff</span>\n</span></div></td></tr>',
    '<tr><td class="line" data-line="2"/><td class="code"><div><span class="hl-source hl-go">\n</span></div></td></tr>',
    '<tr><td class="line" data-line="3"/><td class="code"><div><span class="hl-source hl-go"><span class="hl-keyword hl-other hl-import hl-go">import</span> <span class="hl-punctuation hl-section hl-parens hl-begin hl-go">(</span>\n</span></div></td></tr><tr><td class="line" data-line="4"/><td class="code"><div><span class="hl-source hl-go">\t<span class="hl-string hl-quoted hl-double hl-go"><span class="hl-punctuation hl-definition hl-string hl-begin hl-go">\u0026quot;</span>bytes<span class="hl-punctuation hl-definition hl-string hl-end hl-go">\u0026quot;</span></span>\n</span></div></td></tr>',
    '<tr><td class="line" data-line="5"/><td class="code"><div><span class="hl-source hl-go">\t<span class="hl-string hl-quoted hl-double hl-go"><span class="hl-punctuation hl-definition hl-string hl-begin hl-go">\u0026quot;</span>time<span class="hl-punctuation hl-definition hl-string hl-end hl-go">\u0026quot;</span></span>\n</span></div></td></tr>',
    '<tr><td class="line" data-line="6"/><td class="code"><div><span class="hl-source hl-go"><span class="hl-punctuation hl-section hl-parens hl-end hl-go">)</span>\n</span></div></td></tr>',
    '<tr><td class="line" data-line="7"/><td class="code"><div><span class="hl-source hl-go">\n</span></div></td></tr>',
    '<tr><td class="line" data-line="8"/><td class="code"><div><span class="hl-source hl-go"><span class="hl-comment hl-line hl-go"><span class="hl-punctuation hl-definition hl-comment hl-go">//</span> A FileDiff represents a unified diff for a single file.\n</span></span></div></td></tr>',
    '<tr><td class="line" data-line="9"/><td class="code"><div><span class="hl-source hl-go"><span class="hl-comment hl-line hl-go"><span class="hl-punctuation hl-definition hl-comment hl-go">//</span>\n</span></span></div></td></tr>',
    '<tr><td class="line" data-line="10"/><td class="code"><div><span class="hl-source hl-go"><span class="hl-comment hl-line hl-go"><span class="hl-punctuation hl-definition hl-comment hl-go">//</span> A file unified diff has a header that resembles the following:\n</span></span></div></td></tr>',
    '<tr><td class="line" data-line="11"/><td class="code"><div><span class="hl-source hl-go"><span class="hl-comment hl-line hl-go"><span class="hl-punctuation hl-definition hl-comment hl-go">//</span>\n</span></span></div></td></tr>',
    '<tr><td class="line" data-line="12"/><td class="code"><div><span class="hl-source hl-go"><span class="hl-comment hl-line hl-go"><span class="hl-punctuation hl-definition hl-comment hl-go">//</span>   --- oldname\t2009-10-11 15:12:20.000000000 -0700\n</span></span></div></td></tr>',
    '<tr><td class="line" data-line="13"/><td class="code"><div><span class="hl-source hl-go"><span class="hl-comment hl-line hl-go"><span class="hl-punctuation hl-definition hl-comment hl-go">//</span>   +++ newname\t2009-10-11 15:12:30.000000000 -0700\n</span></span></div></td></tr>',
    '<tr><td class="line" data-line="14"/><td class="code"><div><span class="hl-source hl-go"><span class="hl-storage hl-type hl-keyword hl-type hl-go">type</span> <span class="hl-entity hl-name hl-type hl-go">FileDiff</span> <span class="hl-storage hl-type hl-keyword hl-struct hl-go">struct</span> <span class="hl-meta hl-type hl-go"><span class="hl-punctuation hl-section hl-braces hl-begin hl-go">{</span>\n</span></span></div></td></tr>',
    '<tr><td class="line" data-line="15"/><td class="code"><div><span class="hl-source hl-go"><span class="hl-meta hl-type hl-go">\t<span class="hl-comment hl-line hl-go"><span class="hl-punctuation hl-definition hl-comment hl-go">//</span> the original name of the file\n</span></span></span></div></td></tr>',
    '<tr><td class="line" data-line="16"/><td class="code"><div><span class="hl-source hl-go"><span class="hl-meta hl-type hl-go">\t<span class="hl-variable hl-other hl-member hl-declaration hl-go">OrigName</span> <span class="hl-storage hl-type hl-go"><span class="hl-support hl-type hl-builtin hl-go">string</span></span>\n</span></span></div></td></tr>',
    '<tr><td class="line" data-line="17"/><td class="code"><div><span class="hl-source hl-go"><span class="hl-meta hl-type hl-go">\t<span class="hl-comment hl-line hl-go"><span class="hl-punctuation hl-definition hl-comment hl-go">//</span> the original timestamp (nil if not present)\n</span></span></span></div></td></tr>',
    '<tr><td class="line" data-line="18"/><td class="code"><div><span class="hl-source hl-go"><span class="hl-meta hl-type hl-go">\t<span class="hl-variable hl-other hl-member hl-declaration hl-go">OrigTime</span> <span class="hl-keyword hl-operator hl-go">*</span><span class="hl-variable hl-other hl-go">time</span><span class="hl-punctuation hl-accessor hl-dot hl-go">.</span><span class="hl-storage hl-type hl-go">Time</span>\n</span></span></div></td></tr>',
    '<tr><td class="line" data-line="19"/><td class="code"><div><span class="hl-source hl-go"><span class="hl-meta hl-type hl-go">\t<span class="hl-comment hl-line hl-go"><span class="hl-punctuation hl-definition hl-comment hl-go">//</span> the new name of the file (often same as OrigName)\n</span></span></span></div></td></tr>',
    '<tr><td class="line" data-line="20"/><td class="code"><div><span class="hl-source hl-go"><span class="hl-meta hl-type hl-go">\t<span class="hl-variable hl-other hl-member hl-declaration hl-go">NewName</span> <span class="hl-storage hl-type hl-go"><span class="hl-support hl-type hl-builtin hl-go">string</span></span>\n</span></span></div></td></tr>',
    '<tr><td class="line" data-line="21"/><td class="code"><div><span class="hl-source hl-go"><span class="hl-meta hl-type hl-go">\t<span class="hl-comment hl-line hl-go"><span class="hl-punctuation hl-definition hl-comment hl-go">//</span> the new timestamp (nil if not present)\n</span></span></span></div></td></tr>',
    '<tr><td class="line" data-line="22"/><td class="code"><div><span class="hl-source hl-go"><span class="hl-meta hl-type hl-go">\t<span class="hl-variable hl-other hl-member hl-declaration hl-go">NewTime</span> <span class="hl-keyword hl-operator hl-go">*</span><span class="hl-variable hl-other hl-go">time</span><span class="hl-punctuation hl-accessor hl-dot hl-go">.</span><span class="hl-storage hl-type hl-go">Time</span>\n</span></span></div></td></tr>',
    '<tr><td class="line" data-line="23"/><td class="code"><div><span class="hl-source hl-go"><span class="hl-meta hl-type hl-go">\t<span class="hl-comment hl-line hl-go"><span class="hl-punctuation hl-definition hl-comment hl-go">//</span> extended header lines (e.g., git\u0026#39;s \u0026quot;new mode \u0026lt;mode\u0026gt;\u0026quot;, \u0026quot;rename from \u0026lt;path\u0026gt;\u0026quot;, etc.)\n</span></span></span></div></td></tr>',
    '<tr><td class="line" data-line="24"/><td class="code"><div><span class="hl-source hl-go"><span class="hl-meta hl-type hl-go">\t<span class="hl-variable hl-other hl-member hl-declaration hl-go">Extended</span> <span class="hl-punctuation hl-section hl-brackets hl-begin hl-go">[</span><span class="hl-punctuation hl-section hl-brackets hl-end hl-go">]</span><span class="hl-storage hl-type hl-go"><span class="hl-support hl-type hl-builtin hl-go">string</span></span>\n</span></span></div></td></tr>',
    '<tr><td class="line" data-line="25"/><td class="code"><div><span class="hl-source hl-go"><span class="hl-meta hl-type hl-go">\t<span class="hl-comment hl-line hl-go"><span class="hl-punctuation hl-definition hl-comment hl-go">//</span> hunks that were changed from orig to new\n</span></span></span></div></td></tr>',
    '</tbody>',
    '</table>',
].join('')

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
        format: HighlightResponseFormat.HTML_HIGHLIGHT,
        html: true,
    }

    return {
        url: `/${repoName}@${commit}/-/blob/${path}?L${line}:${character}&subtree=true#tab=references`,
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
                    content: 'content',
                    highlight: {
                        aborted: false,
                        html: highlightedDiffFileContent,
                        lsif: false,
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
    settingsCascade: {
        subjects: null,
        final: {
            // TODO: we won't need to set experimental features explicitly once we cover CodeMirror side blob view with tests:
            // https://github.com/sourcegraph/sourcegraph/issues/48049
            experimentalFeatures: {
                enableCodeMirrorFileView: false,
            },
        },
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
                implementations: { endCursor: '', nodes: [] },
                references: { endCursor: '', nodes: [] },
                definitions: { endCursor: '', nodes: [] },
            },
            loading: true,
            referencesHasNextPage: false,
            fetchMoreReferences: () => {},
            fetchMoreReferencesLoading: false,
            implementationsHasNextPage: false,
            fetchMoreImplementationsLoading: false,
            fetchMoreImplementations: () => {},
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
                if (!data || !data.repository?.commit?.blob?.lsif) {
                    return
                }
                const lsif = data.repository.commit.blob.lsif
                setResult(prevResult => ({
                    ...prevResult,
                    loading: false,
                    data: {
                        implementations: {
                            endCursor: lsif.implementations.pageInfo.endCursor,
                            nodes: lsif.implementations.nodes.map(buildPreciseLocation),
                        },
                        references: {
                            endCursor: lsif.references.pageInfo.endCursor,
                            nodes: lsif.references.nodes.map(buildPreciseLocation),
                        },
                        definitions: {
                            endCursor: lsif.definitions.pageInfo.endCursor,
                            nodes: lsif.definitions.nodes.map(buildPreciseLocation),
                        },
                    },
                }))
            },
        })
        return result
    },
}
