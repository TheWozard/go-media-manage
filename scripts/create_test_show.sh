#!/usr/bin/env bash
set -euo pipefail

usage() {
    echo "Usage: $0 \"Show Name (Year)\" <episodes_s1> [episodes_s2] ..."
    echo "Example: $0 \"Breaking Bad (2008)\" 7 13 13 13 16"
    exit 1
}

[[ $# -lt 2 ]] && usage

show_name="$1"
shift
season_counts=("$@")

root="./tmp/${show_name}"

mkdir -p ./tmp

if [[ -d "$root" ]]; then
    echo "Removing existing: $root"
    rm -rf "$root"
fi

echo "Creating: $root"

for i in "${!season_counts[@]}"; do
    season_num=$(( i + 1 ))
    episode_count="${season_counts[$i]}"
    season_dir=$(printf "%s/Season %02d" "$root" "$season_num")

    mkdir -p "$season_dir"

    for ep in $(seq 1 "$episode_count"); do
        filename="${ep}.mkv"
        touch "${season_dir}/${filename}"
    done

    echo "  Season $(printf '%02d' $season_num): $episode_count episode(s)"
done

echo "Done."
