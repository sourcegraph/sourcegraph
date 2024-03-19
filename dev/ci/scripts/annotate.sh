#!/usr/bin/env bash

# Convenience script for https://buildkite.com/docs/agent/v3/cli-annotate
# If you are writing a pipeline step DO NOT use this script directly - instead, use
# bk.AnnotatedCmd to get your command's annotations picked up.

cd "$(dirname "${BASH_SOURCE[0]}")/../../../.."
set -e

# TODO(burmudar: remove me this should be created by other scripts
mkdir -p ./annotations

print_usage() {
  printf "Usage:"
  printf "  echo \"your annotation\" | annotate.sh -s my-section"
  printf "  echo \"your markdown\" | annotate.sh -m -s my-section"
}

print_heading() {
    output="&bull; [View job output](#$BUILDKITE_JOB_ID)"
    printf "**%s** %s\n\n" "$BUILDKITE_LABEL" "$output"
}

if [ $# -eq 0 ]; then
  print_usage
  exit 1
fi

TYPE='error'
SECTION=''
MARKDOWN='false'
CUSTOM_CONTEXT=''

while getopts 't:s:c:m' flag; do
  case "${flag}" in
    t) TYPE="${OPTARG}" ;;
    s) SECTION="${OPTARG}" ;;
    c) CUSTOM_CONTEXT="${OPTARG}" ;;
    m) MARKDOWN='true' ;;
    *)
      print_usage
      exit 1
      ;;
  esac
done

# Set a default context that is unique per job/custom context and type combination.
CONTEXT=${CUSTOM_CONTEXT:-$BUILDKITE_JOB_ID}
CONTEXT="$CONTEXT-$TYPE"
# when the markdown is created, write the output to a file as well
TEE_FILE="./annotations/${BUILDKITE_JOB_ID}-annotation.md"

# If we are not in Buildkite, exit before doing annotations
if [[ -z "$BUILDKITE" ]]; then
  echo "Not in Buildkite, exiting"
  exit 0
fi

# Custom contexts span multiple jobs, so don't create a title - it's too complicated.
# Otherwise generate one in the context of the job.
if [[ -z "$CUSTOM_CONTEXT" ]]; then
  # We create a file to indicate that this program has already been called within a job
  # and there is no need to add a title to the annotation.
  FILE=.annotate
  LOCKFILE="$FILE.lock"

  exec 100>"$LOCKFILE" || exit 1
  flock 100 || exit 1

  if [ ! -f "$FILE" ]; then
    touch $FILE
    print_heading | tee -a "$TEE_FILE" | buildkite-agent annotate --style "$TYPE" --context "$CONTEXT" --append
  fi
fi

BODY=""
while IFS= read -r line; do
  if [ -z "$BODY" ]; then
    BODY="$line"
  else
    BODY=$(printf "%s\n%s" "$BODY" "$line")
  fi
done


if [ -n "$SECTION" ]; then
  printf "**%s**\n" "$SECTION" | tee -a "$TEE_FILE" | buildkite-agent annotate --style "$TYPE" --context "$CONTEXT" --append
fi


if [ "$MARKDOWN" = true ]; then
  printf "%s\n" "$BODY" | tee -a "$TEE_FILE" | buildkite-agent annotate --style "$TYPE" --context "$CONTEXT" --append
else
  printf "\`\`\`term\n%s\n\`\`\`\n" "$BODY" | tee -a "$TEE_FILE" | buildkite-agent annotate --style "$TYPE" --context "$CONTEXT" --append
fi
