#!/bin/bash
set -eufx

if [ $# -ne 4 ]; then
  printf 'Usage: %s <root_dir> <output_image> <file_contexts> <compression_level>\n' "${0##*/}"
  exit 2
fi

root_dir=$1;shift
output_image=$1;shift
file_contexts=$1;shift
compression_level=$1;shift

if [ -n "${file_contexts:-}" ]; then
  # set SELinux labels for files according to file_contexts supplied
  setfiles -r "${root_dir}" -F -vv "${file_contexts}" "${root_dir}"
fi

mksquashfs "${root_dir}" "${output_image}" \
  -all-root -noappend \
  -comp zstd -Xcompression-level "${compression_level}" \
  -no-progress
