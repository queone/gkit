## Releases

### 1.6.0
Release Date: 2026-may-01
- Tightened error handling: `printOut` now uses explicit error checks instead of silent `_ = json.Unmarshal` / `_ = yaml.Unmarshal` probes; marshaling failures (`goyaml.YAMLToJSON`, `jsonBytesReindent`, `jsonBytesToJsonObj`) now exit with a one-line stderr message instead of producing partial output.
- `processPipedInput` now exits cleanly on stdin read errors instead of continuing with partial bytes.
- `printYaml` and `printYamlColor` return `error` instead of swallowing encode failures with `fmt.Println`.

---

### 1.5.0
Release Date: 2026-may-01
- Internalized `queone/utl` json/yaml/file helpers into `cmd/jy/json.go` and `cmd/jy/yaml.go`; main.go gains local `die`/`fileUsable`/`loadFileText`/`loadFileYamlBytes`. `die` now writes to stderr.
- Removed `github.com/queone/utl` dependency (gkit repo no longer imports it).

---

### 1.4.6
Release Date: 2025-oct-27
- Moved to <github.com/queone/gkit>

---

### 1.4.5
Release Date: 2025-feb-18
- Updated colors in `printUsage()` to bright white `utl.Whi2()`
- Upgraded to new `build_go`
- Updated dependencies:
  - <github.com/queone/utl> v1.3.0

---

### 1.4.4
Release Date: 2025-feb-15
- Updated `printUse()` style

---

### 1.4.3
Release Date: 2025-feb-01
- Moved repo from old user account to this new org
- Updated `main()` args handling to allow args to be positioned anywhere
- Update `printUse()` with new string literal variable allowing more readable code
- Updated README with better reasons for being
- Now using updated `build_go` script
- Updated to use Go version 1.23.5
- Updated `go.mod` dependencies to latest 
