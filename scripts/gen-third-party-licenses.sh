#!/usr/bin/env sh
# Regenerates THIRD_PARTY_LICENSES from the modules that go build links
# into the mado binary. Run it after dependency changes, from the repo root:
#
#   sh scripts/gen-third-party-licenses.sh
set -eu

out=THIRD_PARTY_LICENSES
rule=--------------------------------------------------------------------

{
  cat <<'HDR'
Third-party licenses
====================

mado's release binaries statically include the Go modules listed
below. Each module's copyright notice and license text is
reproduced in full, as required by their licenses.

This file covers the modules compiled into the binary
(go list -deps .). Regenerate it after dependency changes with
scripts/gen-third-party-licenses.sh.
HDR

  go list -deps -f '{{if and .Module (not .Standard)}}{{.Module.Path}} {{.Module.Version}} {{.Module.Dir}}{{end}}' . \
    | grep -v '^github.com/hidekingerz/mado ' \
    | sort -u -f \
    | while read -r path version dir; do
        lic=""
        for name in LICENSE LICENSE.md LICENSE.txt LICENCE COPYING; do
          if [ -f "$dir/$name" ]; then lic="$dir/$name"; break; fi
        done
        if [ -z "$lic" ]; then
          echo "no license file found for $path in $dir" >&2
          exit 1
        fi
        printf '\n%s\n%s %s\n%s\n\n' "$rule" "$path" "$version" "$rule"
        cat "$lic"
      done
} > "$out"
