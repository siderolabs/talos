#!/bin/bash
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
find ${PREFIX} -type f -name \*.a -delete
find ${PREFIX} -type f -name \*.la -delete
# Remove static binaries.
find ${PREFIX} -type f \( -name \*.static -o -name \*.o \) -delete
# Strip debug symbols from all libraries and binaries.
# find ${PREFIX}/{lib,usr/lib} -type f \( -name \*.so* -a ! -name \*dbg \) -exec strip --strip-unneeded {} ';' || true
# find ${PREFIX}/usr/bin -type f -exec strip --strip-all {} ';' || true

# Remove header files, man files, and any other non-runtime dependencies.
rm -rf ${PREFIX}/usr/lib/pkgconfig/ \
       ${PREFIX}/{include,usr/include}/* \
       ${PREFIX}/share/* \
       ${PREFIX}/usr/lib/cmake \
       ${PREFIX}/usr/lib/gconv/ \
       ${PREFIX}/usr/libexec/getconf \
       ${PREFIX}/var/db

# Remove contents of /usr/bin except for udevadm
# TODO: do not install these files in the first place.
# find ${PREFIX}/usr/bin \( -type f -o -type l \) ! -name udevadm -delete
