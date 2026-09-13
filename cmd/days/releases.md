## Releases

### 1.2.0
Release Date: 2026-sep-12
- Help follows the shared gkit standard: a three-line header ending with the repository URL, `Usage`, `Options`, and `Examples` sections rendered by `internal/help`, and `-?` accepted as a help flag.

### 1.1.0
Release Date: 2026-may-01
- Replaced panic-on-bad-input pattern in date helpers (`getDateInDays`, `getDaysSinceOrTo`, `getDaysBetween`) with error returns; main.go now exits with a one-line stderr message instead of a Go stack trace.
- Added local `die` helper in main.go.
