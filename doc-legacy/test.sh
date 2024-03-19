#!/usr/bin/env bash

set -e

docsite_bin="$1"

"${docsite_bin}" check
