#!/usr/bin/env bash
set -euo pipefail
umask 077

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
project_root=$(cd "$script_dir/.." && pwd)
output_dir="$project_root/deploy/generated"

usage() {
  cat <<'EOF'
usage: scripts/release-metadata.sh [--output-dir <directory>]

Generates release.env and release-notes.txt from Git metadata. Optional
WELL_AMBIENT_VERSION, WELL_AMBIENT_BUILD_TIME, WELL_AMBIENT_COMMIT,
WELL_AMBIENT_RELEASE_BATCH and WELL_AMBIENT_WORKTREE_DIRTY values override
their generated counterparts.
EOF
}

while (($# > 0)); do
  case "$1" in
    --output-dir)
      if (($# < 2)); then
        echo "--output-dir requires a directory" >&2
        exit 2
      fi
      output_dir=$2
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [[ "$output_dir" != /* ]]; then
  output_dir="$project_root/$output_dir"
fi
mkdir -p "$output_dir"

git_available=false
if command -v git >/dev/null 2>&1 && git -C "$project_root" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  git_available=true
fi

format_epoch_utc() {
  local epoch=$1
  if date -u -r "$epoch" +%Y-%m-%dT%H:%M:%SZ >/dev/null 2>&1; then
    date -u -r "$epoch" +%Y-%m-%dT%H:%M:%SZ
  else
    date -u -d "@$epoch" +%Y-%m-%dT%H:%M:%SZ
  fi
}

sanitize_tag() {
  printf '%s' "$1" |
    sed -E 's/[^A-Za-z0-9._-]+/-/g; s/^[^A-Za-z0-9]+//; s/[-._]+$//'
}

single_line() {
  printf '%s' "$1" | tr '\r\n\t' '   ' | sed -E 's/[[:space:]]+/ /g; s/^ //; s/ $//'
}

build_time=${WELL_AMBIENT_BUILD_TIME:-${BUILD_TIME:-}}
if [[ -z "$build_time" ]]; then
  if [[ -n "${SOURCE_DATE_EPOCH:-}" ]]; then
    if [[ ! "$SOURCE_DATE_EPOCH" =~ ^[0-9]+$ ]]; then
      echo "SOURCE_DATE_EPOCH must be a non-negative integer" >&2
      exit 2
    fi
    build_time=$(format_epoch_utc "$SOURCE_DATE_EPOCH")
  else
    build_time=$(date -u +%Y-%m-%dT%H:%M:%SZ)
  fi
fi
if [[ ! "$build_time" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$ ]]; then
  echo "WELL_AMBIENT_BUILD_TIME must use UTC YYYY-MM-DDTHH:MM:SSZ" >&2
  exit 2
fi

commit=${WELL_AMBIENT_COMMIT:-${COMMIT:-}}
if [[ -z "$commit" && "$git_available" == true ]]; then
  commit=$(git -C "$project_root" rev-parse HEAD)
fi
commit=${commit:-unknown}
short_commit=${commit:0:8}
if [[ "$short_commit" == "unknown" ]]; then
  short_commit=local
fi

worktree_dirty=${WELL_AMBIENT_WORKTREE_DIRTY:-}
if [[ -z "$worktree_dirty" ]]; then
  worktree_dirty=false
  if [[ "$git_available" == true ]] && [[ -n "$(git -C "$project_root" status --porcelain --untracked-files=normal)" ]]; then
    worktree_dirty=true
  fi
fi
if [[ "$worktree_dirty" != true && "$worktree_dirty" != false ]]; then
  echo "WELL_AMBIENT_WORKTREE_DIRTY must be true or false" >&2
  exit 2
fi

explicit_version=${WELL_AMBIENT_VERSION:-${VERSION:-}}
version=$explicit_version
if [[ -z "$version" ]]; then
  exact_tag=""
  commit_date=${build_time:0:10}
  if [[ "$git_available" == true ]]; then
    exact_tag=$(git -C "$project_root" describe --tags --exact-match HEAD 2>/dev/null || true)
    git_commit_date=$(git -C "$project_root" show -s --format=%cI HEAD 2>/dev/null || true)
    if [[ -n "$git_commit_date" ]]; then
      commit_date=${git_commit_date:0:10}
    fi
  fi
  if [[ -n "$exact_tag" ]]; then
    version=$(sanitize_tag "$exact_tag")
  else
    version="${commit_date//-/.}-$short_commit"
  fi
  if [[ "$worktree_dirty" == true ]]; then
    dirty_stamp=$(printf '%s' "$build_time" | tr -d -- '-:TZ')
    version="$version-dirty.$dirty_stamp"
  fi
fi
version=$(sanitize_tag "$version")
if [[ -z "$version" || "$version" == latest || ${#version} -gt 128 || ! "$version" =~ ^[A-Za-z0-9][A-Za-z0-9._-]*$ ]]; then
  echo "generated release version is not a valid immutable Docker tag: $version" >&2
  exit 2
fi

latest_tag=""
commit_count=0
log_range=""
if [[ "$git_available" == true ]]; then
  latest_tag=$(git -C "$project_root" describe --tags --abbrev=0 HEAD 2>/dev/null || true)
  if [[ -n "$latest_tag" ]]; then
    log_range="$latest_tag..HEAD"
    commit_count=$(git -C "$project_root" rev-list --count "$log_range")
  else
    commit_count=$(git -C "$project_root" rev-list --count HEAD)
    if ((commit_count > 20)); then
      commit_count=20
    fi
  fi
fi

release_batch=${WELL_AMBIENT_RELEASE_BATCH:-${RELEASE_BATCH_CONTENT:-}}
release_batch=$(single_line "$release_batch")
if [[ -z "$release_batch" ]]; then
  if [[ -n "$latest_tag" && $commit_count -eq 0 ]]; then
    release_batch="Git tag $latest_tag"
  elif [[ -n "$latest_tag" ]]; then
    release_batch="$commit_count commits since $latest_tag"
  elif [[ "$git_available" == true ]]; then
    release_batch="latest $commit_count Git commits"
  else
    release_batch="automated local build"
  fi
fi

env_tmp=$(mktemp "$output_dir/.release.env.XXXXXX")
notes_tmp=$(mktemp "$output_dir/.release-notes.XXXXXX")
cleanup() {
  rm -f "$env_tmp" "$notes_tmp"
}
trap cleanup EXIT

{
  printf '# Generated by scripts/release-metadata.sh; do not edit.\n'
  printf 'WELL_AMBIENT_VERSION=%q\n' "$version"
  printf 'WELL_AMBIENT_COMMIT=%q\n' "$commit"
  printf 'WELL_AMBIENT_BUILD_TIME=%q\n' "$build_time"
  printf 'WELL_AMBIENT_RELEASE_BATCH=%q\n' "$release_batch"
  printf 'WELL_AMBIENT_WORKTREE_DIRTY=%q\n' "$worktree_dirty"
} >"$env_tmp"

{
  printf 'Well Ambient release\n'
  printf 'Version: %s\n' "$version"
  printf 'Commit: %s\n' "$commit"
  printf 'Build time: %s\n' "$build_time"
  printf 'Worktree dirty: %s\n' "$worktree_dirty"
  printf 'Batch: %s\n\n' "$release_batch"
  printf 'Changes:\n'
  if [[ -n "${WELL_AMBIENT_RELEASE_BATCH:-${RELEASE_BATCH_CONTENT:-}}" ]]; then
    printf -- '- %s\n' "$release_batch"
  elif [[ "$git_available" == true && -n "$log_range" && $commit_count -gt 0 ]]; then
    git -C "$project_root" log --no-merges --pretty=format:'- %h %s' "$log_range"
    printf '\n'
  elif [[ "$git_available" == true && -z "$latest_tag" && $commit_count -gt 0 ]]; then
    git -C "$project_root" log --no-merges -n "$commit_count" --pretty=format:'- %h %s'
    printf '\n'
  elif [[ -n "$latest_tag" ]]; then
    printf -- '- Exact release tag %s\n' "$latest_tag"
  else
    printf -- '- No Git change history was available.\n'
  fi
  if [[ "$worktree_dirty" == true ]]; then
    printf '\nUncommitted worktree changes:\n'
    if [[ "$git_available" == true ]]; then
      git -C "$project_root" status --short --untracked-files=normal | sed -n '1,100p'
    else
      printf -- '- Present; file list unavailable.\n'
    fi
  fi
} >"$notes_tmp"

chmod 0644 "$env_tmp" "$notes_tmp"
mv "$env_tmp" "$output_dir/release.env"
mv "$notes_tmp" "$output_dir/release-notes.txt"
trap - EXIT

printf 'release version: %s\n' "$version"
printf 'build time: %s\n' "$build_time"
printf 'batch: %s\n' "$release_batch"
printf 'metadata: %s\n' "$output_dir/release.env"
printf 'release notes: %s\n' "$output_dir/release-notes.txt"
