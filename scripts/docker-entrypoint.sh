#!/usr/bin/env sh

set -e

if [ "$1" = 'nomad-autoscaler' ]; then
  shift
fi

exec nomad-autoscaler "$@"
