#!/usr/bin/env sh
# Copyright IBM Corp. 2020, 2025
# SPDX-License-Identifier: MPL-2.0


set -e

if [ "$1" = 'nomad-autoscaler' ]; then
  shift
fi

exec nomad-autoscaler "$@"
