#!/bin/bash
# Deploys one app server over SSH. Invoked by .github/workflows/cd.yml
# (deploy-staging / deploy-production jobs) — never run directly outside
# CI. See docs/deployment-design.md for the design this implements.
#
# Required env: DEPLOY_HOST, DEPLOY_USER, DEPLOY_SSH_KEY, IMAGE_TAG,
# GH_TOKEN, GH_ACTOR. Must run from the repository root (reads
# docker-compose.prod.yml from the current checkout).
set -euo pipefail

: "${DEPLOY_HOST:?DEPLOY_HOST must be set}"
: "${DEPLOY_USER:?DEPLOY_USER must be set}"
: "${DEPLOY_SSH_KEY:?DEPLOY_SSH_KEY must be set}"
: "${IMAGE_TAG:?IMAGE_TAG must be set}"
: "${GH_TOKEN:?GH_TOKEN must be set}"
: "${GH_ACTOR:?GH_ACTOR must be set}"

workdir="$(mktemp -d)"
trap 'rm -rf "$workdir"' EXIT

key_path="$workdir/deploy_key"
printf '%s\n' "$DEPLOY_SSH_KEY" >"$key_path"
chmod 600 "$key_path"

known_hosts="$workdir/known_hosts"
ssh-keyscan -H "$DEPLOY_HOST" >"$known_hosts" 2>/dev/null

ssh_opts=(-i "$key_path" -o UserKnownHostsFile="$known_hosts" -o ConnectTimeout=10)
target="$DEPLOY_USER@$DEPLOY_HOST"

echo "==> Syncing docker-compose.prod.yml to $target:/opt/levelog/"
scp "${ssh_opts[@]}" docker-compose.prod.yml "$target:/opt/levelog/docker-compose.prod.yml"

echo "==> Deploying image tag $IMAGE_TAG on $DEPLOY_HOST"
# Unquoted heredoc: $GH_TOKEN / $GH_ACTOR / $IMAGE_TAG are expanded here,
# locally, and sent to the remote shell as literal values — there is no
# remote-side expansion happening at all here, so no local/remote
# escaping ambiguity to get wrong. GH_TOKEN is only used for this one
# login+pull, live during this SSH session; it is not written to disk
# beyond docker's own credential store, and `docker logout` below removes
# it again immediately after.
# shellcheck disable=SC2087
ssh "${ssh_opts[@]}" "$target" bash -s <<REMOTE
set -euo pipefail
echo "$GH_TOKEN" | docker login ghcr.io -u "$GH_ACTOR" --password-stdin
trap 'docker logout ghcr.io >/dev/null 2>&1 || true' EXIT
export IMAGE_TAG="$IMAGE_TAG"
/opt/levelog/deploy.sh
REMOTE

echo "==> Deploy to $DEPLOY_HOST complete"
