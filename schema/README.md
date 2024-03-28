# Sourcegraph JSON Schemas

[JSON Schema](http://json-schema.org/) is a way to define the structure of a JSON document. It enables typechecking and code intelligence on JSON documents.

Sourcegraph uses the following JSON Schemas:

- [`settings.schema.json`](./settings.schema.json)
- [`critical.schema.json`](./critical.schema.json)
- [`site.schema.json`](./site.schema.json)

# Modifying a schema

1.  Edit the `*.schema.json` file in this directory.
1.  Run `bazel run //schema:write_generated_schema`.
1.  Commit the changes to both files.
1.  Run `sg start` to automatically update TypeScript schema files.

## Known issues

- The JSON Schema IDs (URIs) are of the form `https://sourcegraph.com/v1/*.schema.json#`, but these are not actually valid URLs. This means you generally need to supply them to JSON Schema validation libraries manually instead of having the validator fetch the schema from the web.
Hello World
