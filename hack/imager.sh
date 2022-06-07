# helper scripts for running imager functions

prepare_extension_images() {
    # first argument - image platform, e.g. linux/amd64
    # other arguments - list of system extension images
    local platform="$1"
    shift

	local extensions_dir=$(mktemp -d)

	for ext_image in "$@"; do
        echo "Extracting ${ext_image}..." >&2
		local ext_dir="${extensions_dir}/$(basename `mktemp -u`)"
		mkdir -p "${ext_dir}" && \
			crane export --platform="${platform}" "${ext_image}" - | tar x -C "${ext_dir}"
	done

    echo "${extensions_dir}"
}
