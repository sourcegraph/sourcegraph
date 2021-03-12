import { NEVER, of, Subscription } from 'rxjs'
import sinon from 'sinon'
import { FlatExtensionHostAPI } from '../api/contract'
import { pretendProxySubscribable, pretendRemote } from '../api/util'
import { Controller } from '../extensions/controller'
import {
    IFileMatch,
    IGitBlob,
    IGitCommit,
    ILineMatch,
    IRepository,
    ISearchResultMatch,
    ISearchResults,
    ISymbol,
    SearchResult,
} from '../graphql/schema'

export const RESULT = {
    __typename: 'FileMatch' as const,
    file: {
        path: '.travis.yml',
        url: '/github.com/golang/oauth2/-/blob/.travis.yml',
        commit: { oid: 'e64efc72b421e893cbf63f17ba2221e7d6d0b0f3' } as IGitCommit,
    } as IGitBlob,
    repository: { name: 'github.com/golang/oauth2', url: '/github.com/golang/oauth2' } as IRepository,
    limitHit: false,
    symbols: [] as ISymbol[],
    lineMatches: [
        {
            preview: '  - go test -v golang.org/x/oauth2/...',
            lineNumber: 12,
            offsetAndLengths: [[7, 4]],
            limitHit: false,
        } as ILineMatch,
    ],
} as IFileMatch

export const REPO_MATCH_RESULT = {
    __typename: 'Repository',
    name: 'github.com/golang/oauth2',
    url: '/github.com/golang/oauth2',
    matches: [] as ISearchResultMatch[],
    label: { __typename: 'Markdown', text: '[github.com/golang/oauth2](github.com/golang/oauth2)' },
} as IRepository

export const MULTIPLE_MATCH_RESULT = {
    __typename: 'FileMatch',
    file: {
        path: 'clientcredentials/clientcredentials_test.go',
        url: '/github.com/golang/oauth2/-/blob/clientcredentials/clientcredentials_test.go',
        commit: {
            oid: 'e64efc72b421e893cbf63f17ba2221e7d6d0b0f3',
        },
    },
    repository: {
        name: 'github.com/golang/oauth2',
        url: '/github.com/golang/oauth2',
    },
    limitHit: false,
    symbols: [],
    lineMatches: [
        {
            preview: '\t"net/http/httptest"',
            lineNumber: 11,
            offsetAndLengths: [[15, 4]],
            limitHit: false,
        },
        {
            preview: '\t"testing"',
            lineNumber: 13,
            offsetAndLengths: [[2, 4]],
            limitHit: false,
        },
        {
            preview: 'func TestTokenSourceGrantTypeOverride(t *testing.T) {',
            lineNumber: 36,
            offsetAndLengths: [
                [5, 4],
                [41, 4],
            ],
            limitHit: false,
        },
        {
            preview: '\tts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {',
            lineNumber: 39,
            offsetAndLengths: [[11, 4]],
            limitHit: false,
        },
        {
            preview: 'func TestTokenRequest(t *testing.T) {',
            lineNumber: 73,
            offsetAndLengths: [
                [5, 4],
                [25, 4],
            ],
            limitHit: false,
        },
        {
            preview: '\tts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {',
            lineNumber: 74,
            offsetAndLengths: [[11, 4]],
            limitHit: false,
        },
        {
            preview: 'func TestTokenRefreshRequest(t *testing.T) {',
            lineNumber: 115,
            offsetAndLengths: [
                [5, 4],
                [32, 4],
            ],
            limitHit: false,
        },
        {
            preview: '\tts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {',
            lineNumber: 117,
            offsetAndLengths: [[11, 4]],
            limitHit: false,
        },
        {
            preview: '\t\tio.WriteString(w, `{"access_token": "foo", "refresh_token": "bar"}`)',
            lineNumber: 134,
            offsetAndLengths: [[8, 4]],
            limitHit: false,
        },
    ],
}

export const SEARCH_RESULT = {
    __typename: 'SearchResults' as const,
    limitHit: false,
    matchCount: 1,
    approximateResultCount: '1',
    missing: [] as IRepository[],
    cloning: [] as IRepository[],
    timedout: [] as IRepository[],
    indexUnavailable: false,
    dynamicFilters: [
        { value: 'file:\\.yml$', label: 'file:\\.yml$', count: 1, limitHit: false, kind: 'file' },
        { value: 'case:yes', label: 'case:yes', count: 0, limitHit: false, kind: 'case' },
        {
            value: 'repo:^github\\.com/golang/oauth2$',
            label: 'github.com/golang/oauth2',
            count: 1,
            limitHit: false,
            kind: 'repo',
        },
    ],
    results: [RESULT],
    alert: null,
    elapsedMilliseconds: 78,
} as ISearchResults

export const MULTIPLE_SEARCH_RESULT = {
    ...SEARCH_RESULT,
    limitHit: false,
    matchCount: 136,
    approximateResultCount: '136',
    results: [
        RESULT,
        MULTIPLE_MATCH_RESULT,
        {
            __typename: 'FileMatch',
            file: {
                path: 'example_test.go',
                url: '/github.com/golang/oauth2/-/blob/example_test.go',
                commit: {
                    oid: 'e64efc72b421e893cbf63f17ba2221e7d6d0b0f3',
                } as IGitCommit,
            } as IGitBlob,
            repository: {
                name: 'github.com/golang/oauth2',
                url: '/github.com/golang/oauth2',
            } as IRepository,
            limitHit: false,
            symbols: [] as ISymbol[],
            lineMatches: [
                {
                    preview: 'package oauth2_test',
                    lineNumber: 4,
                    offsetAndLengths: [[15, 4]],
                } as ILineMatch,
            ],
        } as IFileMatch,
    ] as SearchResult[],
} as ISearchResults

// Result from query: repo:^github\.com/golang/oauth2$ test f:travis
export const SEARCH_REQUEST = sinon.spy(() => SEARCH_RESULT)
export const MULTIPLE_SEARCH_REQUEST = sinon.spy(() => MULTIPLE_SEARCH_RESULT)
export const OBSERVABLE_SEARCH_REQUEST = sinon.spy(() => of(SEARCH_RESULT))

export const HIGHLIGHTED_FILE_LINES = [
    [
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
    ],
]

export const HIGHLIGHTED_FILE_LINES_SIMPLE = [
    [
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
    ],
]

export const HIGHLIGHTED_FILE_LINES_LONG = [
    [
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
    ],
]

export const HIGHLIGHTED_FILE_LINES_REQUEST = sinon.fake.returns(of(HIGHLIGHTED_FILE_LINES))
export const HIGHLIGHTED_FILE_LINES_SIMPLE_REQUEST = sinon.fake.returns(of(HIGHLIGHTED_FILE_LINES_SIMPLE))

export const NOOP_SETTINGS_CASCADE = {
    subjects: null,
    final: null,
}

export const extensionsController: Pick<Controller, 'executeCommand' | 'registerCommand' | 'extHostAPI'> = {
    executeCommand: () => Promise.resolve(),
    registerCommand: () => new Subscription(),
    extHostAPI: Promise.resolve(
        pretendRemote<FlatExtensionHostAPI>({
            getContributions: () => pretendProxySubscribable(NEVER),
        })
    ),
}
