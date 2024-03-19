#!/usr/bin/env bash

set -ex

# Install `dot` from https://graphviz.org/download/
dot architecture.dot -Tsvg >architecture.svg
