#!/bin/bash

function net_getInterfaceAddress() {
  ip -4 address show "${1}" | awk '/inet / { print $2 }' | cut -d/ -f1
}

function net_getDefaultRouteAddress() {
  # Default route IP address (seems to be a good way to get host ip)
  ip -4 route get 1.1.1.1 | grep -oP 'src \K\S+'
}
