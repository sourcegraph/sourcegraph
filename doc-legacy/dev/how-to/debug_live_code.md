# How to debug live code

How to debug a program with Visual Studio Code:

## Debug TypeScript code

Requires "Debugger for Chrome" extension.

- Quit Chrome
- Launch Chrome (Canary) from the command line with a remote debugging port:
  - Mac OS: `/Applications/Google\ Chrome\ Canary.app/Contents/MacOS/Google\ Chrome\ Canary --remote-debugging-port=9222`
  - Windows: `start chrome.exe –remote-debugging-port=9222`
  - Linux: `chromium-browser --remote-debugging-port=9222`
- Go to http://localhost:3080
- Open the Debugger in VSCode: "View" > "Debug"
- Launch the `(ui) http://localhost:3080/*` debug configuration
- Set breakpoints, enjoy

## Debug Go code

Instructions for attaching with Goland [here](#attaching-via-goland)

Install [Delve](https://github.com/derekparker/delve):

```bash
xcode-select --install
pushd /tmp
go get github.com/go-delve/delve/cmd/dlv
popd /tmp
```

Then install `pgrep`:

```bash
brew install proctools
```

Make sure to run `env DELVE=true sg start` to disable optimizations during compilation, otherwise Delve will have difficulty stepping through optimized functions (line numbers will be off, you won't be able to print local variables, etc.).

Now you can attach a debugger to any Go process (e.g. frontend, searcher, go-langserver) in 1 command:

```bash
dlv attach $(pgrep frontend)
```

Delve will pause the process once it attaches the debugger. Most used [commands](https://github.com/go-delve/delve/tree/master/Documentation/cli):

- `b internal/db/access_tokens.go:52` to set a breakpoint on a line (`bp` lists all, `clearall` deletes all)
- `c` to continue execution of the program
- `Ctrl-C` pause the program to bring back the command prompt
- `n` to step over the next statement
- `s` to step into the next function call
- `stepout` to step out of the current function call
- `Ctrl-D` to exit

## Attaching via Goland

After running with `env DELVE=true sg start` you may use Run | Attach to Process in Goland to debug a running Go binary (⌥⇧F5 on macOS).
