#!/bin/bash

# parsebool.sh exits with code:
#   0 if passed argument is false, FALSE, f, 0, etc
#   1 if passed argument is true, TRUE, t, 1, etc
#   2 if passed argument is absent, an empty string or something else

set -e

arg=$(echo $* | tr '[:upper:]' '[:lower:]')

case $arg in
  false|f|0) exit 0 ;;
  true|t|1) exit 1 ;;
  *) exit 2 ;;
esac
