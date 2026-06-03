## Why

All plugin modules under `extensions/` need a repeatable local packaging command that produces the same artifact layout expected by downstream plugin distribution tooling. Today the build command and artifact metadata are not captured in a project Makefile, so producing `plugin.wasm` plus release metadata requires manual steps for each extension plugin.

## What Changes

- Add a Makefile-driven build flow for every plugin module under `extensions/`.
- Discover extension plugins from the `extensions/<plugin>/` directory structure.
- Compile each extension plugin and place artifacts under `build/plugins/<plugin>/<version>/`.
- Generate `metadata.txt` beside each `plugin.wasm` with plugin name, size, timestamps, and MD5 fields.
- Keep the change limited to local build/package automation; no runtime plugin behavior changes are intended.

## Capabilities

### New Capabilities
- `extensions-build-artifacts`: Defines the local build output contract for producing versioned artifacts for all extension plugins.

### Modified Capabilities
- None.

## Impact

- Affected code: repository build automation, expected to be a new root-level `Makefile`.
- Affected plugin modules: all plugin directories under `extensions/`, currently `ai-billing`, `ai-quota`, and `ai-statistics`.
- Affected outputs: `build/plugins/<plugin>/<version>/plugin.wasm` and `build/plugins/<plugin>/<version>/metadata.txt`.
- APIs and runtime behavior: no Kubernetes API, Wasm plugin runtime, or configuration behavior changes.
- Dependencies: uses existing Go/TinyGo toolchain conventions already documented in the repository.
