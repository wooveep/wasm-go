## Context

The repository contains multiple extension plugin modules under `extensions/`, currently `ai-billing`, `ai-quota`, and `ai-statistics`. Each module has its own `go.mod`, `main.go`, `plugin.yaml`, and `VERSION` file, while the repository documentation already shows local TinyGo and Go Wasm build commands.

The requested packaging layout is `build/plugins/<plugin>/<version>/` with two files per plugin: `plugin.wasm` and `metadata.txt`. This change should turn the manual build command into a repeatable root-level Makefile flow without changing plugin runtime behavior.

## Goals / Non-Goals

**Goals:**
- Provide a root-level Makefile that can build every plugin under `extensions/`.
- Support building one plugin or all discovered extension plugins.
- Produce `plugin.wasm` and `metadata.txt` under `build/plugins/<plugin>/<version>/`.
- Default the output version directory to `2.0.0` to match the requested example while allowing callers to override it.
- Generate metadata in a consistent field order with values derived from the produced artifact where applicable.

**Non-Goals:**
- Do not modify extension plugin runtime code or configuration behavior.
- Do not publish OCI images or push artifacts to a registry.
- Do not replace the existing Go unit test workflow.
- Do not require a new external build tool beyond the existing Go/TinyGo toolchain.

## Decisions

1. Use a root-level Makefile rather than per-plugin Makefiles.

   A single Makefile keeps the build entry point discoverable and avoids duplicating the same packaging logic in every extension directory. The alternative was to place a Makefile in each plugin module, but that would make all-plugin builds harder to coordinate and would increase maintenance work.

2. Discover plugins from `extensions/*` directories that contain both `go.mod` and `main.go`.

   This treats each extension as an independent Go module and avoids hard-coding today only `ai-billing`, `ai-quota`, and `ai-statistics`. The alternative was a static plugin list, which is simpler but would require updates whenever a new extension plugin is added.

3. Build outputs use `build/plugins/<plugin>/$(PLUGIN_VERSION)/`.

   `PLUGIN_VERSION` defaults to `2.0.0` so `ai-quota` produces the requested shape `build/plugins/ai-quota/2.0.0/`. The Makefile should allow callers to override `PLUGIN_VERSION` for other releases. The alternative was to read each plugin's `VERSION` file by default, but that would not match the requested example for `ai-quota` because its current `VERSION` file is `1.0.0`.

4. Metadata is generated from the output artifact.

   `metadata.txt` should use the field names and order from the request: `Plugin Name`, `Size`, `Last Modified`, `Created`, and `MD5`. `Plugin Name` comes from the plugin directory name, `Size` and `MD5` come from `plugin.wasm`, and timestamps are generated during packaging. The alternative was to hard-code the sample `ai-quota` values, but those values would be incorrect for other plugins and may be incorrect after a rebuild.

5. Use the existing documented Wasm build flags.

   The Makefile should prefer the repository-documented TinyGo command pattern for production plugin builds:
   `tinygo build -scheduler=none -target=wasi -gc=custom -tags='custommalloc nottinygc_finalizer ...'`.
   It should keep the tags configurable so the existing `proxy_wasm_version_0_2_100` tag can be added when needed.

## Risks / Trade-offs

- [TinyGo missing locally] -> The Makefile should fail with a clear command error; implementation can document the required toolchain through target names and variable defaults.
- [Plugin build flags need to vary later] -> Keep build flags and extra tags as Makefile variables so plugin-specific changes do not require restructuring the build flow.
- [Metadata timestamps are environment-dependent] -> Use stable, explicit timestamp formatting so generated files are predictable enough for consumers, while still reflecting the local build time.
- [New extension directories that are not buildable plugins] -> Require both `go.mod` and `main.go` for discovery to avoid packaging unrelated folders.
