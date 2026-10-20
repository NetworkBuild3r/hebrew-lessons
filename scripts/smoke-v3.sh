#!/bin/sh
# Smoke test for a v3 deployment (INIT-015/SPEC-025). POSIX sh, needs curl.
#
# It reads everything from the environment and prints PASS, FAIL, or SKIP for each check.
# The exit status is 1 if any check failed, and 0 otherwise.
#
#   BASE_URL           required. The web origin, for example https://hebrew-next.ibhacked.us
#   MCP_URL            optional. The MCP endpoint, for example https://hebrew-mcp.ibhacked.us/mcp
#                      Without it the MCP checks are skipped.
#   MCP_TOKEN          optional. A bearer token for MCP: the admin token, or the read-only
#                      smoke token (MCP_READONLY_TOKEN). Without it the signed-in MCP checks
#                      are skipped. It is never printed and never put on a command line.
#   SMOKE_LEARNER_ID   optional. The learner id for get_learner_model. An admin or static
#                      token must set it. A learner or read-only token must not.
#   EXPECT_COMMIT      optional. A full or short git sha. /version must report a commit that
#                      starts with it.
#   SMOKE_STRICT       optional. Set to 1 and a skipped check counts as a failure.
#
# What it does not do: sign in, load Today, review a card, or start a placement. Those need a
# browser session and are listed as manual steps in docs/ops/v3-cutover.md.
#
# The script never prints a response body, only a status code or a fixed phrase, and it never
# runs with tracing on. The token goes to curl on standard input, not in the argument list.

set -u

PASS=0
FAIL=0
SKIP=0

pass() { PASS=$((PASS + 1)); printf 'PASS  %s\n' "$1"; }
fail() { FAIL=$((FAIL + 1)); printf 'FAIL  %s\n' "$1"; }
skip() {
	SKIP=$((SKIP + 1))
	if [ "${SMOKE_STRICT:-}" = "1" ]; then
		FAIL=$((FAIL + 1))
		printf 'FAIL  %s (skipped, and SMOKE_STRICT=1)\n' "$1"
	else
		printf 'SKIP  %s\n' "$1"
	fi
}

if ! command -v curl >/dev/null 2>&1; then
	echo "smoke-v3: curl is required" >&2
	exit 2
fi
if [ -z "${BASE_URL:-}" ]; then
	echo "smoke-v3: BASE_URL is required" >&2
	exit 2
fi
BASE_URL=${BASE_URL%/}

WORK=$(mktemp -d "${TMPDIR:-/tmp}/smoke-v3.XXXXXX") || exit 2
trap 'rm -rf "$WORK"' EXIT INT TERM
BODY="$WORK/body"
HDRS="$WORK/headers"

# A token with a quote, a backslash, or a line break would break the curl config line below,
# and it is not a bearer token anyway. Say so without printing it.
TOKEN_OK=0
if [ -n "${MCP_TOKEN:-}" ]; then
	case $MCP_TOKEN in
		*[!A-Za-z0-9._~+/=-]*) TOKEN_OK=0 ;;
		*) TOKEN_OK=1 ;;
	esac
fi

COMMON="--silent --show-error --max-time 20 --connect-timeout 5"

# get URL: GET without following redirects. Sets STATUS. Body and headers go to files.
get() {
	: >"$BODY"
	: >"$HDRS"
	# shellcheck disable=SC2086
	STATUS=$(curl $COMMON -o "$BODY" -D "$HDRS" -w '%{http_code}' "$1" 2>/dev/null) || STATUS=000
}

# mcp_post URL JSON [auth]: POST a JSON-RPC message. With auth set to 1 the bearer token is
# sent from standard input through curl --config, so it is not in the argument list.
mcp_post() {
	: >"$BODY"
	: >"$HDRS"
	if [ "${3:-0}" = "1" ]; then
		# shellcheck disable=SC2086
		STATUS=$(printf 'header = "Authorization: Bearer %s"\n' "$MCP_TOKEN" |
			curl $COMMON --config - -o "$BODY" -D "$HDRS" -w '%{http_code}' \
				-X POST -H 'Content-Type: application/json' -H 'Accept: application/json, text/event-stream' \
				--data "$2" "$1" 2>/dev/null) || STATUS=000
	else
		# shellcheck disable=SC2086
		STATUS=$(curl $COMMON -o "$BODY" -D "$HDRS" -w '%{http_code}' \
			-X POST -H 'Content-Type: application/json' -H 'Accept: application/json, text/event-stream' \
			--data "$2" "$1" 2>/dev/null) || STATUS=000
	fi
}

body_has() { grep -Eq -- "$1" "$BODY"; }

echo "smoke-v3: base $BASE_URL"

# 1. /healthz
get "$BASE_URL/healthz"
if [ "$STATUS" = "200" ]; then
	pass "healthz answers 200"
else
	fail "healthz answered $STATUS, want 200"
fi

# 2. /version
get "$BASE_URL/version"
if [ "$STATUS" != "200" ]; then
	fail "version answered $STATUS, want 200"
else
	COMMIT=$(sed -n 's/.*"commit"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$BODY" | head -n 1)
	if [ -z "$COMMIT" ] || [ "$COMMIT" = "unknown" ]; then
		fail "version has no build commit (the image was built without GIT_SHA)"
	elif [ -n "${EXPECT_COMMIT:-}" ]; then
		case $COMMIT in
			"$EXPECT_COMMIT"*) pass "version commit $COMMIT matches EXPECT_COMMIT" ;;
			*) fail "version commit is $COMMIT, EXPECT_COMMIT is $EXPECT_COMMIT" ;;
		esac
	else
		pass "version reports commit $COMMIT"
	fi
fi

# 3. An anonymous visitor is sent to sign-in and sees no learner page.
get "$BASE_URL/"
case $STATUS in
	301 | 302 | 303 | 307 | 308)
		if grep -i '^location:' "$HDRS" | grep -Eqi 'sign-?in|login'; then
			pass "anonymous / redirects to sign-in ($STATUS)"
		else
			fail "anonymous / redirects ($STATUS), but not to sign-in"
		fi
		;;
	200)
		if grep -Eqi 'sign[ -]?in' "$BODY"; then
			pass "anonymous / shows the sign-in page (200)"
		else
			fail "anonymous / answered 200 with no sign-in page"
		fi
		;;
	*) fail "anonymous / answered $STATUS, want a redirect to sign-in" ;;
esac

# 4. MCP.
INIT='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"smoke-v3","version":"1"}}}'
LIST='{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}'
if [ -n "${SMOKE_LEARNER_ID:-}" ]; then
	case $SMOKE_LEARNER_ID in
		*[!0-9]* | "") SMOKE_LEARNER_ID="" ;;
	esac
fi
if [ -n "${SMOKE_LEARNER_ID:-}" ]; then
	CALL='{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_learner_model","arguments":{"learnerId":'$SMOKE_LEARNER_ID',"limit":1}}}'
else
	CALL='{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_learner_model","arguments":{"limit":1}}}'
fi

if [ -z "${MCP_URL:-}" ]; then
	skip "mcp: MCP_URL is not set"
else
	# 4a. The endpoint refuses a caller with no token.
	mcp_post "$MCP_URL" "$INIT" 0
	if [ "$STATUS" = "401" ]; then
		pass "mcp refuses an anonymous call (401)"
	else
		fail "mcp answered an anonymous call with $STATUS, want 401"
	fi

	if [ -z "${MCP_TOKEN:-}" ]; then
		skip "mcp initialize, tools/list, get_learner_model: MCP_TOKEN is not set"
	elif [ "$TOKEN_OK" != "1" ]; then
		fail "mcp: MCP_TOKEN has characters a bearer token does not (not sent)"
	else
		# 4b. initialize
		mcp_post "$MCP_URL" "$INIT" 1
		if [ "$STATUS" = "200" ] && body_has '"serverInfo"' && body_has 'hebrew-lessons'; then
			pass "mcp initialize (200, server hebrew-lessons)"
		else
			fail "mcp initialize answered $STATUS, or the reply is not a hebrew-lessons server"
		fi

		# 4c. tools/list
		mcp_post "$MCP_URL" "$LIST" 1
		if [ "$STATUS" = "200" ] && body_has '"tools"' && body_has 'get_learner_model'; then
			pass "mcp tools/list includes get_learner_model"
		else
			fail "mcp tools/list answered $STATUS, or get_learner_model is not listed"
		fi

		# 4d. get_learner_model
		mcp_post "$MCP_URL" "$CALL" 1
		if [ "$STATUS" != "200" ]; then
			fail "mcp get_learner_model answered $STATUS, want 200"
		elif body_has '"isError"[[:space:]]*:[[:space:]]*true'; then
			fail "mcp get_learner_model returned a tool error (an admin or static token needs SMOKE_LEARNER_ID, set to a learner that exists)"
		elif body_has '\\?"words\\?"'; then
			pass "mcp get_learner_model returned a page of words"
		else
			fail "mcp get_learner_model reply has no words field"
		fi
	fi
fi

printf 'smoke-v3: %d passed, %d failed, %d skipped\n' "$PASS" "$FAIL" "$SKIP"
if [ "$FAIL" -gt 0 ]; then
	exit 1
fi
exit 0
