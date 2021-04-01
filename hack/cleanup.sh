#!/toolchain/bin/bash

export PATH=/toolchain/bin

PREFIX="${1}"

function remove_symlinks() {
    set +e
    for l in $(find ${PREFIX} -type l); do
        readlink $l | grep -q /toolchain
        if [ $? == 0 ]; then
            unlink $l
        fi
    done
    set -e
}

# Remove any symlinks that might have been need at build time.
remove_symlinks

# Remove any archives as we do not need them since everything is dynamically linked.
find ${PREFIX} -type f -name \*.a -print0 | xargs -0 rm -rf || true
find ${PREFIX} -type f -name \*.la -print0 | xargs -0 rm -rf || true
# Remove static binaries.
find ${PREFIX} -type f -name \*.static -print0 | xargs -0 rm -rf || true
# Strip debug symbols from all libraries and binaries.
find ${PREFIX}/{lib,usr/lib} -type f \( -name \*.so* -a ! -name \*dbg \) -exec strip --strip-unneeded {} ';' || true
find ${PREFIX}/{bin,sbin,usr/bin,usr/sbin} -type f -exec strip --strip-all {} ';' || true

# Remove header files, man files, and any other non-runtime dependencies.
rm -rf ${PREFIX}/{lib,usr/lib}/pkgconfig/ \
       ${PREFIX}/{include,usr/include}/* \
       ${PREFIX}/{share,usr/share}/* \
       ${PREFIX}/lib/gconv/ \
       ${PREFIX}/usr/libexec/getconf \
       ${PREFIX}/var/db
