#!/usr/bin/env bash

# This script is designed to wrap commands to run them and pick up the annotations and
# test reports they leave behind for upload.
#
# An alias for this command, './an', is set up in .buildkite/post-checkout

cmd=$1

# Set up directory for annotated command to leave annotations
annotation_dir="./annotations"
rm -rf $annotation_dir
mkdir -p $annotation_dir
test_report_dir="./test-reports"
rm -rf $test_report_dir
mkdir -p $test_report_dir

# Run the provided command
eval "$cmd"
exit_code="$?"

# Check for annotations left behind by the command
if [ -n "${ANNOTATE_OPTS-''}" ]; then
  # Parse annotation options:
  # - $1 => include_names
  # - $2... => annotate_opts, base options for the ./annotate.sh script
  # shellcheck disable=SC2086
  set -- $ANNOTATE_OPTS
  include_names=$1
  shift 1
  # shellcheck disable=SC2124
  annotate_opts="$@"

  echo "~~~ Uploading annotations"
  echo "include_names=$include_names, annotate_opts=$annotate_opts"
  for file in "$annotation_dir"/*; do
    if [ ! -f "$file" ]; then
      continue
    fi

    echo "handling $file"
    name=$(basename "$file")
    annotate_file_opts=$annotate_opts

    case "$name" in
      # Append markdown annotations as markdown, and remove the suffix from the name
      *.md) annotate_file_opts="$annotate_file_opts -m" && name="${name%.*}" ;;
    esac

    case "$name" in
      "WARN_*") annotate_file_opts="$annotate_file_opts -t warning"
      "ERROR_*") annotate_file_opts="$annotate_file_opts -t error"
      "INFO_*") annotate_file_opts="$annotate_file_opts -t info"
      "SUCCESS_*") annotate_file_opts="$annotate_file_opts -t success"
      "*") annotate_file_opts="$annotate_file_opts -t error"
    esac

    if [ "$include_names" = "true" ]; then
      # Set the name of the file as the title of this annotation section
      annotate_file_opts="-s '$name' $annotate_file_opts"
    fi

    # Generate annotation from file contents
    eval "./enterprise/dev/ci/scripts/annotate.sh $annotate_file_opts <'$file'"
  done
fi

# Check for test reports left behind by the command
if [ -n "${TEST_REPORT_OPTS-''}" ]; then
  test_report_opts="$TEST_REPORT_OPTS"

  echo "~~~ Uploading test reports"
  echo "test_report_opts=$test_report_opts"
  for file in "$test_report_dir"/*; do
    if [ ! -f "$file" ]; then
      continue
    fi

    echo "handling $file"
    eval "./enterprise/dev/ci/scripts/upload-test-report.sh $file $test_report_opts"
  done
fi

exit "$exit_code"
