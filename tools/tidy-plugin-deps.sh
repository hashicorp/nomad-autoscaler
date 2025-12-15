#!/usr/bin/env bash
# tidy-plugin-deps.sh runs 'go mod tidy' on every test plugin directory and adds
# the appropriate files to the git index. Use this when dependabot updates a
# dependency used in the root that then needs to get updated in the test
# plugins.

tidy() {
    plugin=$1
    pushd "plugins/test/$plugin"
    go mod tidy
    git add go.mod
    git add go.sum
    popd
}

tidy noop-apm
tidy noop-strategy
tidy noop-target
