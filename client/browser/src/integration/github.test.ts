import assert from 'assert'

import type { ExtensionContext } from '@sourcegraph/shared/src/codeintel/legacy-extensions/api'
import { Settings } from '@sourcegraph/shared/src/settings/settings'
import { createDriverForTest, Driver } from '@sourcegraph/shared/src/testing/driver'
import { setupExtensionMocking, simpleHoverProvider } from '@sourcegraph/shared/src/testing/integration/mockExtension'
import { afterEachSaveScreenshotIfFailed } from '@sourcegraph/shared/src/testing/screenshotReporter'
import { readEnvironmentBoolean, readEnvironmentString, retry } from '@sourcegraph/shared/src/testing/utils'

import { BrowserIntegrationTestContext, createBrowserIntegrationTestContext } from './context'
import { closeInstallPageTab } from './shared'

describe('GitHub', () => {
    let driver: Driver
    let testContext: BrowserIntegrationTestContext

    const mockUrls = (urls: string[]) => {
        for (const url of urls) {
            testContext.server.any(url).intercept((request, response) => {
                response.sendStatus(200)
            })
        }
    }

    beforeEach(async function () {
        driver = await createDriverForTest({ loadExtension: true })
        await closeInstallPageTab(driver.browser)
        if (driver.sourcegraphBaseUrl !== 'https://sourcegraph.com') {
            await driver.setExtensionSourcegraphUrl()
        }

        testContext = await createBrowserIntegrationTestContext({
            driver,
            currentTest: this.currentTest!,
            directory: __dirname,
        })

        mockUrls([
            'https://api.github.com/_private/browser/*',
            'https://collector.github.com/*path',
            'https://github.com/favicon.ico',
            'https://github.githubassets.com/favicons/*path',
        ])

        testContext.server.any('https://api.github.com/repos/*').intercept((request, response) => {
            response
                .status(200)
                .setHeader('Access-Control-Allow-Origin', 'https://github.com')
                .send(JSON.stringify({ private: false }))
        })

        testContext.overrideGraphQL({
            ViewerConfiguration: () => ({
                viewerConfiguration: {
                    subjects: [],
                    merged: { contents: '', messages: [] },
                },
            }),
            ResolveRev: () => ({
                repository: {
                    mirrorInfo: {
                        cloned: true,
                    },
                    commit: {
                        oid: '1'.repeat(40),
                    },
                },
            }),
            ResolveRepo: ({ rawRepoName }) => ({
                repository: {
                    name: rawRepoName,
                },
            }),
            ResolveRawRepoName: ({ repoName }) => ({
                repository: { uri: `${repoName}`, mirrorInfo: { cloned: true } },
            }),
            SiteProductVersion: () => ({
                site: {
                    productVersion: '129819_2022-02-08_baac612f829f',
                    buildVersion: '129819_2022-02-08_baac612f829f',
                    hasCodeIntelligence: true,
                },
            }),
            BlobContent: () => ({
                repository: {
                    commit: {
                        file: {
                            content:
                                'package jsonrpc2\n\n// CallOption is an option that can be provided to (*Conn).Call to\n// configure custom behavior. See Meta.\ntype CallOption interface {\n\tapply(r *Request) error\n}\n\ntype callOptionFunc func(r *Request) error\n\nfunc (c callOptionFunc) apply(r *Request) error { return c(r) }\n\n// Meta returns a call option which attaches the given meta object to\n// the JSON-RPC 2.0 request (this is a Sourcegraph extension to JSON\n// RPC 2.0 for carrying metadata).\nfunc Meta(meta interface{}) CallOption {\n\treturn callOptionFunc(func(r *Request) error {\n\t\treturn r.SetMeta(meta)\n\t})\n}\n\n// PickID returns a call option which sets the ID on a request. Care must be\n// taken to ensure there are no conflicts with any previously picked ID, nor\n// with the default sequence ID.\nfunc PickID(id ID) CallOption {\n\treturn callOptionFunc(func(r *Request) error {\n\t\tr.ID = id\n\t\treturn nil\n\t})\n}\n',
                        },
                    },
                },
            }),
            UserSettingsURL: () => ({
                currentUser: {
                    settingsURL: '/users/john.doe/settings',
                },
            }),
            CurrentUser: () => ({
                currentUser: {
                    settingsURL: '/users/john.doe/settings',
                    siteAdmin: false,
                },
            }),
        })

        // Ensure that the same assets are requested in all environments.
        await driver.page.emulateMediaFeatures([{ name: 'prefers-color-scheme', value: 'light' }])
    })
    afterEachSaveScreenshotIfFailed(() => driver.page)
    afterEach(async () => {
        await testContext?.dispose()
        await driver?.close()
    })

    it('adds "View on Sourcegraph" buttons to files', async () => {
        const repoName = 'github.com/sourcegraph/jsonrpc2'

        await driver.page.goto(
            'https://github.com/sourcegraph/jsonrpc2/blob/4fb7cd90793ee6ab445f466b900e6bffb9b63d78/call_opt.go'
        )

        await driver.page.waitForSelector('[data-testid="code-view-toolbar"] [data-testid="open-on-sourcegraph"]', {
            timeout: 10000,
        })
        assert.strictEqual(
            (await driver.page.$$('[data-testid="code-view-toolbar"] [data-testid="open-on-sourcegraph"]')).length,
            1
        )

        await retry(async () => {
            assert.strictEqual(
                await driver.page.evaluate(
                    () =>
                        document.querySelector<HTMLAnchorElement>(
                            '[data-testid="code-view-toolbar"] [data-testid="open-on-sourcegraph"]'
                        )?.href
                ),
                new URL(
                    `${driver.sourcegraphBaseUrl}/${repoName}@4fb7cd90793ee6ab445f466b900e6bffb9b63d78/-/blob/call_opt.go`
                ).href
            )
        })
    })

    // TODO(#42743): This test is flaky on CI and was disabled to unblock the pipeline.
    // We need to investigate on what is causing the flakieness and remove it to
    // bring it back.
    //
    // it('shows hover tooltips when hovering a token and respects "Enable single click to go to definition" setting', async () => {
    //     mockUrls(['https://github.com/*path/find-definition'])

    //     const { mockExtension } = setupExtensionMocking()

    //     const userSettings: Settings = {
    //         extensions: extensionSettings,
    //     }
    //     testContext.overrideGraphQL({
    //         ViewerConfiguration: () => ({
    //             viewerConfiguration: {
    //                 subjects: [
    //                     {
    //                         __typename: 'User',
    //                         displayName: 'Test User',
    //                         id: 'TestUserSettingsID',
    //                         latestSettings: {
    //                             id: 123,
    //                             contents: JSON.stringify(userSettings),
    //                         },
    //                         username: 'test',
    //                         viewerCanAdminister: true,
    //                         settingsURL: '/users/test/settings',
    //                     },
    //                 ],
    //                 merged: { contents: JSON.stringify(userSettings), messages: [] },
    //             },
    //         }),
    //         UserSettingsURL: () => ({
    //             currentUser: {
    //                 settingsURL: 'users/john-doe/settings',
    //             },
    //         }),
    //         Extensions,
    //     })

    //     // Serve a mock extension with a simple hover provider
    //     mockExtension({
    //         id: 'simple/hover',
    //         bundle: function extensionBundle(): void {
    //             // eslint-disable-next-line @typescript-eslint/no-require-imports, @typescript-eslint/no-var-requires
    //             const sourcegraph = require('sourcegraph') as typeof import('sourcegraph')

    //             function activate(context: sourcegraph.ExtensionContext): void {
    //                 context.subscriptions.add(
    //                     sourcegraph.languages.registerHoverProvider(['*'], {
    //                         provideHover: (document, position) => {
    //                             const range = document.getWordRangeAtPosition(position)
    //                             const token = document.getText(range)
    //                             if (!token) {
    //                                 return null
    //                             }
    //                             return {
    //                                 contents: {
    //                                     value: `User is hovering over ${token}`,
    //                                     kind: sourcegraph.MarkupKind.Markdown,
    //                                 },
    //                                 range,
    //                             }
    //                         },
    //                     })
    //                 )

    //                 context.subscriptions.add(
    //                     sourcegraph.languages.registerDefinitionProvider(['*'], {
    //                         provideDefinition: () =>
    //                             new sourcegraph.Location(
    //                                 new URL(
    //                                     'https://github.com/sourcegraph/jsonrpc2/blob/4fb7cd90793ee6ab445f466b900e6bffb9b63d78/call_opt.go'
    //                                 ),
    //                                 new sourcegraph.Range(
    //                                     new sourcegraph.Position(4, 5),
    //                                     new sourcegraph.Position(5, 14)
    //                                 )
    //                             ),
    //                     })
    //                 )
    //             }

    //             exports.activate = activate
    //         },
    //     })

    //     let hasRedirectedToDefinition = false

    //     // For some reason in test requested definition URL is different from the actual one:
    //     // 'https://github.com/sourcegraph/jsonrpc2/blob/4fb7cd90793ee6ab445f466b900e6bffb9b63d78/call_opt.go/blob/HEAD/#L5:6' instead of
    //     // 'https://github.com/sourcegraph/jsonrpc2/blob/4fb7cd90793ee6ab445f466b900e6bffb9b63d78/call_opt.go#L5:6'.
    //     // The former URL is returns 404 page so for test sake we intercept such requests and track the fact of redirect.
    //     testContext.server
    //         .get(
    //             'https://github.com/sourcegraph/jsonrpc2/blob/4fb7cd90793ee6ab445f466b900e6bffb9b63d78/call_opt.go/blob/HEAD/#L5:6'
    //         )
    //         .intercept((request, response) => {
    //             response.sendStatus(200)
    //             hasRedirectedToDefinition = true
    //         })

    //     const openPageAndGetToken = async () => {
    //         await driver.page.goto(
    //             'https://github.com/sourcegraph/jsonrpc2/blob/4fb7cd90793ee6ab445f466b900e6bffb9b63d78/call_opt.go'
    //         )
    //         await driver.page.waitForSelector('[data-testid="code-view-toolbar"] [data-testid="open-on-sourcegraph"]')

    //         // Pause to give codeintellify time to register listeners for
    //         // tokenization (only necessary in CI, not sure why).
    //         await driver.page.waitForTimeout(1000)

    //         const lineSelector = '.js-file-line-container tr'

    //         // Trigger tokenization of the line.
    //         const lineNumber = 16
    //         const line = await driver.page.waitForSelector(`${lineSelector}:nth-child(${lineNumber})`, {
    //             timeout: 10000,
    //         })

    //         if (!line) {
    //             throw new Error(`Found no line with number ${lineNumber}`)
    //         }

    //         const [token] = await line.$x('.//span[text()="CallOption"]')
    //         return token
    //     }

    //     let token = await openPageAndGetToken()

    //     // 1. Check that hovering a token shows code intel popup.
    //     await token.hover()
    //     await driver.findElementWithText('User is hovering over CallOption', {
    //         selector: ' [data-testid="hover-overlay-content"] > p',
    //         fuzziness: 'contains',
    //         wait: {
    //             timeout: 6000,
    //         },
    //     })

    //     await percySnapshot(driver.page, 'Browser extension: GitHub - blob view with code intel popup')

    //     // 2. Check that token click does not do anything by default
    //     await token.click()
    //     await driver.page.waitForTimeout(1000)
    //     assert(!hasRedirectedToDefinition, 'Expected to not be redirected to definition')

    //     // 3. Enable click-to-def setting and check that it redirects to the proper page
    //     await driver.setClickGoToDefOptionFlag(true)
    //     token = await openPageAndGetToken()
    //     await token.hover()
    //     await driver.findElementWithText('User is hovering over CallOption', {
    //         selector: ' [data-testid="hover-overlay-content"] > p',
    //         fuzziness: 'contains',
    //         wait: {
    //             timeout: 6000,
    //         },
    //     })
    //     await token.click()
    //     await driver.page.waitForTimeout(1000)

    //     assert(hasRedirectedToDefinition, 'Expected to be redirected to definition')
    // })

    describe('Pull request pages', () => {
        // TODO(sqs): skipped because these have not been reimplemented after the extension API deprecation
        describe.skip('Files Changed view', () => {
            // For each pull request test, set up a mock extension that verifies that the correct
            // file and revision info reach extensions.
            beforeEach(() => {
                mockUrls(['https://github.com/*path/find-definition'])

                const { mockExtension } = setupExtensionMocking()

                const userSettings: Settings = {}
                testContext.overrideGraphQL({
                    ViewerConfiguration: () => ({
                        viewerConfiguration: {
                            subjects: [
                                {
                                    __typename: 'User',
                                    displayName: 'Test User',
                                    id: 'TestUserSettingsID',
                                    latestSettings: {
                                        id: 123,
                                        contents: JSON.stringify(userSettings),
                                    },
                                    username: 'test',
                                    viewerCanAdminister: true,
                                    settingsURL: '/users/test/settings',
                                },
                            ],
                            merged: { contents: JSON.stringify(userSettings), messages: [] },
                        },
                    }),
                    ResolveRev: ({ revision }) => ({
                        repository: {
                            mirrorInfo: { cloned: true },
                            commit: {
                                oid: revision,
                            },
                        },
                    }),
                    BlobContent: ({ commitID }) => ({
                        repository: {
                            commit: {
                                file: {
                                    content:
                                        commitID === tokens.head.commitID
                                            ? '// Copyright 2012 The Gorilla Authors. All rights reserved.\n// Use of this source code is governed by a BSD-style\n// license that can be found in the LICENSE file.\n\npackage mux\n\nimport (\n\t"bytes"\n\t"fmt"\n\t"net/http"\n\t"net/url"\n\t"regexp"\n\t"strings"\n)\n\n// newRouteRegexp parses a route template and returns a routeRegexp,\n// used to match a host, a path or a query string.\n//\n// It will extract named variables, assemble a regexp to be matched, create\n// a "reverse" template to build URLs and compile regexps to validate variable\n// values used in URL building.\n//\n// Previously we accepted only Python-like identifiers for variable\n// names ([a-zA-Z_][a-zA-Z0-9_]*), but currently the only restriction is that\n// name and pattern can\'t be empty, and names can\'t contain a colon.\nfunc newRouteRegexp(tpl string, matchHost, matchPrefix, matchQuery, strictSlash bool) (*routeRegexp, error) {\n\t// Check if it is well-formed.\n\tidxs, errBraces := braceIndices(tpl)\n\tif errBraces != nil {\n\t\treturn nil, errBraces\n\t}\n\t// Backup the original.\n\ttemplate := tpl\n\t// Now let\'s parse it.\n\tdefaultPattern := "[^/]+"\n\tif matchQuery {\n\t\tdefaultPattern = "[^?&]*"\n\t} else if matchHost {\n\t\tdefaultPattern = "[^.]+"\n\t\tmatchPrefix = false\n\t}\n\t// Only match strict slash if not matching\n\tif matchPrefix || matchHost || matchQuery {\n\t\tstrictSlash = false\n\t}\n\t// Set a flag for strictSlash.\n\tendSlash := false\n\tif strictSlash && strings.HasSuffix(tpl, "/") {\n\t\ttpl = tpl[:len(tpl)-1]\n\t\tendSlash = true\n\t}\n\tvarsN := make([]string, len(idxs)/2)\n\tvarsR := make([]*regexp.Regexp, len(idxs)/2)\n\tpattern := bytes.NewBufferString("")\n\tpattern.WriteByte(\'^\')\n\treverse := bytes.NewBufferString("")\n\tvar end int\n\tvar err error\n\tfor i := 0; i < len(idxs); i += 2 {\n\t\t// Set all values we are interested in.\n\t\traw := tpl[end:idxs[i]]\n\t\tend = idxs[i+1]\n\t\tparts := strings.SplitN(tpl[idxs[i]+1:end-1], ":", 2)\n\t\tname := parts[0]\n\t\tpatt := defaultPattern\n\t\tif len(parts) == 2 {\n\t\t\tpatt = parts[1]\n\t\t}\n\t\t// Name or pattern can\'t be empty.\n\t\tif name == "" || patt == "" {\n\t\t\treturn nil, fmt.Errorf("mux: missing name or pattern in %q",\n\t\t\t\ttpl[idxs[i]:end])\n\t\t}\n\t\t// Build the regexp pattern.\n\t\tfmt.Fprintf(pattern, "%s(?P<%s>%s)", regexp.QuoteMeta(raw), name, patt)\n\t\t// Build the reverse template.\n\t\tfmt.Fprintf(reverse, "%s%%s", raw)\n\n\t\t// Append variable name and compiled pattern.\n\t\tvarsN[i/2] = name\n\t\tvarsR[i/2], err = regexp.Compile(fmt.Sprintf("^%s$", patt))\n\t\tif err != nil {\n\t\t\treturn nil, err\n\t\t}\n\t}\n\t// Add the remaining.\n\traw := tpl[end:]\n\tpattern.WriteString(regexp.QuoteMeta(raw))\n\tif strictSlash {\n\t\tpattern.WriteString("[/]?")\n\t}\n\tif matchQuery {\n\t\t// Add the default pattern if the query value is empty\n\t\tif queryVal := strings.SplitN(template, "=", 2)[1]; queryVal == "" {\n\t\t\tpattern.WriteString(defaultPattern)\n\t\t}\n\t}\n\tif !matchPrefix {\n\t\tpattern.WriteByte(\'$\')\n\t}\n\treverse.WriteString(raw)\n\tif endSlash {\n\t\treverse.WriteByte(\'/\')\n\t}\n\t// Compile full regexp.\n\treg, errCompile := regexp.Compile(pattern.String())\n\tif errCompile != nil {\n\t\treturn nil, errCompile\n\t}\n\t// Done!\n\treturn &routeRegexp{\n\t\ttemplate:    template,\n\t\tmatchHost:   matchHost,\n\t\tmatchQuery:  matchQuery,\n\t\tstrictSlash: strictSlash,\n\t\tregexp:      reg,\n\t\treverse:     reverse.String(),\n\t\tvarsN:       varsN,\n\t\tvarsR:       varsR,\n\t}, nil\n}\n\n// routeRegexp stores a regexp to match a host or path and information to\n// collect and validate route variables.\ntype routeRegexp struct {\n\t// The unmodified template.\n\ttemplate string\n\t// True for host match, false for path or query string match.\n\tmatchHost bool\n\t// True for query string match, false for path and host match.\n\tmatchQuery bool\n\t// The strictSlash value defined on the route, but disabled if PathPrefix was used.\n\tstrictSlash bool\n\t// Expanded regexp.\n\tregexp *regexp.Regexp\n\t// Reverse template.\n\treverse string\n\t// Variable names.\n\tvarsN []string\n\t// Variable regexps (validators).\n\tvarsR []*regexp.Regexp\n}\n\n// Match matches the regexp against the URL host or path.\nfunc (r *routeRegexp) Match(req *http.Request, match *RouteMatch) bool {\n\tif !r.matchHost {\n\t\tif r.matchQuery {\n\t\t\treturn r.matchQueryString(req)\n\t\t} else {\n\t\t\treturn r.regexp.MatchString(req.URL.Path)\n\t\t}\n\t}\n\treturn r.regexp.MatchString(getHost(req))\n}\n\n// url builds a URL part using the given values.\nfunc (r *routeRegexp) url(values map[string]string) (string, error) {\n\turlValues := make([]interface{}, len(r.varsN))\n\tfor k, v := range r.varsN {\n\t\tvalue, ok := values[v]\n\t\tif !ok {\n\t\t\treturn "", fmt.Errorf("mux: missing route variable %q", v)\n\t\t}\n\t\turlValues[k] = value\n\t}\n\trv := fmt.Sprintf(r.reverse, urlValues...)\n\tif !r.regexp.MatchString(rv) {\n\t\t// The URL is checked against the full regexp, instead of checking\n\t\t// individual variables. This is faster but to provide a good error\n\t\t// message, we check individual regexps if the URL doesn\'t match.\n\t\tfor k, v := range r.varsN {\n\t\t\tif !r.varsR[k].MatchString(values[v]) {\n\t\t\t\treturn "", fmt.Errorf(\n\t\t\t\t\t"mux: variable %q doesn\'t match, expected %q", values[v],\n\t\t\t\t\tr.varsR[k].String())\n\t\t\t}\n\t\t}\n\t}\n\treturn rv, nil\n}\n\n// getUrlQuery returns a single query parameter from a request URL.\n// For a URL with foo=bar&baz=ding, we return only the relevant key\n// value pair for the routeRegexp.\nfunc (r *routeRegexp) getUrlQuery(req *http.Request) string {\n\tif !r.matchQuery {\n\t\treturn ""\n\t}\n\ttemplateKey := strings.SplitN(r.template, "=", 2)[0]\n\tfor key, vals := range req.URL.Query() {\n\t\tif key == templateKey && len(vals) > 0 {\n\t\t\treturn key + "=" + vals[0]\n\t\t}\n\t}\n\treturn ""\n}\n\nfunc (r *routeRegexp) matchQueryString(req *http.Request) bool {\n\treturn r.regexp.MatchString(r.getUrlQuery(req))\n}\n\n// braceIndices returns the first level curly brace indices from a string.\n// It returns an error in case of unbalanced braces.\nfunc braceIndices(s string) ([]int, error) {\n\tvar level, idx int\n\tidxs := make([]int, 0)\n\tfor i := 0; i < len(s); i++ {\n\t\tswitch s[i] {\n\t\tcase \'{\':\n\t\t\tif level++; level == 1 {\n\t\t\t\tidx = i\n\t\t\t}\n\t\tcase \'}\':\n\t\t\tif level--; level == 0 {\n\t\t\t\tidxs = append(idxs, idx, i+1)\n\t\t\t} else if level < 0 {\n\t\t\t\treturn nil, fmt.Errorf("mux: unbalanced braces in %q", s)\n\t\t\t}\n\t\t}\n\t}\n\tif level != 0 {\n\t\treturn nil, fmt.Errorf("mux: unbalanced braces in %q", s)\n\t}\n\treturn idxs, nil\n}\n\n// ----------------------------------------------------------------------------\n// routeRegexpGroup\n// ----------------------------------------------------------------------------\n\n// routeRegexpGroup groups the route matchers that carry variables.\ntype routeRegexpGroup struct {\n\thost    *routeRegexp\n\tpath    *routeRegexp\n\tqueries []*routeRegexp\n}\n\n// setMatch extracts the variables from the URL once a route matches.\nfunc (v *routeRegexpGroup) setMatch(req *http.Request, m *RouteMatch, r *Route) {\n\t// Store host variables.\n\tif v.host != nil {\n\t\thostVars := v.host.regexp.FindStringSubmatch(getHost(req))\n\t\tif hostVars != nil {\n\t\t\tsubexpNames := v.host.regexp.SubexpNames()\n\t\t\tvarName := 0\n\t\t\tfor i, name := range subexpNames[1:] {\n\t\t\t\tif name != "" && v.host.varsN[varName] == name {\n\t\t\t\t\tm.Vars[name] = hostVars[i+1]\n\t\t\t\t\tvarName++\n\t\t\t\t}\n\t\t\t}\n\t\t}\n\t}\n\t// Store path variables.\n\tif v.path != nil {\n\t\tpathVars := v.path.regexp.FindStringSubmatch(req.URL.Path)\n\t\tif pathVars != nil {\n\t\t\tsubexpNames := v.path.regexp.SubexpNames()\n\t\t\tvarName := 0\n\t\t\tfor i, name := range subexpNames[1:] {\n\t\t\t\tif name != "" && v.path.varsN[varName] == name {\n\t\t\t\t\tm.Vars[name] = pathVars[i+1]\n\t\t\t\t\tvarName++\n\t\t\t\t}\n\t\t\t}\n\t\t\t// Check if we should redirect.\n\t\t\tif v.path.strictSlash {\n\t\t\t\tp1 := strings.HasSuffix(req.URL.Path, "/")\n\t\t\t\tp2 := strings.HasSuffix(v.path.template, "/")\n\t\t\t\tif p1 != p2 {\n\t\t\t\t\tu, _ := url.Parse(req.URL.String())\n\t\t\t\t\tif p1 {\n\t\t\t\t\t\tu.Path = u.Path[:len(u.Path)-1]\n\t\t\t\t\t} else {\n\t\t\t\t\t\tu.Path += "/"\n\t\t\t\t\t}\n\t\t\t\t\tm.Handler = http.RedirectHandler(u.String(), 301)\n\t\t\t\t}\n\t\t\t}\n\t\t}\n\t}\n\t// Store query string variables.\n\tfor _, q := range v.queries {\n\t\tqueryVars := q.regexp.FindStringSubmatch(q.getUrlQuery(req))\n\t\tif queryVars != nil {\n\t\t\tsubexpNames := q.regexp.SubexpNames()\n\t\t\tvarName := 0\n\t\t\tfor i, name := range subexpNames[1:] {\n\t\t\t\tif name != "" && q.varsN[varName] == name {\n\t\t\t\t\tm.Vars[name] = queryVars[i+1]\n\t\t\t\t\tvarName++\n\t\t\t\t}\n\t\t\t}\n\t\t}\n\t}\n}\n\n// getHost tries its best to return the request host.\nfunc getHost(r *http.Request) string {\n\tif r.URL.IsAbs() {\n\t\treturn r.URL.Host\n\t}\n\thost := r.Host\n\t// Slice off any port information.\n\tif i := strings.Index(host, ":"); i != -1 {\n\t\thost = host[:i]\n\t}\n\treturn host\n\n}\n'
                                            : '// Copyright 2012 The Gorilla Authors. All rights reserved.\n// Use of this source code is governed by a BSD-style\n// license that can be found in the LICENSE file.\n\npackage mux\n\nimport (\n\t"bytes"\n\t"fmt"\n\t"net/http"\n\t"net/url"\n\t"regexp"\n\t"strings"\n)\n\n// newRouteRegexp parses a route template and returns a routeRegexp,\n// used to match a host, a path or a query string.\n//\n// It will extract named variables, assemble a regexp to be matched, create\n// a "reverse" template to build URLs and compile regexps to validate variable\n// values used in URL building.\n//\n// Previously we accepted only Python-like identifiers for variable\n// names ([a-zA-Z_][a-zA-Z0-9_]*), but currently the only restriction is that\n// name and pattern can\'t be empty, and names can\'t contain a colon.\nfunc newRouteRegexp(tpl string, matchHost, matchPrefix, matchQuery, strictSlash bool) (*routeRegexp, error) {\n\t// Check if it is well-formed.\n\tidxs, errBraces := braceIndices(tpl)\n\tif errBraces != nil {\n\t\treturn nil, errBraces\n\t}\n\t// Backup the original.\n\ttemplate := tpl\n\t// Now let\'s parse it.\n\tdefaultPattern := "[^/]+"\n\tif matchQuery {\n\t\tdefaultPattern = "[^?&]*"\n\t} else if matchHost {\n\t\tdefaultPattern = "[^.]+"\n\t\tmatchPrefix = false\n\t}\n\t// Only match strict slash if not matching\n\tif matchPrefix || matchHost || matchQuery {\n\t\tstrictSlash = false\n\t}\n\t// Set a flag for strictSlash.\n\tendSlash := false\n\tif strictSlash && strings.HasSuffix(tpl, "/") {\n\t\ttpl = tpl[:len(tpl)-1]\n\t\tendSlash = true\n\t}\n\tvarsN := make([]string, len(idxs)/2)\n\tvarsR := make([]*regexp.Regexp, len(idxs)/2)\n\tpattern := bytes.NewBufferString("")\n\tpattern.WriteByte(\'^\')\n\treverse := bytes.NewBufferString("")\n\tvar end int\n\tvar err error\n\tfor i := 0; i < len(idxs); i += 2 {\n\t\t// Set all values we are interested in.\n\t\traw := tpl[end:idxs[i]]\n\t\tend = idxs[i+1]\n\t\tparts := strings.SplitN(tpl[idxs[i]+1:end-1], ":", 2)\n\t\tname := parts[0]\n\t\tpatt := defaultPattern\n\t\tif len(parts) == 2 {\n\t\t\tpatt = parts[1]\n\t\t}\n\t\t// Name or pattern can\'t be empty.\n\t\tif name == "" || patt == "" {\n\t\t\treturn nil, fmt.Errorf("mux: missing name or pattern in %q",\n\t\t\t\ttpl[idxs[i]:end])\n\t\t}\n\t\t// Build the regexp pattern.\n\t\tfmt.Fprintf(pattern, "%s(%s)", regexp.QuoteMeta(raw), patt)\n\t\t// Build the reverse template.\n\t\tfmt.Fprintf(reverse, "%s%%s", raw)\n\n\t\t// Append variable name and compiled pattern.\n\t\tvarsN[i/2] = name\n\t\tvarsR[i/2], err = regexp.Compile(fmt.Sprintf("^%s$", patt))\n\t\tif err != nil {\n\t\t\treturn nil, err\n\t\t}\n\t}\n\t// Add the remaining.\n\traw := tpl[end:]\n\tpattern.WriteString(regexp.QuoteMeta(raw))\n\tif strictSlash {\n\t\tpattern.WriteString("[/]?")\n\t}\n\tif matchQuery {\n\t\t// Add the default pattern if the query value is empty\n\t\tif queryVal := strings.SplitN(template, "=", 2)[1]; queryVal == "" {\n\t\t\tpattern.WriteString(defaultPattern)\n\t\t}\n\t}\n\tif !matchPrefix {\n\t\tpattern.WriteByte(\'$\')\n\t}\n\treverse.WriteString(raw)\n\tif endSlash {\n\t\treverse.WriteByte(\'/\')\n\t}\n\t// Compile full regexp.\n\treg, errCompile := regexp.Compile(pattern.String())\n\tif errCompile != nil {\n\t\treturn nil, errCompile\n\t}\n\t// Done!\n\treturn &routeRegexp{\n\t\ttemplate:    template,\n\t\tmatchHost:   matchHost,\n\t\tmatchQuery:  matchQuery,\n\t\tstrictSlash: strictSlash,\n\t\tregexp:      reg,\n\t\treverse:     reverse.String(),\n\t\tvarsN:       varsN,\n\t\tvarsR:       varsR,\n\t}, nil\n}\n\n// routeRegexp stores a regexp to match a host or path and information to\n// collect and validate route variables.\ntype routeRegexp struct {\n\t// The unmodified template.\n\ttemplate string\n\t// True for host match, false for path or query string match.\n\tmatchHost bool\n\t// True for query string match, false for path and host match.\n\tmatchQuery bool\n\t// The strictSlash value defined on the route, but disabled if PathPrefix was used.\n\tstrictSlash bool\n\t// Expanded regexp.\n\tregexp *regexp.Regexp\n\t// Reverse template.\n\treverse string\n\t// Variable names.\n\tvarsN []string\n\t// Variable regexps (validators).\n\tvarsR []*regexp.Regexp\n}\n\n// Match matches the regexp against the URL host or path.\nfunc (r *routeRegexp) Match(req *http.Request, match *RouteMatch) bool {\n\tif !r.matchHost {\n\t\tif r.matchQuery {\n\t\t\treturn r.matchQueryString(req)\n\t\t} else {\n\t\t\treturn r.regexp.MatchString(req.URL.Path)\n\t\t}\n\t}\n\treturn r.regexp.MatchString(getHost(req))\n}\n\n// url builds a URL part using the given values.\nfunc (r *routeRegexp) url(values map[string]string) (string, error) {\n\turlValues := make([]interface{}, len(r.varsN))\n\tfor k, v := range r.varsN {\n\t\tvalue, ok := values[v]\n\t\tif !ok {\n\t\t\treturn "", fmt.Errorf("mux: missing route variable %q", v)\n\t\t}\n\t\turlValues[k] = value\n\t}\n\trv := fmt.Sprintf(r.reverse, urlValues...)\n\tif !r.regexp.MatchString(rv) {\n\t\t// The URL is checked against the full regexp, instead of checking\n\t\t// individual variables. This is faster but to provide a good error\n\t\t// message, we check individual regexps if the URL doesn\'t match.\n\t\tfor k, v := range r.varsN {\n\t\t\tif !r.varsR[k].MatchString(values[v]) {\n\t\t\t\treturn "", fmt.Errorf(\n\t\t\t\t\t"mux: variable %q doesn\'t match, expected %q", values[v],\n\t\t\t\t\tr.varsR[k].String())\n\t\t\t}\n\t\t}\n\t}\n\treturn rv, nil\n}\n\n// getUrlQuery returns a single query parameter from a request URL.\n// For a URL with foo=bar&baz=ding, we return only the relevant key\n// value pair for the routeRegexp.\nfunc (r *routeRegexp) getUrlQuery(req *http.Request) string {\n\tif !r.matchQuery {\n\t\treturn ""\n\t}\n\ttemplateKey := strings.SplitN(r.template, "=", 2)[0]\n\tfor key, vals := range req.URL.Query() {\n\t\tif key == templateKey && len(vals) > 0 {\n\t\t\treturn key + "=" + vals[0]\n\t\t}\n\t}\n\treturn ""\n}\n\nfunc (r *routeRegexp) matchQueryString(req *http.Request) bool {\n\treturn r.regexp.MatchString(r.getUrlQuery(req))\n}\n\n// braceIndices returns the first level curly brace indices from a string.\n// It returns an error in case of unbalanced braces.\nfunc braceIndices(s string) ([]int, error) {\n\tvar level, idx int\n\tidxs := make([]int, 0)\n\tfor i := 0; i < len(s); i++ {\n\t\tswitch s[i] {\n\t\tcase \'{\':\n\t\t\tif level++; level == 1 {\n\t\t\t\tidx = i\n\t\t\t}\n\t\tcase \'}\':\n\t\t\tif level--; level == 0 {\n\t\t\t\tidxs = append(idxs, idx, i+1)\n\t\t\t} else if level < 0 {\n\t\t\t\treturn nil, fmt.Errorf("mux: unbalanced braces in %q", s)\n\t\t\t}\n\t\t}\n\t}\n\tif level != 0 {\n\t\treturn nil, fmt.Errorf("mux: unbalanced braces in %q", s)\n\t}\n\treturn idxs, nil\n}\n\n// ----------------------------------------------------------------------------\n// routeRegexpGroup\n// ----------------------------------------------------------------------------\n\n// routeRegexpGroup groups the route matchers that carry variables.\ntype routeRegexpGroup struct {\n\thost    *routeRegexp\n\tpath    *routeRegexp\n\tqueries []*routeRegexp\n}\n\n// setMatch extracts the variables from the URL once a route matches.\nfunc (v *routeRegexpGroup) setMatch(req *http.Request, m *RouteMatch, r *Route) {\n\t// Store host variables.\n\tif v.host != nil {\n\t\thostVars := v.host.regexp.FindStringSubmatch(getHost(req))\n\t\tif hostVars != nil {\n\t\t\tfor k, v := range v.host.varsN {\n\t\t\t\tm.Vars[v] = hostVars[k+1]\n\t\t\t}\n\t\t}\n\t}\n\t// Store path variables.\n\tif v.path != nil {\n\t\tpathVars := v.path.regexp.FindStringSubmatch(req.URL.Path)\n\t\tif pathVars != nil {\n\t\t\tfor k, v := range v.path.varsN {\n\t\t\t\tm.Vars[v] = pathVars[k+1]\n\t\t\t}\n\t\t\t// Check if we should redirect.\n\t\t\tif v.path.strictSlash {\n\t\t\t\tp1 := strings.HasSuffix(req.URL.Path, "/")\n\t\t\t\tp2 := strings.HasSuffix(v.path.template, "/")\n\t\t\t\tif p1 != p2 {\n\t\t\t\t\tu, _ := url.Parse(req.URL.String())\n\t\t\t\t\tif p1 {\n\t\t\t\t\t\tu.Path = u.Path[:len(u.Path)-1]\n\t\t\t\t\t} else {\n\t\t\t\t\t\tu.Path += "/"\n\t\t\t\t\t}\n\t\t\t\t\tm.Handler = http.RedirectHandler(u.String(), 301)\n\t\t\t\t}\n\t\t\t}\n\t\t}\n\t}\n\t// Store query string variables.\n\tfor _, q := range v.queries {\n\t\tqueryVars := q.regexp.FindStringSubmatch(q.getUrlQuery(req))\n\t\tif queryVars != nil {\n\t\t\tfor k, v := range q.varsN {\n\t\t\t\tm.Vars[v] = queryVars[k+1]\n\t\t\t}\n\t\t}\n\t}\n}\n\n// getHost tries its best to return the request host.\nfunc getHost(r *http.Request) string {\n\tif r.URL.IsAbs() {\n\t\treturn r.URL.Host\n\t}\n\thost := r.Host\n\t// Slice off any port information.\n\tif i := strings.Index(host, ":"); i != -1 {\n\t\thost = host[:i]\n\t}\n\treturn host\n\n}\n',
                                },
                            },
                        },
                    }),
                    RepositoryComparisonDiff: () => ({
                        repository: {
                            comparison: {
                                fileDiffs: {
                                    totalCount: 2,
                                    nodes: [
                                        {
                                            oldPath: 'mux_test.go',
                                            newPath: 'mux_test.go',
                                            internalID: '3f13e90b2675f05493474c5403806dd0',
                                        },
                                        {
                                            oldPath: 'regexp.go',
                                            newPath: 'regexp.go',
                                            internalID: '7907dc0efec1833675561b8c7f402e59',
                                        },
                                    ],
                                },
                            },
                        },
                    }),
                })

                // Serve a mock extension that displays the revision in the hover overlay.
                mockExtension({
                    id: 'show/revision',
                    bundle: function extensionBundle(): void {
                        // eslint-disable-next-line @typescript-eslint/no-require-imports, @typescript-eslint/no-var-requires
                        const sourcegraph = require('sourcegraph') as typeof import('sourcegraph')

                        function activate(context: ExtensionContext): void {
                            context.subscriptions.add(
                                sourcegraph.languages.registerHoverProvider(['*'], {
                                    provideHover: (document, position) => {
                                        const lines = document.text?.split('\n')
                                        if (!lines) {
                                            return null
                                        }
                                        const line = lines[position.line]
                                        const hoverIndex = position.character
                                        let startCharacter = hoverIndex
                                        let endCharacter = hoverIndex

                                        while (line[startCharacter - 1].match(/\w/)) {
                                            startCharacter--
                                        }
                                        while (line[endCharacter + 1].match(/\w/)) {
                                            endCharacter++
                                        }
                                        endCharacter++ // Not inclusive

                                        const range = new sourcegraph.Range(
                                            new sourcegraph.Position(position.line, startCharacter),
                                            new sourcegraph.Position(position.line, endCharacter)
                                        )
                                        const token = line.slice(startCharacter, endCharacter)

                                        const parsed = new URL(document.uri)
                                        const revision = decodeURIComponent(parsed.search.slice('?'.length))

                                        return {
                                            contents: {
                                                value: `User is hovering over ${token}, revision: ${revision}`,
                                                kind: sourcegraph.MarkupKind.Markdown,
                                            },
                                            range,
                                        }
                                    },
                                })
                            )
                        }

                        exports.activate = activate
                    },
                })
            })

            // regexp.go
            const tokens = {
                // https://github.com/gorilla/mux/pull/117/files#diff-9ef8a22c4ce5141c30a501c542fb1adeL244
                base: {
                    token: 'varsN',
                    lineId: 'diff-a609417fa264c6aed88fb8cfe2d9b4fb24226ffdf7db1f685e344d5239783d46L244',
                    commitID: 'f15e0c49460fd49eebe2bcc8486b05d1bef68d3a',
                },
                // https://github.com/gorilla/mux/pull/117/files#diff-9ef8a22c4ce5141c30a501c542fb1adeR247
                head: {
                    token: 'host',
                    lineId: 'diff-a609417fa264c6aed88fb8cfe2d9b4fb24226ffdf7db1f685e344d5239783d46R247',
                    commitID: 'e73f183699f8ab7d54609771e1fa0ab7ffddc21b',
                },
            }

            it('provides hover tooltips for pull requests in unified mode', async () => {
                await driver.page.goto('https://github.com/gorilla/mux/pull/117/files?diff=unified')

                // The browser extension takes a bit to initialize and register all event listeners.
                // Waiting here saves one retry cycle below in the common case.
                // If it's not enough, the retry will catch it.
                await driver.page.waitForTimeout(1500)

                // Base
                const baseTokenElement = await retry(async () => {
                    const lineNumberElement = await driver.page.waitForSelector(`#${tokens.base.lineId}`, {
                        timeout: 10000,
                    })
                    const row = (
                        await driver.page.evaluateHandle((element: Element) => element.closest('tr'), lineNumberElement)
                    ).asElement()!
                    assert(row, 'Expected row to exist')
                    const tokenElement = (
                        await driver.page.evaluateHandle(
                            (row: Element, token: string) =>
                                [...row.querySelectorAll('span')].find(element => element.textContent === token),
                            row,
                            tokens.base.token
                        )
                    ).asElement()
                    assert(tokenElement, 'Expected token element to exist')
                    return tokenElement
                })
                // Retry is here to wait for listeners to be registered
                await retry(async () => {
                    await baseTokenElement.hover()
                    await driver.page.waitForSelector('[data-testid="hover-overlay-content"] > p', { timeout: 5000 })

                    try {
                        await driver.findElementWithText(
                            `User is hovering over ${tokens.base.token}, revision: ${tokens.base.commitID}`,
                            {
                                selector: '[data-testid="hover-overlay-content"] > p',
                                fuzziness: 'contains',
                                wait: {
                                    timeout: 6000,
                                },
                            }
                        )
                    } catch {
                        throw new Error('Timed out waiting for hover tooltip for base side.')
                    }
                })

                // Head
                const headTokenElement = await (async () => {
                    const lineNumberElement = await driver.page.waitForSelector(`#${tokens.head.lineId}`, {
                        timeout: 10000,
                    })
                    const row = (
                        await driver.page.evaluateHandle((element: Element) => element.closest('tr'), lineNumberElement)
                    ).asElement()!
                    assert(row, 'Expected row to exist')
                    const tokenElement = (
                        await driver.page.evaluateHandle(
                            (row: Element, token: string) =>
                                [...row.querySelectorAll('span')].find(element => element.textContent === token),
                            row,
                            tokens.head.token
                        )
                    ).asElement()
                    assert(tokenElement, 'Expected token element to exist')
                    return tokenElement
                })()
                await headTokenElement.hover()

                try {
                    await driver.findElementWithText(
                        `User is hovering over ${tokens.head.token}, revision: ${tokens.head.commitID}`,
                        {
                            selector: '[data-testid="hover-overlay-content"] > p',
                            fuzziness: 'contains',
                            wait: {
                                timeout: 6000,
                            },
                        }
                    )
                } catch {
                    throw new Error('Timed out waiting for hover tooltip for head side.')
                }
            })

            it('provides hover tooltips for pull requests in split mode', async () => {
                await driver.page.goto('https://github.com/gorilla/mux/pull/117/files?diff=split')

                // The browser extension takes a bit to initialize and register all event listeners.
                // Waiting here saves one retry cycle below in the common case.
                // If it's not enough, the retry will catch it.
                await driver.page.waitForTimeout(1500)

                // Base
                const baseTokenElement = await retry(async () => {
                    const lineNumberElement = await driver.page.waitForSelector(`#${tokens.base.lineId}`, {
                        timeout: 10000,
                    })
                    const row = (
                        await driver.page.evaluateHandle((element: Element) => element.closest('tr'), lineNumberElement)
                    ).asElement()!
                    assert(row, 'Expected row to exist')
                    const tokenElement = (
                        await driver.page.evaluateHandle(
                            (row: Element, token: string) =>
                                [...row.querySelectorAll('span')].find(element => element.textContent === token),
                            row,
                            tokens.base.token
                        )
                    ).asElement()
                    assert(tokenElement, 'Expected token element to exist')
                    return tokenElement
                })
                // Retry is here to wait for listeners to be registered
                await retry(async () => {
                    await baseTokenElement.hover()
                    await driver.page.waitForSelector('[data-testid="hover-overlay-content"] > p', { timeout: 5000 })

                    try {
                        await driver.findElementWithText(
                            `User is hovering over ${tokens.base.token}, revision: ${tokens.base.commitID}`,
                            {
                                selector: '[data-testid="hover-overlay-content"] > p',
                                fuzziness: 'contains',
                                wait: {
                                    timeout: 6000,
                                },
                            }
                        )
                    } catch {
                        throw new Error('Timed out waiting for hover tooltip for base side.')
                    }
                })

                // Head
                const headTokenElement = await (async () => {
                    const lineNumberElement = await driver.page.waitForSelector(`#${tokens.head.lineId}`, {
                        timeout: 10000,
                    })
                    const row = (
                        await driver.page.evaluateHandle((element: Element) => element.closest('tr'), lineNumberElement)
                    ).asElement()!
                    assert(row, 'Expected row to exist')
                    const tokenElement = (
                        await driver.page.evaluateHandle(
                            (row: Element, token: string) =>
                                [...row.querySelectorAll('span')].find(element => element.textContent === token),
                            row,
                            tokens.head.token
                        )
                    ).asElement()
                    assert(tokenElement, 'Expected token element to exist')
                    return tokenElement
                })()
                await headTokenElement.hover()

                try {
                    await driver.findElementWithText(
                        `User is hovering over ${tokens.head.token}, revision: ${tokens.head.commitID}`,
                        {
                            selector: '[data-testid="hover-overlay-content"] > p',
                            fuzziness: 'contains',
                            wait: {
                                timeout: 6000,
                            },
                        }
                    )
                } catch {
                    throw new Error('Timed out waiting for hover tooltip for head side.')
                }
            })
        })

        // TODO(sqs): skipped because these have not been reimplemented after the extension API deprecation
        describe.skip('Commit view', () => {
            beforeEach(() => {
                mockUrls([
                    'https://github.com/*path/find-definition',
                    'https://github.com/**/commits/checks-statuses-rollups',
                    'https://github.com/commits/badges',
                ])

                const { mockExtension } = setupExtensionMocking()

                const userSettings: Settings = {}
                testContext.overrideGraphQL({
                    ViewerConfiguration: () => ({
                        viewerConfiguration: {
                            subjects: [
                                {
                                    __typename: 'User',
                                    displayName: 'Test User',
                                    id: 'TestUserSettingsID',
                                    latestSettings: {
                                        id: 123,
                                        contents: JSON.stringify(userSettings),
                                    },
                                    username: 'test',
                                    viewerCanAdminister: true,
                                    settingsURL: '/users/test/settings',
                                },
                            ],
                            merged: { contents: JSON.stringify(userSettings), messages: [] },
                        },
                    }),
                    RepositoryComparisonDiff: () => ({
                        repository: {
                            comparison: {
                                fileDiffs: {
                                    nodes: [
                                        {
                                            oldPath: 'mux.go',
                                            newPath: 'mux.go',
                                            internalID: '19f8c0b76d9f9caa0ba82d49c988f9f5',
                                        },
                                    ],
                                    totalCount: 1,
                                },
                            },
                        },
                    }),
                })

                // Serve a mock extension with a simple hover provider
                mockExtension({
                    id: 'simple/hover',
                    bundle: simpleHoverProvider,
                })
            })

            it('has Sourcegraph icon button and provides hover tooltips for pull requests in unified mode', async () => {
                await driver.page.goto(
                    'https://github.com/gorilla/mux/pull/613/commits/0759b72aecaaf40b02af1ebf032e5a23d7a4bedf?diff=unified'
                )

                await driver.page.waitForSelector(
                    '[data-testid="code-view-toolbar"] [data-testid="open-on-sourcegraph"]'
                )

                // Pause to give codeintellify time to register listeners for
                // tokenization (only necessary in CI, not sure why).
                await driver.page.waitForTimeout(1000)

                const lineSelector = '.diff-table tr'

                // Trigger tokenization of the line.
                const lineNumber = 7
                const line = await driver.page.waitForSelector(`${lineSelector}:nth-child(${lineNumber})`)

                if (!line) {
                    throw new Error(`Found no line with number ${lineNumber}`)
                }

                const [token] = await line.$x('.//span[text()="HandlerFunc"]')
                await token.hover()

                await driver.page.waitForSelector('[data-testid="hover-overlay-contents"]')
            })

            it('has Sourcegraph icon button and provides hover tooltips for pull requests in split mode', async () => {
                await driver.page.goto(
                    'https://github.com/gorilla/mux/pull/613/commits/0759b72aecaaf40b02af1ebf032e5a23d7a4bedf?diff=split'
                )

                await driver.page.waitForSelector(
                    '[data-testid="code-view-toolbar"] [data-testid="open-on-sourcegraph"]'
                )

                // Pause to give codeintellify time to register listeners for
                // tokenization (only necessary in CI, not sure why).
                await driver.page.waitForTimeout(1000)

                const lineSelector = '.diff-table.file-diff-split tr'

                // Trigger tokenization of the line.
                const lineNumber = 6
                const line = await driver.page.waitForSelector(`${lineSelector}:nth-child(${lineNumber})`)

                if (!line) {
                    throw new Error(`Found no line with number ${lineNumber}`)
                }

                const [token] = await line.$x('.//span[text()="HandlerFunc"]')
                await token.hover()

                await driver.page.waitForSelector('[data-testid="hover-overlay-contents"]')
            })
        })
    })

    // TODO(#44327): Search on Sourcegraph buttons were removed from GitHub search pages.
    // We need to reenable these tests if we decide to keep those buttons or delete them if we don't.
    describe.skip('Search pages', () => {
        const sourcegraphSearchPage = 'https://sourcegraph.com/search'

        const pages = [
            { name: 'Simple search page', url: 'https://github.com/search' },
            { name: 'Advanced search page', url: 'https://github.com/search/advanced' },
        ]

        for (const page of pages) {
            describe(page.name, () => {
                it('if search input has value "Search on Sourcegraph" click navigates to Sourcegraph search page with type "repo" and search query', async () => {
                    await driver.newPage()
                    await driver.page.goto(page.url)

                    const query = 'Hello world!'
                    const searchInput = await driver.page.waitForSelector('#search_form input[type="text"]')
                    const linkToSourcegraph = await driver.page.waitForSelector(
                        '[data-testid="search-on-sourcegraph"]',
                        { timeout: 3000 }
                    )

                    assert(linkToSourcegraph, 'Expected link to Sourcegraph search page exists')

                    let hasRedirectedToSourcegraphSearch = false
                    testContext.server.get(sourcegraphSearchPage).intercept(request => {
                        if (['type:repo', query].every(value => request.query.q?.includes(value))) {
                            hasRedirectedToSourcegraphSearch = true
                        }
                    })

                    await searchInput?.type(query, { delay: 100 })
                    await linkToSourcegraph?.click()
                    await driver.page.waitForTimeout(1000)

                    assert(
                        hasRedirectedToSourcegraphSearch,
                        'Expected to be redirected to Sourcegraph search page with type "repo" and search query'
                    )
                })
            })
        }

        const isRecordMode = readEnvironmentString({ variable: 'POLLYJS_MODE', defaultValue: 'replay' }) === 'record'
        const isCI = readEnvironmentBoolean({ variable: 'CI', defaultValue: false }) === true

        // global and repository search pages
        // do not record in CI (see https://github.com/sourcegraph/sourcegraph/pull/34171)
        ;(isRecordMode && isCI ? describe.skip : describe)('Search results page', () => {
            beforeEach(() => {
                mockUrls([
                    'https://github.com/_graphql/GetSuggestedNavigationDestinations',
                    'https://github.com/**/commits/checks-statuses-rollups',
                    'https://github.com/commits/badges',
                ])
            })

            const buildGitHubSearchResultsURL = (page: string, searchTerm: string): string => {
                const url = new URL(page)
                url.searchParams.set('q', searchTerm)
                return url.toString()
            }

            const globalSearchPage = 'https://github.com/search'
            const repo = 'sourcegraph/sourcegraph'
            const repoSearchPage = `https://github.com/${repo}/search`

            const pages = [
                { name: 'Global search page', url: globalSearchPage },
                { name: 'Repo search page', url: repoSearchPage },
            ]

            const viewportM = { width: 768, height: 1024 }
            const viewportL = { width: 1024, height: 768 }

            const viewportConfigs = [
                {
                    name: 'M',
                    viewport: viewportM,
                    sourcegraphButtonSelector: '#pageSearchFormSourcegraphButton [data-testid="search-on-sourcegraph"]',
                    searchInputSelector: ".application-main form.js-site-search-form input.form-control[name='q']",
                },
                {
                    name: 'L',
                    viewport: viewportL,
                    searchInputSelector: "header form.js-site-search-form input.form-control[name='q']",
                    sourcegraphButtonSelector:
                        '#headerSearchInputSourcegraphButton [data-testid="search-on-sourcegraph"]',
                },
            ]

            for (let index = 0; index < pages.length; index++) {
                // Reduce the number of tests for search results pages.
                // We record and run the global search results page test on a smaller viewport (M) and repo search results page on a larger (L).
                // As these two pages follow the same logic and use similar search input elements per viewport we're still able to catch regressions, but reduce:
                // - the number of calls to GitHub search API which can result in 429 HTTP Errors (Too Many Requests)
                // - the size of recordings added to the repository.
                const page = pages[index]
                const viewportConfig = viewportConfigs[index]

                describe(`${page.name}: viewport ${viewportConfig.name}`, () => {
                    it('"Search on Sourcegraph" click navigates to Sourcegraph search page with proper result type, language and search query from search input', async () => {
                        const initialQuery = 'fix'

                        const url = buildGitHubSearchResultsURL(page.url, initialQuery)
                        const query = 'issue'

                        await driver.newPage()
                        await driver.page.goto(url)
                        await driver.page.setViewport(viewportConfig.viewport)

                        let hasRedirectedToSourcegraphSearch = false
                        let lang = ''
                        testContext.server.get(sourcegraphSearchPage).intercept(request => {
                            const resultQuery = `${initialQuery} ${query}`
                            const parameters = ['type:commit', `lang:${lang}`, resultQuery]

                            if (page.url === repoSearchPage) {
                                parameters.push(`repo:${repo}`)
                            }

                            hasRedirectedToSourcegraphSearch = parameters.every(value =>
                                request.query.q?.includes(value)
                            )
                        })

                        // filter results by language (handled by client-side routing)
                        const langLinkHandle = await driver.page.$('ul.filter-list li:first-child a.filter-item')
                        assert(langLinkHandle, 'Expected language result type link to exist')
                        lang = await langLinkHandle.evaluate(node => {
                            if (!(node instanceof HTMLAnchorElement) || !node.href) {
                                return ''
                            }

                            return new URL(node.href).searchParams.get('l') || ''
                        })
                        await langLinkHandle.click()
                        await driver.page.waitForTimeout(3000)

                        // filter results by type (handled by client-side routing)
                        const commitsLinkHandle = await driver.page.$("nav.menu a.menu-item[href*='type=commits']")
                        assert(commitsLinkHandle, 'Expected commits result type link to exist')
                        await commitsLinkHandle.click()
                        await driver.page.waitForTimeout(3000)

                        const searchInput = await driver.page.waitForSelector(viewportConfig.searchInputSelector)
                        // For some reason puppeteer when typing into input field prepends the exising value.
                        // To replicate the natural behavior we navigate to the end of exisiting value and then start typing.
                        await searchInput?.focus()
                        for (const _char of initialQuery) {
                            await driver.page.keyboard.press('ArrowRight')
                        }
                        await searchInput?.type(` ${query}`, { delay: 100 })
                        await driver.page.keyboard.press('Escape') // if input focus opened dropdown, ensure the latter is closed

                        const linkToSourcegraph = await driver.page.waitForSelector(
                            viewportConfig.sourcegraphButtonSelector,
                            {
                                timeout: 3000,
                            }
                        )
                        assert(linkToSourcegraph, 'Expected link to Sourcegraph search page exists')
                        await linkToSourcegraph?.click()
                        await driver.page.waitForTimeout(3000)

                        assert(
                            hasRedirectedToSourcegraphSearch,
                            'Expected to be redirected to Sourcegraph search page with type "commit", language "HTML" and search query'
                        )
                    })
                })
            }
        })
    })
})
