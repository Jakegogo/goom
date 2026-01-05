#!/usr/bin/env bash
set -euo pipefail

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "error: missing required command: $1" >&2
    exit 1
  }
}

require_cmd git

repo_root="$(git rev-parse --show-toplevel 2>/dev/null || true)"
if [[ -z "${repo_root}" ]]; then
  echo "error: not a git repository" >&2
  exit 1
fi
cd "${repo_root}"

if ! git diff --quiet || ! git diff --cached --quiet; then
  echo "error: working tree is not clean. Please commit or stash changes first." >&2
  exit 1
fi

if ! git rev-parse --abbrev-ref HEAD >/dev/null 2>&1; then
  echo "error: unable to determine current branch" >&2
  exit 1
fi

echo "请输入发布信息："
read -r -p "版本号 (例如 v1.0.5): " version
version="${version//[[:space:]]/}"
if [[ -z "${version}" ]]; then
  echo "error: version is empty" >&2
  exit 1
fi
if [[ ! "${version}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+([-.][0-9A-Za-z.]+)?$ ]]; then
  echo "error: invalid version format: ${version} (expected like v1.0.5)" >&2
  exit 1
fi

if git rev-parse -q --verify "refs/tags/${version}" >/dev/null; then
  echo "error: tag already exists: ${version}" >&2
  exit 1
fi

read -r -p "变更内容 (输入后回车；多行请输入，结束请输入一行空行): " first_line || true
changes=""
if [[ -n "${first_line}" ]]; then
  changes="${first_line}"$'\n'
fi
while true; do
  read -r line || break
  [[ -z "${line}" ]] && break
  changes+="${line}"$'\n'
done
changes="$(printf "%s" "${changes}" | sed -e '${/^[[:space:]]*$/d;}')"
if [[ -z "${changes}" ]]; then
  echo "error: changes is empty" >&2
  exit 1
fi

tag_msg=$'Release '"${version}"$'\n\n'"${changes}"

echo
echo "将创建并推送 tag:"
echo "- version: ${version}"
echo "- changes:"
echo "${changes}" | sed 's/^/  /'
echo

git tag -a "${version}" -m "${tag_msg}"

remote="${REMOTE:-origin}"
if ! git remote get-url "${remote}" >/dev/null 2>&1; then
  echo "error: git remote not found: ${remote}" >&2
  echo "hint: set REMOTE=<remote> or add remote '${remote}'" >&2
  exit 1
fi

git push "${remote}" "${version}"
echo "done: pushed tag ${version} to ${remote}"


