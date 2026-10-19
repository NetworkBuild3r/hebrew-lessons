#!/bin/sh
# Refuse an image name that is not the published Hebrew Lessons tag.
# The only accepted form is ghcr.io/networkbuild3r/hebrew:sha-<40 lowercase hex>.
# A name containing hebrew-lessons is called out on its own.
# CI passes the full tag (ghcr.io/networkbuild3r/hebrew:sha-<sha>), so the
# bare repository name is not accepted.
set -eu

usage() {
	echo "usage: check-image-name.sh <image> | --self-check" >&2
	exit 2
}

# allow returns 0 when name is the one published tag.
allow() {
	printf '%s' "$1" | grep -Eq '^ghcr\.io/networkbuild3r/hebrew:sha-[0-9a-f]{40}$'
}

check() {
	name=$1
	flat=$(printf '%s' "$name" | tr -d '\012\015')
	if [ "$flat" != "$name" ]; then
		echo "refusing image ${name}: name contains a newline" >&2
		return 1
	fi
	case "$name" in
	*hebrew-lessons*)
		echo "refusing image ${name}: the name contains hebrew-lessons" >&2
		return 1
		;;
	esac
	if ! allow "$name"; then
		echo "refusing image ${name}: name is not ghcr.io/networkbuild3r/hebrew:sha-<40 hex>" >&2
		return 1
	fi
	return 0
}

self_check() {
	good=ghcr.io/networkbuild3r/hebrew:sha-0123456789abcdef0123456789abcdef01234567
	bad_old=ghcr.io/networkbuild3r/hebrew-lessons:sha-0123456789abcdef0123456789abcdef01234567
	bare=ghcr.io/networkbuild3r/hebrew
	short=ghcr.io/networkbuild3r/hebrew:sha-01234567
	other=ghcr.io/example/hebrew:sha-0123456789abcdef0123456789abcdef01234567

	if ! check "$good"; then
		echo "self-check: valid tag was refused" >&2
		exit 1
	fi
	err=$(check "$bad_old" 2>&1 || true)
	case "$err" in
	*hebrew-lessons*) ;;
	*)
		echo "self-check: hebrew-lessons message missing: $err" >&2
		exit 1
		;;
	esac
	multiline=$(printf '%s\nignored' "$good")
	for name in "$bare" "$short" "$other" "$multiline"; do
		if check "$name" >/dev/null 2>&1; then
			echo "self-check: accepted $name" >&2
			exit 1
		fi
	done
	echo "self-check ok"
}

if [ "${1:-}" = "--self-check" ]; then
	self_check
	exit 0
fi

if [ "$#" -ne 1 ] || [ -z "$1" ]; then
	usage
fi

check "$1"
