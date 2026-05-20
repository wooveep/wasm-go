## ADDED Requirements

### Requirement: Discover extension plugins
The build system SHALL discover extension plugins from immediate child directories under `extensions/` that contain both `go.mod` and `main.go`.

#### Scenario: Discover current extension plugins
- **WHEN** the build system scans the current `extensions/` directory
- **THEN** it identifies `ai-billing`, `ai-quota`, and `ai-statistics` as buildable plugins

#### Scenario: Ignore non-plugin extension directories
- **WHEN** an immediate child directory under `extensions/` does not contain both `go.mod` and `main.go`
- **THEN** the build system does not include that directory in the plugin build list

### Requirement: Build all extension plugin artifacts
The build system SHALL provide a Makefile target that compiles every discovered extension plugin and writes each plugin's artifacts under `build/plugins/<plugin>/<version>/`.

#### Scenario: Build all plugins with default version
- **WHEN** the user runs the default all-plugin build target without overriding the version
- **THEN** each discovered plugin has `plugin.wasm` written under `build/plugins/<plugin>/2.0.0/`

#### Scenario: Build all plugins with overridden version
- **WHEN** the user runs the all-plugin build target with `PLUGIN_VERSION=3.1.4`
- **THEN** each discovered plugin has `plugin.wasm` written under `build/plugins/<plugin>/3.1.4/`

### Requirement: Build a single extension plugin artifact
The build system SHALL provide a Makefile path to compile a single selected extension plugin without requiring all other plugins to build.

#### Scenario: Build selected plugin
- **WHEN** the user requests a build for `PLUGIN=ai-quota`
- **THEN** the build system writes `build/plugins/ai-quota/<version>/plugin.wasm`

#### Scenario: Reject missing plugin
- **WHEN** the user requests a build for a plugin name that is not a discovered extension plugin
- **THEN** the build system fails without creating artifacts for that plugin name

### Requirement: Generate plugin metadata
The build system SHALL generate `metadata.txt` beside every generated `plugin.wasm` using the field order `Plugin Name`, `Size`, `Last Modified`, `Created`, and `MD5`.

#### Scenario: Generate metadata for each plugin
- **WHEN** a plugin build succeeds
- **THEN** `metadata.txt` exists in the same versioned output directory as `plugin.wasm`

#### Scenario: Metadata identifies plugin
- **WHEN** `metadata.txt` is generated for `ai-quota`
- **THEN** it contains `Plugin Name: ai-quota`

#### Scenario: Metadata describes generated wasm
- **WHEN** `metadata.txt` is generated for a plugin
- **THEN** its `Size` and `MD5` values describe the generated `plugin.wasm` in the same directory

### Requirement: Keep build automation non-runtime
The build system SHALL implement artifact packaging without changing extension plugin runtime code, plugin configuration schemas, or WasmPlugin resource behavior.

#### Scenario: Packaging-only change
- **WHEN** the Makefile build flow is added
- **THEN** no extension `main.go` runtime logic changes are required
