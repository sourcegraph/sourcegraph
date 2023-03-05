import { MarkupKind } from '@sourcegraph/extension-api-classes'
import type { MarkupContent, Badged, AggregableBadge } from '@sourcegraph/shared/src/codeintel/legacy-extensions/api'
import { FIXTURE_SEMANTIC_BADGE } from '@sourcegraph/shared/src/hover/HoverOverlay.fixtures'

export const FIXTURE_CONTENT_LONG_CODE: Badged<MarkupContent> = {
    kind: MarkupKind.Markdown,
    value:
        '```typescript\nexport interface LongTestInterface<A, B, C, D, E, F, G, H, I, J, K, L, M, N, O, P, Q, R, S, T, U, V, W, X, Y, Z, A, B, C, D, E, F, G, H, I, J, K, L, M, N, O, P, Q, R, S, T, U, V, W, X, Y, Z>\n```\n\n' +
        '---\n\nNisi id deserunt culpa dolore aute pariatur ut amet veniam. Proident id Lorem reprehenderit veniam sunt velit.\n',
}

export const FIXTURE_CONTENT_MARKDOWN: Badged<MarkupContent> = {
    kind: MarkupKind.Markdown,
    value: '<div>Hello from the unexpected value! It uses Markdown and wraps content into a "div" element. To render things easily. <b>Cool!<b></div>',
}

export const FIXTURE_CONTENT_LONG_TEXT_ONLY: Badged<MarkupContent> = {
    value: 'WebAssembly (sometimes abbreviated Wasm) is an open standard that defines a portable binary-code format for executable programs, and a corresponding textual assembly language, as well as interfaces for facilitating interactions between such programs and their host environment. The main goal of WebAssembly is to enable high-performance applications on web pages, but the format is designed to be executed and integrated in other environments as well, including standalone ones.  WebAssembly (i.e. WebAssembly Core Specification (then version 1.0, 1.1 is in draft) and WebAssembly JavaScript Interface) became a World Wide Web Consortium recommendation on 5 December 2019, alongside HTML, CSS, and JavaScript. WebAssembly can support (at least in theory) any language (e.g. compiled or interpreted) on any operating system (with help of appropriate tools), and in practice all of the most popular languages already have at least some level of support.  The Emscripten SDK can compile any LLVM-supported languages (such as C, C++ or Rust, among others) source code into a binary file which runs in the same sandbox as JavaScript code. Emscripten provides bindings for several commonly used environment interfaces like WebGL. There is no direct Document Object Model (DOM) access; however, it is possible to create proxy functions for this, for example through stdweb, web_sys, and js_sys when using the Rust language.  WebAssembly implementations usually use either ahead-of-time (AOT) or just-in-time (JIT) compilation, but may also use an interpreter. While the currently best-known implementations may be in web browsers, there are also non-browser implementations for general-purpose use, including WebAssembly Micro Runtime, a Bytecode Alliance project (with subproject iwasm code VM supporting "interpreter, ahead of time compilation (AoT) and Just-in-Time compilation (JIT)"), Wasmtime, and Wasmer.',
}

export const FIXTURE_PARTIAL_BADGE: AggregableBadge = {
    ...FIXTURE_SEMANTIC_BADGE,
    text: 'partial semantic',
}
