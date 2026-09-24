#!/usr/bin/env bash
#
# Brings a Logwolf MongoDB data directory forward to the version
# docker-compose.yml runs, one major version at a time.
#
# Logwolf ran mongo:4.2 before, and mongod only opens data files written by the
# previous major version whose featureCompatibilityVersion (FCV) has been
# raised. So a 4.2 data directory has to go through 4.4, 5.0, 6.0 and 7.0
# before 8.0 will start on it. This does that: for each version it starts a
# throwaway container on the data directory, sets the FCV to that version, and
# stops it cleanly.
#
# It is safe to run more than once. A version that cannot open the data,
# because the data is already newer, is skipped, so a directory that is already
# on 8.0 goes through one no-op step.
#
# Usage, from logwolf-server/, with the stack stopped:
#
#   docker compose down
#   cp -a db-data/mongo db-data/mongo.bak     # a backup, in case
#   scripts/upgrade-mongo.sh                  # or: scripts/upgrade-mongo.sh path/to/data
#   docker compose up -d
#
# The containers get no network and no published port, and run without access
# control, which is why nothing else may be using the directory. Users and
# passwords stored in the data are kept.

set -euo pipefail

# Git Bash on Windows would otherwise rewrite the container paths below
# (/data/db) into Windows ones. Elsewhere it means nothing.
export MSYS_NO_PATHCONV=1

VERSIONS=("4.4" "5.0" "6.0" "7.0" "8.0.32")
CONTAINER="logwolf-mongo-upgrade"
READY_TIMEOUT=180

data_dir=${1:-"$(dirname "$0")/../db-data/mongo"}
if [ ! -d "$data_dir" ]; then
	echo "No data directory at $data_dir" >&2
	exit 1
fi
# pwd -W gives Git Bash on Windows a path Docker Desktop can mount.
data_dir=$(cd "$data_dir" && (pwd -W 2>/dev/null || pwd))

if docker ps --format '{{.Image}}' | grep -q '^mongo:'; then
	echo "A mongo container is running. Stop the stack first: docker compose down" >&2
	exit 1
fi

cleanup() { docker rm -f "$CONTAINER" >/dev/null 2>&1 || true; }
trap cleanup EXIT

fail() {
	echo "$1" >&2
	exit 1
}

# mongo_eval runs JavaScript in the upgrade container with whichever shell its
# image ships: the legacy mongo shell up to 5.0, mongosh from 6.0.
mongo_eval() {
	docker exec "$CONTAINER" sh -c '
		shell=$(command -v mongosh || command -v mongo)
		exec "$shell" --quiet --eval "$1"
	' sh "$1"
}

# start_mongo starts version $1 on the data directory and waits until it is the
# replica set's writable primary. It returns non-zero if mongod exits instead,
# which is what it does on data it cannot open.
start_mongo() {
	cleanup
	# The replica set config names its member mongo:27017, as docker-compose.yml's
	# healthcheck registered it. The node has to recognize that name as itself,
	# or it finds itself missing from its own set and never becomes primary; with
	# no network, "mongo" resolves to nothing unless pointed at loopback.
	#
	# Docker failing is fatal here, not a skip: only mongod refusing the data is.
	# (Inside `if ! start_mongo`, set -e does not apply.)
	docker image inspect "mongo:$1" >/dev/null 2>&1 ||
		docker pull -q "mongo:$1" >/dev/null || fail "could not pull mongo:$1"
	docker run -d --name "$CONTAINER" --hostname mongo --add-host mongo:127.0.0.1 --network none \
		-v "$data_dir:/data/db" "mongo:$1" --replSet rs0 --bind_ip_all >/dev/null ||
		fail "could not start mongo:$1"

	# A data directory that was never part of a replica set is initiated, the
	# way the compose healthcheck would.
	local init='
		var s; try { s = rs.status() } catch (e) { s = e }
		if (s.codeName === "NotYetInitialized") {
			rs.initiate({ _id: "rs0", members: [{ _id: 0, host: "mongo:27017" }] })
		}
		print(db.hello ? db.hello().isWritablePrimary : db.isMaster().ismaster)'

	local waited=0
	while [ "$waited" -lt "$READY_TIMEOUT" ]; do
		if [ "$(docker inspect -f '{{.State.Running}}' "$CONTAINER")" != "true" ]; then
			return 1
		fi
		if [ "$(mongo_eval "$init" 2>/dev/null | tail -n 1)" = "true" ]; then
			return 0
		fi
		sleep 2
		waited=$((waited + 2))
	done
	echo "mongo:$1 did not become primary within ${READY_TIMEOUT}s" >&2
	docker logs --tail 20 "$CONTAINER" >&2
	exit 1
}

upgraded=false
first_refusal=""
for version in "${VERSIONS[@]}"; do
	major=${version%.*}
	[[ "$version" == *.*.* ]] || major=$version
	echo "== mongo:$version"

	if ! start_mongo "$version"; then
		if $upgraded; then
			echo "mongo:$version could not open the data mongo:$previous left behind:" >&2
			docker logs --tail 20 "$CONTAINER" >&2
			exit 1
		fi
		# Kept in case no version opens the data: then the oldest one's reason is
		# the one that matters.
		[ -n "$first_refusal" ] || first_refusal=$(docker logs --tail 20 "$CONTAINER" 2>&1)
		echo "   mongod exited on this data, which must be newer; skipping"
		continue
	fi

	fcv=$(mongo_eval 'print(db.adminCommand({ getParameter: 1, featureCompatibilityVersion: 1 }).featureCompatibilityVersion.version)' | tail -n 1)
	echo "   featureCompatibilityVersion $fcv -> $major"

	# 7.0 and later refuse the change without confirm, and earlier versions
	# refuse the unknown field.
	confirm=""
	if [ "${major%%.*}" -ge 7 ]; then confirm=", confirm: true"; fi
	result=$(mongo_eval "printjson(db.adminCommand({ setFeatureCompatibilityVersion: '$major'$confirm }).ok)" | tail -n 1)
	if [ "$result" != "1" ]; then
		echo "setFeatureCompatibilityVersion $major failed" >&2
		exit 1
	fi

	# SIGTERM makes mongod shut down cleanly; give it time to.
	docker stop -t 60 "$CONTAINER" >/dev/null
	upgraded=true
	previous=$version
done

if ! $upgraded; then
	echo "No MongoDB version could open $data_dir. mongo:${VERSIONS[0]} said:" >&2
	echo "$first_refusal" >&2
	exit 1
fi
echo "Done: $data_dir is on MongoDB ${VERSIONS[-1]}."
