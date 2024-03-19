# Writing an indexer

This page describes the [SCIP Code Intelligence Protocol](https://github.com/sourcegraph/scip)
and how you can write an indexer to emit SCIP.

At a high level, you need to follow these steps:

1. Familiarize yourself with the [SCIP protobuf schema][].
1. Import or generate SCIP bindings.
1. Generate minimal index with occurrence information.
1. Test your indexer using [scip CLI][]'s `snapshot` subcommand.
1. Progressively add support for more features with tests.

If you run into problems or have questions for any of these steps,
please open an issue on the [SCIP issue tracker][].

[SCIP protobuf schema]: https://github.com/sourcegraph/scip/blob/main/scip.proto

[scip CLI]: https://github.com/sourcegraph/scip#scip-cli-reference

[SCIP issue tracker]: https://github.com/sourcegraph/scip/issues

Let's go over each step one-by-one.

## Understanding the SCIP protobuf schema

The [SCIP protobuf schema][] describes the structure
of a SCIP index in a machine-readable format.

The main structure is an [`Index`][]
which consists of a list of documents
along with some metadata.
Optionally, an index can also provide
hover documentation for external symbols
that will not be indexed.

[`Index`]: https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/scip%24+%5Emessage+Index+%7B%5Cn%28.%2B%5Cn%29%2B%7D&patternType=regexp

A [`Document`][] has a unique path relative to the project root.
It also has a list of occurrences,
which attach information to source ranges,
as well as a list of symbols that are defined
in the document.

[`Document`]: https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/scip%24+message+Document+%7B...%7D&patternType=structural

The information covered by an [`Occurrence`][] can be syntactic or semantic:

- Syntactic information such as the `syntax_kind` field
  is used for highlighting.
- Semantic information such as the `symbol` and `symbol_role` fields
  are used to power code navigation features
  like Go to definition and Find references.

[`Occurrence`]: https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/scip%24+message+Occurrence+%7B...%7D&patternType=structural

Occurrences also allow attaching diagnostic information,
which can be used by static analysis tools.

For more details, see the doc comments
in the [SCIP protobuf schema][].

You may also find it helpful
to see how existing indexers emit information.
For example, you can take a look at
the [scip-typescript][] or [scip-java][] code
to see how they emit SCIP indexes.

[scip-typescript]: https://github.com/sourcegraph/scip-typescript
[scip-java]: https://github.com/sourcegraph/scip-java

## Importing or generating SCIP bindings

The SCIP repository contains bindings for several languages.

Depending on your indexer's implementation language,
you can import the bindings directly using your language's package manager,
or by using git submodules.
One benefit of this approach is that you do not need to
have a protobuf toolchain to generate code from the schema.
This also makes it easier to bump the version of SCIP to pick up
newer changes to the schema.

Alternately, you can vendor the SCIP protobuf schema into your repository
and set up Protobuf generation yourself.
This has the benefit of being able to control the process
from end-to-end, at the cost of making updates a bit more cumbersome.

<!-- TODO: Is it OK to make this promise here? -->
Newer Sourcegraph versions will maintain backwards compatibility
with older SCIP versions, so there is no risk of not being able
to upload SCIP indexes if a vendored schema has not been updated
in a while.

## Generating minimal index with occurrence information

As a first pass,
we recommend generating occurrences for a subset of declarations
and checking that the generation works from end-to-end.

In the context of an indexer,
this typically involves using a compiler frontend or a language server as a library.
First, run the compiler pipeline until semantic analysis is completed.
Next, perform a top-down traversal of ASTs for all files,
recording information about different kinds of occurrences.

At the end, write a conversion pass from the intermediate
data to SCIP using the SCIP bindings.

As a convention, indexers should use `index.scip` as the default filename
for the output. The [Sourcegraph CLI][] recognizes this filename and uses
it as the default upload path.

[Sourcegraph CLI]: https://github.com/sourcegraph/src-cli

You can inspect the Protobuf output using `protoc`:

```sh
# assuming scip.proto and index.scip are in the current directory
protoc --decode=scip.Index scip.proto < index.scip
```

For robust testing,
we recommend making sure that the result of indexing is deterministic.
One potential source of issues here is non-determinstic
iteration over the key-value pairs of a hash table.
If re-running your indexer changes the order in which occurrences are emitted,
snapshot testing may report different results.

## Snapshot testing with scip CLI

One of the key design criteria for SCIP
was that it should be easy to understand an index file
and test an indexer for correctness.

The [scip CLI][] has a `snapshot` subcommand
which can be used for golden testing.
It `snapshot` command inspects an index file
and regenerates the source code,
attaching comments describing occurrence information.

Here is slightly cleaned up snippet from running
`scip snapshot` on the index generated by
running `scip-typescript` over itself:

```ts
  function scriptElementKind(
//         ^^^^^^^^^^^^^^^^^ definition scip-typescript npm @sourcegraph/scip-typescript 0.2.0 src/FileIndexer.ts/scriptElementKind().
    node: ts.Node,
//  ^^^^ definition scip-typescript npm @sourcegraph/scip-typescript 0.2.0 src/FileIndexer.ts/scriptElementKind().(node)
//        ^^ reference local 1
//           ^^^^ reference scip-typescript npm typescript 4.6.2 lib/typescript.d.ts/ts/Node#
    sym: ts.Symbol
//  ^^^ definition scip-typescript npm @sourcegraph/scip-typescript 0.2.0 src/FileIndexer.ts/scriptElementKind().(sym)
//  documentation ```ts
//       ^^ reference local 1
//          ^^^^^^ reference scip-typescript npm typescript 4.6.2 lib/typescript.d.ts/ts/Symbol#
  ): ts.ScriptElementKind {
//   ^^ reference local 1
//      ^^^^^^^^^^^^^^^^^ reference scip-typescript npm typescript 4.6.2 lib/typescript.d.ts/ts/ScriptElementKind#
```

The carets and contextual information make it easy to visually check that:

- Occurrences are being emitted for the right source ranges.
- Occurrences have the expected symbol strings.
  The exact syntax for the symbol strings is described
  in the doc comment for [`Symbol`][] in the SCIP Protobuf schema.
- Symbols correspond to the right package.
  For example, the `ScriptElementKind` is defined in the
  `typescript` package (the compiler) whereas
  `scriptElementKind` is defined in `@sourcegraph/scip-typescript`.

[`Symbol`]: https://sourcegraph.com/github.com/sourcegraph/scip@12459c75fc15117e68b4c15a58e8581b738b855f/-/blob/scip.proto?L87-115

## Progressively adding support for language features

We recommend adding support for different features in the following order:

1. Emit occurrences and symbols for a single file.
   - Iterate over different kinds of entities (functions, classes, properties etc.)
1. Emit hover documentation for entities.
   If the markup is in a format other than CommonMark,
   we recommend addressing that difference after addressing other features.
1. Add support for implementation relationships, enabling Find implementations.
1. (Optional) If the hover documentation uses markup in a format other than CommonMark,
   implement a conversion from the custom markup language to CommonMark.

> NOTE: If implementing an indexer in C++, you need to careful about
> very large indexes, as the default Protoc-generated code has a
> 2GB implementation limit on message sizes. This limit can be worked
> around by writing documents to disk one-by-one. For example,
> this PR [adds support to scip-ruby for writing documents one-by-one](https://github.com/sourcegraph/scip-ruby/pull/190).
