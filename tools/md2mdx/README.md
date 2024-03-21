# md2mdx

> WARNING

If you arrived here from a search engine, this is not a full markdown to `mdx` converter. This is merely a hack
to turn our generated docs from the monitoring stack and `src-cli` which are outputting markdown into `mdx`.

This is heavily tied to our content, and is built empirically. TL;DR Don't use this.

---

Right now, the docsite served at https://sourcegraph.com/docs uses a different stack than the previous docsite.
Most notably, it uses the `mdx` format, which is full of quirks and to be frank, quite unpredictable (which is a
big departure from the original spirit of markdown if I were to disgress).

For example, there are times where you need to escape `|` like inside backticks. And `{` needs to be escaped everywhere but inside backticks.
Luckily the triple backticks are safe.

## Usage

It takes the path to a `.md` file and prints the converted output on `stdout`.

```
md2mdx [docs root]/your/file/in/markdown.md
```

It's important to make sure to start from the docs root, because there's a bug with linking from an `index.md` because the new docsite
eliminates the trailing slash for index pages, but doesn't account for that when rendering the links.

## Contributing

As mentioned above, it's pretty empirical. When finding some broken rendering, add a unit test in `main_test.go` and work out
some way to get it right.
