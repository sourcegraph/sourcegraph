import { noop } from 'lodash'
import { EMPTY, NEVER, of, Subscription } from 'rxjs'
import sinon from 'sinon'

import type { FlatExtensionHostAPI } from '../api/contract'
import { pretendProxySubscribable, pretendRemote } from '../api/util'
import type { FetchFileParameters } from '../backend/file'
import type { Controller } from '../extensions/controller'
import type { PlatformContext } from '../platform/context'
import type { AggregateStreamingSearchResults, ContentMatch, RepositoryMatch } from '../search/stream'
import type { SettingsCascade } from '../settings/settings'
import { noOpTelemetryRecorder } from '../telemetry'

export const CHUNK_MATCH_RESULT: ContentMatch = {
    type: 'content',
    path: '.travis.yml',
    repository: 'github.com/golang/oauth2',
    chunkMatches: [
        {
            content: '  - go test -v golang.org/x/oauth2/...',
            contentStart: {
                line: 12,
                offset: 12,
                column: 0,
            },
            ranges: [
                {
                    start: {
                        line: 12,
                        offset: 19,
                        column: 7,
                    },
                    end: {
                        line: 12,
                        offset: 23,
                        column: 11,
                    },
                },
            ],
        },
    ],
}

export const LINE_MATCH_RESULT: ContentMatch = {
    type: 'content',
    path: '.travis.yml',
    repository: 'github.com/golang/oauth2',
    lineMatches: [
        {
            line: '  - go test -v golang.org/x/oauth2/...',
            lineNumber: 12,
            offsetAndLengths: [[7, 4]],
        },
    ],
}

export const REPO_MATCH_RESULT: RepositoryMatch = {
    type: 'repo',
    repository: 'github.com/golang/oauth2',
    metadata: {
        'open-source': undefined,
        authentication: undefined,
    },
}

export const REPO_MATCH_RESULTS_WITH_METADATA: RepositoryMatch[] = [
    {
        type: 'repo',
        repository: 'github.com/golang/oauth2',
        description: 'The Go package for OAuth2.',
    },
    {
        type: 'repo',
        repository: 'github.com/sourcegraph/sourcegraph',
        description: 'Universtal code search',
        repoStars: 123,
        repoLastFetched: '2017-01-01T00:00:00Z',
        private: true,
    },
    {
        type: 'repo',
        repository: 'github.com/sourcegraph/go-langserver',
        description: 'Go language server',
        repoStars: 9000,
        fork: true,
        archived: true,
    },
]

export const MULTIPLE_MATCH_RESULT: ContentMatch = {
    type: 'content',
    path: 'clientcredentials/clientcredentials_test.go',
    repository: 'github.com/golang/oauth2',
    chunkMatches: [
        {
            content: '\t"net/http/httptest"',
            contentStart: {
                offset: 238,
                line: 11,
                column: 0,
            },
            ranges: [
                {
                    start: {
                        offset: 253,
                        line: 11,
                        column: 15,
                    },
                    end: {
                        offset: 257,
                        line: 11,
                        column: 19,
                    },
                },
            ],
        },
        {
            content: '\t"testing"',
            contentStart: {
                offset: 270,
                line: 13,
                column: 0,
            },
            ranges: [
                {
                    start: {
                        offset: 272,
                        line: 13,
                        column: 2,
                    },
                    end: {
                        offset: 276,
                        line: 13,
                        column: 6,
                    },
                },
            ],
        },
        {
            content: 'func TestTokenSourceGrantTypeOverride(t *testing.T) {',
            contentStart: {
                offset: 793,
                line: 36,
                column: 0,
            },
            ranges: [
                {
                    start: {
                        offset: 798,
                        line: 36,
                        column: 5,
                    },
                    end: {
                        offset: 802,
                        line: 36,
                        column: 9,
                    },
                },
                {
                    start: {
                        offset: 834,
                        line: 36,
                        column: 41,
                    },
                    end: {
                        offset: 838,
                        line: 36,
                        column: 45,
                    },
                },
            ],
        },
        {
            content: '\tts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {',
            contentStart: {
                offset: 901,
                line: 39,
                column: 0,
            },
            ranges: [
                {
                    start: {
                        offset: 912,
                        line: 39,
                        column: 11,
                    },
                    end: {
                        offset: 916,
                        line: 39,
                        column: 15,
                    },
                },
            ],
        },
        {
            content: 'func TestTokenRequest(t *testing.T) {',
            contentStart: {
                offset: 2084,
                line: 73,
                column: 0,
            },
            ranges: [
                {
                    start: {
                        offset: 2089,
                        line: 73,
                        column: 5,
                    },
                    end: {
                        offset: 2093,
                        line: 73,
                        column: 9,
                    },
                },
                {
                    start: {
                        offset: 2109,
                        line: 73,
                        column: 25,
                    },
                    end: {
                        offset: 2113,
                        line: 73,
                        column: 29,
                    },
                },
            ],
        },
        {
            content: '\tts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {',
            contentStart: {
                offset: 2122,
                line: 74,
                column: 0,
            },
            ranges: [
                {
                    start: {
                        offset: 2133,
                        line: 74,
                        column: 11,
                    },
                    end: {
                        offset: 2137,
                        line: 74,
                        column: 15,
                    },
                },
            ],
        },
        {
            content: 'func TestTokenRefreshRequest(t *testing.T) {',
            contentStart: {
                offset: 3663,
                line: 115,
                column: 0,
            },
            ranges: [
                {
                    start: {
                        offset: 3668,
                        line: 115,
                        column: 5,
                    },
                    end: {
                        offset: 3672,
                        line: 115,
                        column: 9,
                    },
                },
                {
                    start: {
                        offset: 3695,
                        line: 115,
                        column: 32,
                    },
                    end: {
                        offset: 3699,
                        line: 115,
                        column: 36,
                    },
                },
            ],
        },
        {
            content: '\tts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {',
            contentStart: {
                offset: 3735,
                line: 117,
                column: 0,
            },
            ranges: [
                {
                    start: {
                        offset: 3746,
                        line: 117,
                        column: 11,
                    },
                    end: {
                        offset: 3750,
                        line: 117,
                        column: 15,
                    },
                },
            ],
        },
        {
            content: '\t\tio.WriteString(w, `{"access_token": "foo", "refresh_token": "bar"}`)',
            contentStart: {
                offset: 4469,
                line: 134,
                column: 0,
            },
            ranges: [
                {
                    start: {
                        offset: 4477,
                        line: 134,
                        column: 8,
                    },
                    end: {
                        offset: 4481,
                        line: 134,
                        column: 12,
                    },
                },
            ],
        },
    ],
}

export const SEARCH_RESULT: AggregateStreamingSearchResults = {
    state: 'complete',
    progress: {
        durationMs: 78,
        matchCount: 1,
        skipped: [],
    },
    filters: [
        { value: 'file:\\.yml$', label: 'YAML', count: 1, exhaustive: true, kind: 'file' },
        { value: 'case:yes', label: 'Make search case sensitive', count: 0, exhaustive: true, kind: 'utility' },
        {
            value: 'repo:^github\\.com/golang/oauth2$',
            label: 'github.com/golang/oauth2',
            count: 1,
            exhaustive: true,
            kind: 'repo',
        },
    ],
    results: [CHUNK_MATCH_RESULT],
}

export const MULTIPLE_SEARCH_RESULT: AggregateStreamingSearchResults = {
    ...SEARCH_RESULT,
    progress: {
        durationMs: 78,
        matchCount: 136,
        skipped: [],
    },
    results: [
        CHUNK_MATCH_RESULT,
        MULTIPLE_MATCH_RESULT,
        {
            type: 'content',
            path: 'example_test.go',
            commit: 'abcd1234',
            repository: 'github.com/golang/oauth2',
            chunkMatches: [
                {
                    content: 'package oauth2_test',
                    contentStart: {
                        offset: 160,
                        line: 4,
                        column: 0,
                    },
                    ranges: [
                        {
                            start: {
                                offset: 175,
                                line: 4,
                                column: 15,
                            },
                            end: {
                                offset: 179,
                                line: 4,
                                column: 19,
                            },
                        },
                    ],
                },
            ],
            repoStars: 42,
            repoLastFetched: '2017-01-01T00:00:00Z',
        },
    ],
}

export const COLLAPSABLE_SEARCH_RESULT: AggregateStreamingSearchResults = {
    ...SEARCH_RESULT,
    progress: {
        durationMs: 78,
        matchCount: 136,
        skipped: [],
    },
    results: [
        CHUNK_MATCH_RESULT,
        MULTIPLE_MATCH_RESULT,
        {
            type: 'content',
            path: 'example_test.go',
            commit: 'abcd1234',
            repository: 'github.com/golang/oauth2',
            chunkMatches: [
                {
                    content: 'package oauth2_test',
                    contentStart: {
                        offset: 160,
                        line: 4,
                        column: 0,
                    },
                    ranges: [
                        {
                            start: {
                                offset: 175,
                                line: 4,
                                column: 15,
                            },
                            end: {
                                offset: 179,
                                line: 4,
                                column: 19,
                            },
                        },
                        {
                            start: {
                                offset: 175,
                                line: 4,
                                column: 15,
                            },
                            end: {
                                offset: 179,
                                line: 4,
                                column: 19,
                            },
                        },
                        {
                            start: {
                                offset: 175,
                                line: 4,
                                column: 15,
                            },
                            end: {
                                offset: 179,
                                line: 4,
                                column: 19,
                            },
                        },
                        {
                            start: {
                                offset: 175,
                                line: 4,
                                column: 15,
                            },
                            end: {
                                offset: 179,
                                line: 4,
                                column: 19,
                            },
                        },
                        {
                            start: {
                                offset: 175,
                                line: 4,
                                column: 15,
                            },
                            end: {
                                offset: 179,
                                line: 4,
                                column: 19,
                            },
                        },
                        {
                            start: {
                                offset: 175,
                                line: 4,
                                column: 15,
                            },
                            end: {
                                offset: 179,
                                line: 4,
                                column: 19,
                            },
                        },
                        {
                            start: {
                                offset: 175,
                                line: 4,
                                column: 15,
                            },
                            end: {
                                offset: 179,
                                line: 4,
                                column: 19,
                            },
                        },
                        {
                            start: {
                                offset: 175,
                                line: 4,
                                column: 15,
                            },
                            end: {
                                offset: 179,
                                line: 4,
                                column: 19,
                            },
                        },
                        {
                            start: {
                                offset: 175,
                                line: 4,
                                column: 15,
                            },
                            end: {
                                offset: 179,
                                line: 4,
                                column: 19,
                            },
                        },
                        {
                            start: {
                                offset: 175,
                                line: 4,
                                column: 15,
                            },
                            end: {
                                offset: 179,
                                line: 4,
                                column: 19,
                            },
                        },
                    ],
                },
            ],
            repoStars: 42,
            repoLastFetched: '2017-01-01T00:00:00Z',
        },
    ],
}

export const HIGHLIGHTED_FILE_LINES = [
    '<tr><td class="line" data-line="1"></td><td class="code"><span style="color:#268bd2;">language</span><span style="color:#657b83;">: </span><span style="color:#2aa198;">go↵</span></td></tr>',
    '<tr><td class="line" data-line="2"></td><td class="code"><span style="color:#657b83;">↵</span></td></tr>',
    '<tr><td class="line" data-line="3"></td><td class="code"><span style="color:#268bd2;">go</span><span style="color:#657b83;">:↵</span></td></tr>',
    '<tr><td class="line" data-line="4"></td><td class="code"><span style="color:#657b83;">  - </span><span style="color:#2aa198;">tip↵</span></td></tr>',
    '<tr><td class="line" data-line="5"></td><td class="code"><span style="color:#657b83;">↵</span></td></tr>',
    '<tr><td class="line" data-line="6"></td><td class="code"><span style="color:#268bd2;">install</span><span style="color:#657b83;">:↵</span></td></tr>',
    '<tr><td class="line" data-line="7"></td><td class="code"><span style="color:#657b83;">  - </span><span style="color:#2aa198;">export GOPATH=&#34;$HOME/gopath&#34;↵</span></td></tr>',
    '<tr><td class="line" data-line="8"></td><td class="code"><span style="color:#657b83;">  - </span><span style="color:#2aa198;">mkdir -p &#34;$GOPATH/src/golang.org/x&#34;↵</span></td></tr>',
    '<tr><td class="line" data-line="9"></td><td class="code"><span style="color:#657b83;">  - </span><span style="color:#2aa198;">mv &#34;$TRAVIS_BUILD_DIR&#34; &#34;$GOPATH/src/golang.org/x/oauth2&#34;↵</span></td></tr>',
    '<tr><td class="line" data-line="10"></td><td class="code"><span style="color:#657b83;">  - </span><span style="color:#2aa198;">go get -v -t -d golang.org/x/oauth2/...↵</span></td></tr><tr>',
    '<td class="line" data-line="11"></td><td class="code"><span style="color:#657b83;">↵</span></td></tr>',
    '<tr><td class="line" data-line="12"></td><td class="code"><span style="color:#268bd2;">script</span><span style="color:#657b83;">:↵</span></td></tr><tr>',
    '<td class="line" data-line="13"></td><td class="code"><span style="color:#657b83;">  - </span><span style="color:#2aa198;">go test -v golang.org/x/oauth2/...</span></td></tr>',
]

export const HIGHLIGHTED_FILE_LINES_SIMPLE = [
    '<tr><td class="line" data-line="1"></td><td class="code">first line of code</td></tr>',
    '<tr><td class="line" data-line="2"></td><td class="code">second line of code</td></tr>',
    '<tr><td class="line" data-line="3"></td><td class="code">third line of code</td></tr>',
    '<tr><td class="line" data-line="4"></td><td class="code">fourth</td></tr>',
    '<tr><td class="line" data-line="5"></td><td class="code">fifth</td></tr>',
    '<tr><td class="line" data-line="6"></td><td class="code">sixth</td></tr>',
    '<tr><td class="line" data-line="7"></td><td class="code">seventh</td></tr>',
    '<tr><td class="line" data-line="8"></td><td class="code">eighth</td></tr>',
    '<tr><td class="line" data-line="9"></td><td class="code">ninth</td></tr>',
    '<tr><td class="line" data-line="10"></td><td class="code">tenth</td></tr>',
]

export const HIGHLIGHTED_FILE_LINES_LONG = [
    '<tr><td class="line" data-line="1"></td><td class="code"><div><span style="color:#93a1a1;">// Copyright 2014 The Go Authors. All rights reserved.↵</span></div></td></tr>',
    '<tr><td class="line" data-line="2"></td><td class="code"><div><span style="color:#93a1a1;">// Use of this source code is governed by a BSD-style↵</span></div></td></tr>',
    '<tr><td class="line" data-line="3"></td><td class="code"><div><span style="color:#93a1a1;">// license that can be found in the LICENSE file.↵</span></div></td></tr>',
    '<tr><td class="line" data-line="4"></td><td class="code"><div><span style="color:#657b83;">↵</span></div></td></tr>',
    '<tr><td class="line" data-line="5"></td><td class="code"><div><span style="color:#859900;">package</span><span style="color:#657b83;"> oauth2_test↵</span></div></td></tr>',
    '<tr><td class="line" data-line="6"></td><td class="code"><div><span style="color:#657b83;">↵</span></div></td></tr>',
    '<tr><td class="line" data-line="7"></td><td class="code"><div><span style="color:#cb4b16;">import </span><span style="color:#657b83;">(↵</span></div></td></tr>',
    '<tr><td class="line" data-line="8"></td><td class="code"><div><span style="color:#657b83;">	</span><span style="color:#839496;">&#34;</span><span style="color:#2aa198;">context</span><span style="color:#839496;">&#34;↵</span></div></td></tr>',
    '<tr><td class="line" data-line="9"></td><td class="code"><div><span style="color:#657b83;">	</span><span style="color:#839496;">&#34;</span><span style="color:#2aa198;">fmt</span><span style="color:#839496;">&#34;↵</span></div></td></tr>',
    '<tr><td class="line" data-line="10"></td><td class="code"><div><span style="color:#657b83;">	</span><span style="color:#839496;">&#34;</span><span style="color:#2aa198;">log</span><span style="color:#839496;">&#34;↵</span></div></td></tr>',
    '<tr><td class="line" data-line="11"></td><td class="code"><div><span style="color:#657b83;">	</span><span style="color:#839496;">&#34;</span><span style="color:#2aa198;">net/http</span><span style="color:#839496;">&#34;↵</span></div></td></tr>',
    '<tr><td class="line" data-line="12"></td><td class="code"><div><span style="color:#657b83;">	</span><span style="color:#839496;">&#34;</span><span style="color:#2aa198;">time</span><span style="color:#839496;">&#34;↵</span></div></td></tr>',
    '<tr><td class="line" data-line="13"></td><td class="code"><div><span style="color:#657b83;">↵</span></div></td></tr>',
    '<tr><td class="line" data-line="14"></td><td class="code"><div><span style="color:#657b83;">	</span><span style="color:#839496;">&#34;</span><span style="color:#2aa198;">golang.org/x/oauth2</span><span style="color:#839496;">&#34;↵</span></div></td></tr>',
    '<tr><td class="line" data-line="15"></td><td class="code"><div><span style="color:#657b83;">)↵</span></div></td></tr>',
    '<tr><td class="line" data-line="16"></td><td class="code"><div><span style="color:#657b83;">↵</span></div></td></tr>',
    '<tr><td class="line" data-line="17"></td><td class="code"><div><span style="color:#268bd2;">func </span><span style="color:#b58900;">ExampleConfig</span><span style="color:#657b83;">() {↵</span></div></td></tr>',
    '<tr><td class="line" data-line="18"></td><td class="code"><div><span style="color:#657b83;">	</span><span style="color:#268bd2;">ctx </span><span style="color:#859900;">:=</span><span style="color:#657b83;"> context.</span><span style="color:#b58900;">Background</span><span style="color:#657b83;">()↵</span></div></td></tr>',
    '<tr><td class="line" data-line="19"></td><td class="code"><div><span style="color:#657b83;">	</span><span style="color:#268bd2;">conf </span><span style="color:#859900;">:= &amp;</span><span style="color:#657b83;">oauth2.</span><span style="color:#268bd2;">Config</span><span style="color:#657b83;">{↵</span></div></td></tr>',
    '<tr><td class="line" data-line="20"></td><td class="code"><div><span style="color:#657b83;">		</span><span style="color:#b58900;">ClientID</span><span style="color:#657b83;">:     </span><span style="color:#839496;">&#34;</span><span style="color:#2aa198;">YOUR_CLIENT_ID</span><span style="color:#839496;">&#34;</span><span style="color:#657b83;">,↵</span></div></td></tr>',
    '<tr><td class="line" data-line="21"></td><td class="code"><div><span style="color:#657b83;">		</span><span style="color:#b58900;">ClientSecret</span><span style="color:#657b83;">: </span><span style="color:#839496;">&#34;</span><span style="color:#2aa198;">YOUR_CLIENT_SECRET</span><span style="color:#839496;">&#34;</span><span style="color:#657b83;">,↵</span></div></td></tr>',
    '<tr><td class="line" data-line="22"></td><td class="code"><div><span style="color:#657b83;">		</span><span style="color:#b58900;">Scopes</span><span style="color:#657b83;">:       []</span><span style="color:#268bd2;">string</span><span style="color:#657b83;">{</span><span style="color:#839496;">&#34;</span><span style="color:#2aa198;">SCOPE1</span><span style="color:#839496;">&#34;</span><span style="color:#657b83;">, </span><span style="color:#839496;">&#34;</span><span style="color:#2aa198;">SCOPE2</span><span style="color:#839496;">&#34;</span><span style="color:#657b83;">},↵</span></div></td></tr>',
    '<tr><td class="line" data-line="23"></td><td class="code"><div><span style="color:#657b83;">		</span><span style="color:#b58900;">Endpoint</span><span style="color:#657b83;">: oauth2.</span><span style="color:#268bd2;">Endpoint</span><span style="color:#657b83;">{↵</span></div></td></tr>',
    '<tr><td class="line" data-line="24"></td><td class="code"><div><span style="color:#657b83;">			</span><span style="color:#b58900;">AuthURL</span><span style="color:#657b83;">:  </span><span style="color:#839496;">&#34;</span><span style="color:#2aa198;">https://provider.com/o/oauth2/auth</span><span style="color:#839496;">&#34;</span><span style="color:#657b83;">,↵</span></div></td></tr>',
]

const extractText = (html: string): string => {
    const span = document.createElement('span')
    span.innerHTML = html
    return span.textContent || span.innerText
}

export const FILE_LINES = HIGHLIGHTED_FILE_LINES.map(extractText)
export const FILE_LINES_SIMPLE = HIGHLIGHTED_FILE_LINES_SIMPLE.map(extractText)
export const FILE_LINES_LONG = HIGHLIGHTED_FILE_LINES_LONG.map(extractText)

export const HIGHLIGHTED_FILE_LINES_REQUEST = sinon.fake((parameters: FetchFileParameters) =>
    of(parameters.ranges.map(range => HIGHLIGHTED_FILE_LINES.slice(range.startLine, range.endLine)))
)
export const HIGHLIGHTED_FILE_LINES_SIMPLE_REQUEST = sinon.fake((parameters: FetchFileParameters) =>
    of(parameters.ranges.map(range => HIGHLIGHTED_FILE_LINES_SIMPLE.slice(range.startLine, range.endLine)))
)
export const HIGHLIGHTED_FILE_LINES_LONG_REQUEST = sinon.fake((parameters: FetchFileParameters) =>
    of(parameters.ranges.map(range => HIGHLIGHTED_FILE_LINES_LONG.slice(range.startLine, range.endLine)))
)

export const NOOP_SETTINGS_CASCADE = {
    subjects: null,
    final: null,
} as any as SettingsCascade

export const extensionsController: Controller = {
    executeCommand: () => Promise.resolve(),
    registerCommand: () => new Subscription(),
    extHostAPI: Promise.resolve(
        pretendRemote<FlatExtensionHostAPI>({
            getContributions: () => pretendProxySubscribable(NEVER),
            registerContributions: () => pretendProxySubscribable(EMPTY).subscribe(noop as any),
            haveInitialExtensionsLoaded: () => pretendProxySubscribable(of(true)),
        })
    ),
    unsubscribe: noop,
}

export const NOOP_PLATFORM_CONTEXT: Pick<
    PlatformContext,
    'sourcegraphURL' | 'requestGraphQL' | 'urlToFile' | 'settings' | 'telemetryRecorder'
> = {
    requestGraphQL: () => EMPTY,
    urlToFile: () => '',
    settings: of(NOOP_SETTINGS_CASCADE),
    sourcegraphURL: '',
    telemetryRecorder: noOpTelemetryRecorder,
}
