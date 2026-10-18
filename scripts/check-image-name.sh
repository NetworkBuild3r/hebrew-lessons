#!/bin/sh
# Refuse an image reference whose name contains hebrew-lessons.
#
# ghcr.io/networkbuild3r/hebrew-lessons is the name the production
# auto-updater watches. Pushing this tree under that name would deploy it.
# The image for this app is ghcr.io/networkbuild3r/hebrew.
#
# Usage: scripts/check-image-name.sh IMAGE [IMAGE...]
# Exit 0 when every argument is allowed, 1 when one is not, 2 when none were given.
set -eu

if [ "$#" -eq 0 ]; then
	echo "usage: scripts/check-image-name.sh IMAGE [IMAGE...]" >&2
	exit 2
fi

for name in "$@"; do
	case "$name" in
	*hebrew-lessons*)
		echo "refusing image ${name}: the name contains hebrew-lessons" >&2
		exit 1
		;;
	esac
done
