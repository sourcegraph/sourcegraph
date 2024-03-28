import { registerLanguage } from 'highlight.js/lib/core'
import bash from 'highlight.js/lib/languages/bash'
import cLike from 'highlight.js/lib/languages/c-like'
import clojure from 'highlight.js/lib/languages/clojure'
import cpp from 'highlight.js/lib/languages/cpp'
import csharp from 'highlight.js/lib/languages/csharp'
import css from 'highlight.js/lib/languages/css'
import dart from 'highlight.js/lib/languages/dart'
import diff from 'highlight.js/lib/languages/diff'
import dockerfile from 'highlight.js/lib/languages/dockerfile'
import elixir from 'highlight.js/lib/languages/elixir'
import goLang from 'highlight.js/lib/languages/go'
import haskell from 'highlight.js/lib/languages/haskell'
import http from 'highlight.js/lib/languages/http'
import java from 'highlight.js/lib/languages/java'
import javascript from 'highlight.js/lib/languages/javascript'
import json from 'highlight.js/lib/languages/json'
import kotlin from 'highlight.js/lib/languages/kotlin'
import lua from 'highlight.js/lib/languages/lua'
import markdown from 'highlight.js/lib/languages/markdown'
import ocaml from 'highlight.js/lib/languages/ocaml'
import perl from 'highlight.js/lib/languages/perl'
import php from 'highlight.js/lib/languages/php'
import python from 'highlight.js/lib/languages/python'
import rLang from 'highlight.js/lib/languages/r'
import ruby from 'highlight.js/lib/languages/ruby'
import rust from 'highlight.js/lib/languages/rust'
import scala from 'highlight.js/lib/languages/scala'
import swift from 'highlight.js/lib/languages/swift'
import typescript from 'highlight.js/lib/languages/typescript'
import xml from 'highlight.js/lib/languages/xml'
import yaml from 'highlight.js/lib/languages/yaml'

// TODO(sqs): copied from sourcegraph/sourcegraph. should dedupe.

let registered = false

/**
 * Registers syntax highlighters for commonly used languages.
 *
 * This function must be called exactly once. A function is used instead of having the registerLanguage calls be
 * side effects of importing this module to prevent this module from being omitted from production builds due to
 * tree-shaking.
 */
export function registerHighlightContributions(): void {
    if (registered) {
        // Don't double-register these. (There is no way to unregister them.)
        return
    }
    registered = true
    registerLanguage('go', goLang)
    registerLanguage('javascript', javascript)
    registerLanguage('typescript', typescript)
    registerLanguage('java', java)
    registerLanguage('python', python)
    registerLanguage('php', php)
    registerLanguage('bash', bash)
    registerLanguage('clojure', clojure)
    // This is a dependency of cpp.
    registerLanguage('c-like', cLike)
    registerLanguage('cpp', cpp)
    registerLanguage('cs', csharp)
    registerLanguage('csharp', csharp)
    registerLanguage('css', css)
    registerLanguage('dockerfile', dockerfile)
    registerLanguage('elixir', elixir)
    registerLanguage('haskell', haskell)
    registerLanguage('html', xml)
    registerLanguage('lua', lua)
    registerLanguage('ocaml', ocaml)
    registerLanguage('r', rLang)
    registerLanguage('ruby', ruby)
    registerLanguage('rust', rust)
    registerLanguage('swift', swift)
    registerLanguage('markdown', markdown)
    registerLanguage('diff', diff)
    registerLanguage('json', json)
    registerLanguage('jsonc', json)
    registerLanguage('yaml', yaml)
    registerLanguage('kotlin', kotlin)
    registerLanguage('dart', dart)
    registerLanguage('perl', perl)
    registerLanguage('scala', scala)
    // Apex is not supported by highlight.js, but it's very close to Java.
    registerLanguage('apex', java)
    // We use HTTP to render incoming webhook deliveries.
    registerLanguage('http', http)
}
