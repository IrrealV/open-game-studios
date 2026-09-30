# Godot 4 setup and installation consent

This is agent guidance, not a deterministic installer or consent-enforcement tool. Approval must come from the current conversation or an explicit tool response. A prior user's development-machine permission never carries to another user, project, version, destination, or command plan.

## Readiness check

Check without changing the host or project:

1. If the user supplied an executable path, run that exact path with `--headless --version`.
2. Otherwise, resolve and try `godot --headless --version`, then `godot4 --headless --version` with host process tools rather than shell text searches.
3. Require a runnable Godot 4 version. Diagnose lookup failure, execution failure, empty output, and a non-4 major version separately; do not treat one as another.

Use the explicit executable path for later commands. Do not edit `PATH` or personal configuration. Readiness only permits approved project checks; it does not authorize gameplay edits.

## Missing or unusable Godot

Offer exactly these branches:

1. Provide an existing Godot 4 executable path.
2. Review an installation plan.
3. Cancel and preserve the project.

Before any installation, present the exact official source, version, platform, archive size and checksum, destination, commands, requested permissions, side effects, and removal plan. Then **STOP and await explicit approval of that plan**. Unanswered or declined approval means no download, extraction, configuration change, or gameplay execution.

After approval, execute only the approved commands with available host tools. If a check, download, checksum, extraction, version, or destination condition fails, do not declare readiness, choose an alternate install, retry with changed commands, elevate privileges, or reinstall a global runtime. Report the failure and bounded artifacts created, then request approval before changing the plan.

Never use `curl | sh`, `sudo`, automatic `PATH` edits, an existing or symlink destination, or an overwrite. Each requires a new, explicit plan and consent; global runtime replacement also requires separate consent.

## Verified artifact and proposed Linux/WSL x86_64 shell plan

On Linux/WSL x86_64, the official archive URL, exact size and SHA-256 below were verified, and the extracted binary reported Godot `4.7.2.stable.official.ed1daf0bf`. The development installation used Python `urllib`, `hashlib`, and `zipfile`; it did not execute the shell recipe below. The recipe is a proposed, reviewable command plan and has not been downloaded or executed end-to-end. Only its delimited destination preflight is exercised by the Node tests against temporary paths.

This platform evidence is not a validated Windows or macOS installer. For another platform, verify an official platform-specific artifact and present a new plan. Do not use GitHub's global `/releases/latest` endpoint to choose Godot 4; it has returned a 3.x release. The preflight reduces accidental symlink traversal but is not a race-proof sandbox and must not be described as one.

Plan facts to show before asking:

- Official artifact: `https://github.com/godotengine/godot-builds/releases/download/4.7.2-stable/Godot_v4.7.2-stable_linux.x86_64.zip`
- Version/platform: Godot `4.7.2-stable`, Linux/WSL x86_64
- Exact archive size: `77860424` bytes
- SHA-256: `cadd3204e728a35d3f13adb7fd0d7902636b79f6b95c40c265eb73b6c35329e4`
- Archive member: `Godot_v4.7.2-stable_linux.x86_64`
- Destination: `${HOME}/.local/share/ogs/tools/godot/4.7.2/Godot_v4.7.2-stable_linux.x86_64`
- Permissions: current user only; destination directory and executable mode `0700`; no administrator access
- Side effects: one temporary work directory/archive and one new versioned destination; no `PATH`, shell-profile, package-manager, or global-runtime changes

Exact proposed installation commands:

```sh
set -eu
url='https://github.com/godotengine/godot-builds/releases/download/4.7.2-stable/Godot_v4.7.2-stable_linux.x86_64.zip'
expected_size='77860424'
expected_sha256='cadd3204e728a35d3f13adb7fd0d7902636b79f6b95c40c265eb73b6c35329e4'
member='Godot_v4.7.2-stable_linux.x86_64'

# DESTINATION PREFLIGHT BEGIN
destination_dir="${HOME}/.local/share/ogs/tools/godot/4.7.2"
destination="${destination_dir}/Godot_v4.7.2-stable_linux.x86_64"
case "${destination_dir}" in
  /*) ;;
  *) printf '%s\n' 'destination directory must be absolute' >&2; exit 1 ;;
esac
set -f
old_ifs=${IFS}
IFS='/'
set -- ${destination_dir#/}
IFS=${old_ifs}
current_path=''
for component do
  test -n "${component}" || continue
  current_path="${current_path}/${component}"
  if test -L "${current_path}"; then
    printf 'symlink destination ancestor rejected: %s\n' "${current_path}" >&2
    exit 1
  fi
  if test -e "${current_path}" && test ! -d "${current_path}"; then
    printf 'non-directory destination ancestor rejected: %s\n' "${current_path}" >&2
    exit 1
  fi
done
if test -e "${destination}" || test -L "${destination}"; then
  printf 'final destination already exists: %s\n' "${destination}" >&2
  exit 1
fi
if test -e "${destination_dir}" || test -L "${destination_dir}"; then
  printf 'destination directory already exists: %s\n' "${destination_dir}" >&2
  exit 1
fi
# DESTINATION PREFLIGHT END

work_dir="$(mktemp -d "${TMPDIR:-/tmp}/ogs-godot-install-XXXXXX")"
archive="${work_dir}/${member}.zip"
mkdir -m 0700 -p "$(dirname "${destination_dir}")"
mkdir -m 0700 "${destination_dir}"
curl --fail --location --proto '=https' --tlsv1.2 --output "${archive}" "${url}"
test "$(stat --printf='%s' "${archive}")" = "${expected_size}"
printf '%s  %s\n' "${expected_sha256}" "${archive}" | sha256sum --check --strict -
unzip -j "${archive}" "${member}" -d "${destination_dir}"
chmod 0700 "${destination}"
sandbox="$(mktemp -d "${TMPDIR:-/tmp}/ogs-godot-check-XXXXXX")"
mkdir -m 0700 "${sandbox}/home" "${sandbox}/config" "${sandbox}/data" "${sandbox}/cache"
HOME="${sandbox}/home" XDG_CONFIG_HOME="${sandbox}/config" XDG_DATA_HOME="${sandbox}/data" XDG_CACHE_HOME="${sandbox}/cache" "${destination}" --headless --version
printf 'GODOT_BIN=%s\nWORK_DIR=%s\nCHECK_SANDBOX=%s\n' "${destination}" "${work_dir}" "${sandbox}"
```

In this proposed recipe, the destination preflight runs before creating directories, temporary download state, or network activity; it inspects each existing path component without resolving it and rejects symlink ancestors. The archive would be verified before extraction, and the binary path would be returned explicitly without changing `PATH`. Confirm required host tools (`curl`, `stat`, `sha256sum`, `unzip`, `mktemp`) before approval; missing tools require a changed plan.

Removal is a separate destructive action. First show the exact returned `WORK_DIR`, `CHECK_SANDBOX`, and versioned destination, confirm they were created by this plan, and obtain explicit removal approval. Remove only those paths; never clean sibling versions, user preferences, projects, or unrelated temporary data.

## Runtime check boundary

Use official Godot command-line forms such as `--headless --path <temporary-project-copy> --script <script>`. Headless execution can validate a selected mechanical behavior. It is not fun or visual QA, a security sandbox, export validation, or proof that the whole project is correct.
