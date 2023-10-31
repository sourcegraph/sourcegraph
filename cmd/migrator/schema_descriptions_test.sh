#!/usr/bin/env bash

# Path to the schema_descriptions tool
generate_bin="$1"

# Path to the wip_git_versions.txt
wip_git_versions="$2"

# Array of paths for each of the outputs from the :generate_config target.
# shellcheck disable=SC2124
got_files="${@:3}"

# Manually run the script again, so have a list of all the files
# we expect the :schema_descriptions target to output.
#
# We put them in the ./expected folder.
"$generate_bin" expected/ "$wip_git_versions"

# Loop over all of them and check if we can find each of them in the
# outputs from :schema_descriptions target.
for file in expected/**/*; do
  # Trim the "expected" part of the path
  want="${file##expected}"
  found="false"

  # Loop over all files we got.
  # shellcheck disable=SC2068
  for got in ${got_files[@]}; do
    # Trim the path from the "monitoring/output" prefix
    # and test it against the expected file we're currently iterating with.
    if [[ "${got##cmd/migrator}" == "$want" ]]; then
      found="true"
      break
    fi
  done

  # If we didn't find it, return an error.
  if [[ $found == "false" ]]; then
    echo "Couldn't find expected output $want, perhaps it's missing from the 'srcs' attribute?"
    exit 1
  fi
done

