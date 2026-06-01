#!/bin/bash
PREFIX="${1}"

# Remove any archives as we do not need them since everything is dynamically linked.
find ${PREFIX} -type f -name \*.a -delete
find ${PREFIX} -type f -name \*.la -delete
# Remove static binaries.
find ${PREFIX} -type f \( -name \*.static -o -name \*.o \) -delete
# Strip debug symbols from all libraries and binaries.
find ${PREFIX}/{lib,usr/lib} -type f \( -name \*.so* -a ! -name \*dbg \) -exec strip --strip-unneeded {} ';' || true
find ${PREFIX}/usr/bin -type f -exec strip --strip-all {} ';' || true

# Remove header files, man files, and any other non-runtime dependencies.
rm -rf ${PREFIX}/usr/lib/pkgconfig/ \
       ${PREFIX}/{include,usr/include}/* \
       ${PREFIX}/share/* \
       ${PREFIX}/usr/lib/cmake \
       ${PREFIX}/usr/lib/gconv/ \
       ${PREFIX}/usr/libexec/getconf \
       ${PREFIX}/var/db

# Drop broken symlinks.
find ${PREFIX} -xtype l -print -delete
