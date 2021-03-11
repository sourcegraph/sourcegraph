#!/bin/bash

psql -c "drop schema public cascade; create schema public;"
redis-cli -c flushall
rm -rf "$HOME/.sourcegraph-dev/data/codeinsights-db"
