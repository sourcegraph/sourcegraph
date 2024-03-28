# scanprotects

`scanprotects` is a command line tool that can scan a Perforce protection table and output debug information about how it was interpreted.

It is intended to be used to debug our parsing logic for protection tables as we often don't have access to them ourselves, so, instead we can send this program to the customer and ask them to run it against their `p4 protects` output.

## Usage

The intention is to pipe the output of `p4 protects` into this tool:

```
p4 protects -u USER | ./scanprotects -d "//some/test/depot/"
```

Note that the output is in JSON format and only a couple of fields are necessary so for best results you should pipe through jq:

```
p4 protects -u USER | ./scanprotects -d "//some/test/depot/" |& jq '{"Body": .Body, "Attributes": .Attributes}'
```

## Example output

```
...
"Processing parsed line"
{
  "line.match": "//depot/.../.../base/foo-test/...",
  "line.isExclusion": false
}
"Relevant depots"
{
  "line.match": "//depot/.../.../base/foo-test/...",
  "line.isExclusion": false,
  "depots": [
    "//depot/main"
  ]
}
"Adding include rules"
{
  "line.match": "//depot/.../.../base/foo-test/...",
  "line.isExclusion": false,
  "rules": [
    "**/**/base/foo-test/**"
  ]
}
"Processing parsed line"
{
  "line.match": "//depot/.../.../base/jkl/test/...",
  "line.isExclusion": false
}
"Relevant depots"
{
  "line.match": "//depot/.../.../base/jkl/test/...",
  "line.isExclusion": false,
  "depots": [
    "//depot/main"
  ]
}
"Adding include rules"
{
  "line.match": "//depot/.../.../base/jkl/test/...",
  "line.isExclusion": false,
  "rules": [
    "**/**/base/jkl/test/**"
  ]
}
"Processing parsed line"
{
  "line.match": "//depot/.../base/foo/config/labels/...",
  "line.isExclusion": false
}
"Relevant depots"
{
  "line.match": "//depot/.../base/foo/config/labels/...",
  "line.isExclusion": false,
  "depots": [
    "//depot/main"
  ]
}
"Adding include rules"
{
  "line.match": "//depot/.../base/foo/config/labels/...",
  "line.isExclusion": false,
  "rules": [
    "**/base/foo/config/labels/**"
  ]
}
...
```
Hello World
