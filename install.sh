#!/bin/sh

readonly base=github.com/creachadair/notifier

cd "$(dirname $0)"
find . -type d -maxdepth 1 -not -name '.*' -print \
    | sed "s@^\.@$base@" \
    | xargs -n1 -t go install
