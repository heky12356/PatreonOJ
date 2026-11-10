#!/usr/bin/env bash
set -euo pipefail
deploy_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
env_file=${PATREONOJ_COMPOSE_ENV:-/opt/patreonoj/shared/compose.env}
exec docker compose --env-file "$env_file" -f "$deploy_dir/compose.yaml" "$@"
