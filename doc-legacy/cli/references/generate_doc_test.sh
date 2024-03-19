#!/usr/bin/env bash

set -e

src_bin="$1"

# Array of paths for each of the outputs from the :generate_doc target.
# shellcheck disable=SC2124
got_files="${@:2}"

# Manually run src-cli doc again, so have a list of all the files
# we expect the :generate_doc target to output.
#
# We put them in the ./expected folder.
USER=nobody HOME=. "$src_bin" doc -o=expected/

while IFS= read -r -d '' file
do
  want="${file##expected}"
  found="false"

  # Loop over all files we got.
  # shellcheck disable=SC2068
  for got in ${got_files[@]}; do
    # Trim the path from the "monitoring/output" prefix
    # and test it against the expected file we're currently iterating with.
    if [[ "${got##doc/cli/references}" == "$want" ]]; then
      found="true"
      break
    fi
  done

  # If we didn't find it, return an error.
  if [[ $found == "false" ]]; then
    echo "Couldn't find expected output $want, perhaps it's missing from the 'srcs' attribute?"
    exit 1
  fi
done < <(find expected -name "*.md" -print0)
