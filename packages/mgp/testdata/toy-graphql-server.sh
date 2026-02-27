#!/usr/bin/env bash
# toy-graphql-server.sh — speaks GraphQL-over-stdio
#
# Usage: mgp --graphql-server ./toy-graphql-server.sh

while IFS= read -r line; do
  if echo "$line" | grep -q '__schema'; then
    echo '{"data":{"__schema":{"queryType":{"name":"Query"}}}}'
  elif echo "$line" | grep -q 'tools'; then
    echo '{"data":{"tools":[{"name":"hello","package":"toy","description":"Says hello"}]}}'
  else
    echo '{"data":null}'
  fi
done
