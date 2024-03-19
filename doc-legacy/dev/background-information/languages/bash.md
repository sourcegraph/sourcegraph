# Bash style

Bash is frequently used in our build, CI and deployment systems. Some general guidelines and recommended reading

- [Bash style](#bash-style)
  - [Reading from a file](#reading-from-a-file)
  - [Set -eu -o pipefail](#set--eu--o-pipefail)
  - [References](#references)

### Reading from a file

Prefer using `mapfile` instead of `while IFS` to read a file

```bash
mapfile -t myArray < file.txt
```

```bash
mapfile -t myArray < <(find -d .)
```

instead of

```bash
input="/path/to/txt/file"
while IFS= read -r line
do
  echo "$line"
done < "$input"
```

### Set -eu -o pipefail

This is generally "bash strict mode" and  sets

- `e` exit if error
- `u` error on variable unset (and exit)
- `-o pipefail` fail if items in the pipe | fail. Bash otherwise continues if error | pass which causes some unexpected behavior.

Recommend using these at the start of all scripts and specifically disabling if a section of a bash script does not need them (for example, you want to let a pipe fail).

### Utilities in use

We use `shfmt` and `shellcheck` for our shell script linters

## References

<https://wiki.bash-hackers.org/>
<https://www.notion.so/daxmc99/One-liners-Basic-Cheat-sheet-Linux-Command-Library-b3676fb5a49b44f8a507cce0185ca5d7>
