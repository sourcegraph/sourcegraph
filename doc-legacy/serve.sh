#!/usr/bin/env bash

docsite_bin="$1"

"$docsite_bin" -config doc-legacy/docsite.json serve -http=localhost:5080
