## 1. Build Discovery

- [x] 1.1 Add a root-level Makefile with variables for `EXTENSIONS_DIR`, `BUILD_DIR`, `PLUGIN_VERSION`, `PLUGIN`, and TinyGo build flags.
- [x] 1.2 Implement plugin discovery from immediate `extensions/*` directories that contain both `go.mod` and `main.go`.
- [x] 1.3 Add a target that prints or otherwise validates the discovered plugin list for troubleshooting.

## 2. Artifact Build Targets

- [x] 2.1 Add an all-plugin build target that builds every discovered extension plugin.
- [x] 2.2 Add a single-plugin build path controlled by `PLUGIN=<name>`.
- [x] 2.3 Compile each plugin from its own module directory and write `plugin.wasm` to `build/plugins/<plugin>/$(PLUGIN_VERSION)/plugin.wasm`.
- [x] 2.4 Fail clearly when `PLUGIN=<name>` does not match a discovered plugin.

## 3. Metadata Generation

- [x] 3.1 Generate `metadata.txt` beside each `plugin.wasm` after a successful build.
- [x] 3.2 Populate metadata fields in order: `Plugin Name`, `Size`, `Last Modified`, `Created`, and `MD5`.
- [x] 3.3 Ensure `Plugin Name` uses the plugin directory name and `Size`/`MD5` describe the generated `plugin.wasm`.

## 4. Verification

- [x] 4.1 Run the single-plugin build for `PLUGIN=ai-quota` and verify `build/plugins/ai-quota/2.0.0/plugin.wasm` and `metadata.txt` are created.
- [x] 4.2 Run the all-plugin build and verify each discovered plugin gets its own versioned output directory.
- [x] 4.3 Run `openspec validate add-extensions-build-makefile --strict` and fix any spec issues.
- [x] 4.4 Run `gitnexus_detect_changes()` and `graphify update .` after implementation before finalizing.
