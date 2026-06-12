#!/usr/bin/env bash

set -euo pipefail
set -x

export TERM="${TERM:-dumb}"

PLUGIN_NAME="wp"
PLUGIN_BINARY="sitectl-wp"
SITE_DIR_NAME="wp"
CREATE_DEFINITION="${CREATE_DEFINITION:-default}"
CREATE_ARGS="${CREATE_ARGS:-}"
SITECTL_CONTEXT="${SITECTL_CONTEXT:-integration-test}"

REPO_ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." &>/dev/null && pwd)"

if [ -n "${SITECTL_TMP_PARENT:-}" ]; then
	TMP_PARENT="${SITECTL_TMP_PARENT}"
elif [ -n "${GITHUB_WORKSPACE:-}" ]; then
	TMP_PARENT="${GITHUB_WORKSPACE}"
else
	TMP_PARENT="${HOME}/.tmp"
fi
mkdir -p "${TMP_PARENT}"
TMP_DIR="$(mktemp -d "${TMP_PARENT%/}/${PLUGIN_BINARY}-test.XXXXXX")"
SITECTL_HOME="${TMP_DIR}/home"
BIN_DIR="${TMP_DIR}/bin"
SITE_DIR="${TMP_DIR}/${SITE_DIR_NAME}"
PATH="${BIN_DIR}:${PATH}"
export PATH
mkdir -p "${SITECTL_HOME}"

remove_tmp_dir() {
	if [ ! -d "${TMP_DIR}" ]; then
		return
	fi
	chmod -R u+rwX "${TMP_DIR}" 2>/dev/null || true
	if rm -rf "${TMP_DIR}" 2>/dev/null; then
		return
	fi
	if command -v sudo >/dev/null 2>&1; then
		sudo chown -R "$(id -u):$(id -g)" "${TMP_DIR}" 2>/dev/null || true
		sudo chmod -R u+rwX "${TMP_DIR}" 2>/dev/null || true
	fi
	rm -rf "${TMP_DIR}"
}

cleanup() {
	if [ -d "${SITE_DIR}" ] && command -v sitectl >/dev/null 2>&1; then
		HOME="${SITECTL_HOME}" sitectl compose down -v --remove-orphans >/dev/null 2>&1 || true
	fi
	remove_tmp_dir
}
trap cleanup EXIT

build_plugin() {
	mkdir -p "${BIN_DIR}"
	(
		cd "${REPO_ROOT}" &&
			go build -o "${BIN_DIR}/${PLUGIN_BINARY}" .
	)
	command -v sitectl >/dev/null
	command -v "${PLUGIN_BINARY}" >/dev/null
}

create_site() {
	local target="${PLUGIN_NAME}/${CREATE_DEFINITION}"
	local extra_args=()
	if [ -n "${CREATE_ARGS}" ]; then
		read -r -a extra_args <<< "${CREATE_ARGS}"
	fi

	HOME="${SITECTL_HOME}" sitectl create "${target}" \
		--path "${SITE_DIR}" \
		--type local \
		--checkout-source template \
		--default-context \
		--setup-only \
		"${extra_args[@]}"
}

run_make_target() {
	local target="$1"
	if ! (
		cd "${SITE_DIR}" &&
			make "${target}"
	); then
		(
			cd "${SITE_DIR}" &&
				docker compose ps -a || true
		)
		exit 1
	fi
}

run_healthcheck() {
	HOME="${SITECTL_HOME}" sitectl healthcheck
}

main() {
	build_plugin
	create_site
	run_make_target init
	run_make_target up
	run_healthcheck
}

main "$@"
