// Command repoctl controls a collection of local Git repositories.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/mattn/go-runewidth"
)

const (
	programName    = "repoctl"
	programVersion = "0.4.0"
	yellow         = "\033[38;5;226m"
	reset          = "\033[0m"
)

type cliError struct {
	message string
	code    int
}

func (e *cliError) Error() string { return e.message }

type command struct {
	name string
	args []string
}

type repoResult struct {
	name, origin, status string
	details              []string
	failed               bool
}

func main() {
	code := 0
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		if e, ok := err.(*cliError); ok {
			code = e.code
		} else {
			code = 1
		}
	}
	os.Exit(code)
}

func run(args []string, stdout, stderr io.Writer) error {
	cmd, err := parse(args)
	if err != nil {
		return err
	}
	color := colorEnabled(stdout)
	switch cmd.name {
	case "help":
		_, err = fmt.Fprint(stdout, helpText(color))
	case "version":
		_, err = fmt.Fprintf(stdout, "%s %s\n", programName, programVersion)
	case "status", "pull", "build":
		err = runLocal(cmd.name, cmd.args, color, stdout)
	case "clone":
		err = runClone(cmd.args, color, stdout)
	case "list":
		err = runList(stdout)
	}
	if err != nil {
		if _, ok := err.(*cliError); ok {
			return err
		}
		return &cliError{message: err.Error(), code: 1}
	}
	return nil
}

func parse(args []string) (command, error) {
	if len(args) == 0 {
		return command{name: "help"}, nil
	}
	for _, arg := range args {
		if arg == "-h" || arg == "-?" || arg == "--help" {
			return command{name: "help"}, nil
		}
		if arg == "-v" || arg == "--version" {
			return command{name: "version"}, nil
		}
	}
	for _, arg := range args[1:] {
		if strings.HasPrefix(arg, "-") {
			return command{}, usageError("parse operand: unsupported option; use repoctl --help")
		}
	}
	name := args[0]
	switch name {
	case "s", "status", "p", "pull", "b", "build":
		return command{name: map[string]string{"s": "status", "status": "status", "p": "pull", "pull": "pull", "b": "build", "build": "build"}[name], args: args[1:]}, nil
	case "l", "list":
		if len(args) != 1 {
			return command{}, usageError("list: expected no arguments; use repoctl --help")
		}
		return command{name: "list"}, nil
	case "c", "clone":
		return parseClone(args[1:])
	default:
		return command{}, usageError(fmt.Sprintf("parse command %q: expected s/status, p/pull, c/clone, l/list, or b/build; use repoctl --help", name))
	}
}

func parseClone(args []string) (command, error) {
	if len(args) == 0 {
		return command{}, usageError("clone: expected NAME, OWNER/REPO, or OWNER REPO ...; use repoctl --help")
	}
	if slices.Contains(args, "") {
		return command{}, usageError("clone: OWNER and REPO names must not be empty; use repoctl --help")
	}
	if strings.Contains(args[0], "/") {
		if len(args) != 1 || !validQualified(args[0]) {
			return command{}, usageError("clone: OWNER/REPO must have exactly one '/' with both parts non-empty; use repoctl --help")
		}
		return command{name: "clone", args: args}, nil
	}
	return command{name: "clone", args: args}, nil
}

func usageError(message string) error { return &cliError{message: message, code: 2} }

func runLocal(operation string, requested []string, color bool, stdout io.Writer) error {
	if err := require("git", "install Git and ensure git is on PATH"); err != nil {
		return err
	}
	repos, err := discover()
	if err != nil {
		return err
	}
	for _, name := range requested {
		if !containsRepo(repos, name) {
			return usageError(fmt.Sprintf("select repository %q: no matching immediate Git repository; use repoctl --help", name))
		}
	}
	if len(requested) > 0 {
		repos = filterRepos(repos, requested)
	}
	for i := range repos {
		repos[i].origin = origin(repos[i].name)
	}
	sort.Slice(repos, func(i, j int) bool {
		if repos[i].origin == repos[j].origin {
			return repos[i].name < repos[j].name
		}
		return repos[i].origin < repos[j].origin
	})
	widths := widths(repos)
	failed := false
	for _, repo := range repos {
		var result repoResult
		switch operation {
		case "status":
			result = statusRepo(repo)
		case "pull":
			result = pullRepo(repo)
		case "build":
			fmt.Fprint(stdout, processing(repo, "Building", color, widths))
			result = buildRepo(repo, color)
			printDetails(stdout, result.details)
			fmt.Fprint(stdout, finalStatus(result, color))
		}
		if operation != "build" {
			fmt.Fprint(stdout, resultRow(result, color, widths))
		}
		failed = failed || result.failed
	}
	if failed {
		return &cliError{message: "one or more repository operations failed", code: 1}
	}
	return nil
}

func runClone(args []string, color bool, stdout io.Writer) error {
	if err := require("git", "install Git and ensure git is on PATH"); err != nil {
		return err
	}
	owner, names, err := resolveClone(args)
	if err != nil {
		return err
	}
	if len(names) == 0 {
		if err := require("gh", "install GitHub CLI and authenticate with gh"); err != nil {
			return err
		}
		names, err = remoteRepos(owner)
		if err != nil {
			return err
		}
	}
	for _, name := range names {
		if !validName(name) {
			return usageError("clone: repository names must be simple directory names; use repoctl --help")
		}
	}
	repos := make([]repoResult, 0, len(names))
	for _, name := range names {
		repos = append(repos, repoResult{name: name, origin: fmt.Sprintf("https://github.com/%s/%s.git", owner, name)})
	}
	sort.Slice(repos, func(i, j int) bool { return repos[i].origin < repos[j].origin })
	w := widths(repos)
	failed := false
	for _, repo := range repos {
		fmt.Fprint(stdout, processing(repo, "Cloning", color, w))
		result := cloneRepo(repo)
		printDetails(stdout, result.details)
		fmt.Fprint(stdout, finalStatus(result, color))
		failed = failed || result.failed
	}
	if failed {
		return &cliError{message: "one or more repository operations failed", code: 1}
	}
	return nil
}

func resolveClone(args []string) (string, []string, error) {
	if len(args) == 1 && strings.Contains(args[0], "/") {
		parts := strings.Split(args[0], "/")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return "", nil, usageError("clone: OWNER/REPO must have exactly one '/' with both parts non-empty; use repoctl --help")
		}
		return parts[0], []string{parts[1]}, nil
	}
	if len(args) >= 2 {
		return args[0], args[1:], nil
	}
	if len(args) != 1 {
		return "", nil, usageError("clone: expected NAME, OWNER/REPO, or OWNER REPO ...; use repoctl --help")
	}
	if err := require("gh", "install GitHub CLI and authenticate with gh"); err != nil {
		return "", nil, err
	}
	owners, err := scopeOwners()
	if err != nil {
		return "", nil, err
	}
	for _, owner := range owners {
		if strings.EqualFold(owner, args[0]) {
			return owner, nil, nil
		}
	}
	var matches []string
	for _, owner := range owners {
		names, listErr := remoteRepos(owner)
		if listErr != nil {
			return "", nil, listErr
		}
		for _, name := range names {
			if name == args[0] {
				matches = append(matches, owner)
			}
		}
	}
	switch len(matches) {
	case 0:
		return "", nil, usageError(fmt.Sprintf("clone: %q not found among your GitHub account and orgs; use OWNER/REPO to clone a repository outside that scope", args[0]))
	case 1:
		return matches[0], []string{args[0]}, nil
	default:
		hits := make([]string, len(matches))
		for i, owner := range matches {
			hits[i] = owner + "/" + args[0]
		}
		return "", nil, usageError(fmt.Sprintf("clone: %q matches more than one repository in scope (%s); qualify as OWNER/REPO", args[0], strings.Join(hits, ", ")))
	}
}

func runList(stdout io.Writer) error {
	return runListAt(stdout, time.Now(), colorEnabled(stdout))
}

type remoteRepo struct {
	NameWithOwner string    `json:"nameWithOwner"`
	Description   string    `json:"description"`
	Visibility    string    `json:"visibility"`
	IsArchived    bool      `json:"isArchived"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

func runListAt(stdout io.Writer, now time.Time, color bool) error {
	if err := require("gh", "install GitHub CLI and authenticate with gh"); err != nil {
		return err
	}
	owners, err := scopeOwners()
	if err != nil {
		return err
	}
	var repos []remoteRepo
	failed := false
	for _, owner := range owners {
		ownerRepos, listErr := remoteRepoDetails(owner)
		if listErr != nil {
			fmt.Fprintln(stdout, "    "+listErr.Error())
			failed = true
			continue
		}
		repos = append(repos, ownerRepos...)
	}
	sortRemoteRepos(repos)
	rows := make([][4]string, 0, len(repos)+1)
	rows = append(rows, [4]string{"REPO", "DESCRIPTION", "INFO", "UPDATED"})
	for _, repo := range repos {
		rows = append(rows, [4]string{
			"github.com/" + repo.NameWithOwner,
			tableCell(repo.Description),
			repoInfo(repo, false),
			relativeAge(repo.UpdatedAt, now),
		})
	}
	widths := [3]int{}
	for _, row := range rows {
		for column := range widths {
			widths[column] = max(widths[column], runewidth.StringWidth(row[column]))
		}
	}
	for index, row := range rows {
		info := row[2]
		if color && index > 0 {
			info = repoInfo(repos[index-1], true)
		}
		if _, err := fmt.Fprintf(stdout, "%s%s  %s%s  %s%s  %s\n",
			row[0], spaces(widths[0]-runewidth.StringWidth(row[0])),
			row[1], spaces(widths[1]-runewidth.StringWidth(row[1])),
			info, spaces(widths[2]-runewidth.StringWidth(row[2])), row[3]); err != nil {
			return err
		}
	}
	if failed {
		return &cliError{message: "one or more repository listings failed", code: 1}
	}
	return nil
}

func repoInfo(repo remoteRepo, color bool) string {
	visibility := strings.ToLower(repo.Visibility)
	if visibility == "private" {
		visibility = yellowTableLabel(visibility, color)
	}
	if !repo.IsArchived {
		return visibility
	}
	return visibility + ", " + yellowTableLabel("archived", color)
}

func yellowTableLabel(label string, color bool) string {
	if !color {
		return label
	}
	return yellow + label + reset
}

func sortRemoteRepos(repos []remoteRepo) {
	sort.Slice(repos, func(i, j int) bool {
		if repos[i].NameWithOwner == repos[j].NameWithOwner {
			return repos[i].UpdatedAt.After(repos[j].UpdatedAt)
		}
		return repos[i].NameWithOwner < repos[j].NameWithOwner
	})
}

func tableCell(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func relativeAge(updatedAt, now time.Time) string {
	elapsed := max(now.Sub(updatedAt), 0)
	quantity, unit := int64(elapsed/time.Second), "second"
	switch {
	case elapsed >= 365*24*time.Hour:
		quantity, unit = int64(elapsed/(365*24*time.Hour)), "year"
	case elapsed >= 30*24*time.Hour:
		quantity, unit = int64(elapsed/(30*24*time.Hour)), "month"
	case elapsed >= 24*time.Hour:
		quantity, unit = int64(elapsed/(24*time.Hour)), "day"
	case elapsed >= time.Hour:
		quantity, unit = int64(elapsed/time.Hour), "hour"
	case elapsed >= time.Minute:
		quantity, unit = int64(elapsed/time.Minute), "minute"
	}
	if quantity != 1 {
		unit += "s"
	}
	return fmt.Sprintf("about %d %s ago", quantity, unit)
}

func discover() ([]repoResult, error) {
	entries, err := os.ReadDir(".")
	if err != nil {
		return nil, runtimeError("read repository directory", err, "verify the current directory is readable and retry")
	}
	var repos []repoResult
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		if info, statErr := os.Stat(filepath.Join(entry.Name(), ".git")); statErr == nil && info.IsDir() {
			repos = append(repos, repoResult{name: entry.Name()})
		}
	}
	sort.Slice(repos, func(i, j int) bool { return repos[i].name < repos[j].name })
	return repos, nil
}

func statusRepo(repo repoResult) repoResult {
	branch, branchErr := inRepo(repo.name, "git", "branch", "--show-current")
	status, statusErr := inRepo(repo.name, "git", "status", "--porcelain")
	if branchErr != nil || statusErr != nil {
		repo.status, repo.failed = "Status failed", true
		if branchErr != nil {
			repo.details = append(repo.details, branchErr.Error())
		}
		if statusErr != nil {
			repo.details = append(repo.details, statusErr.Error())
		}
		return repo
	}
	if strings.TrimSpace(branch) == "" {
		branch = "(detached)"
	}
	mark := "👍"
	if strings.TrimSpace(status) != "" {
		mark = "❌"
	}
	repo.status = mark + " " + strings.TrimSpace(branch)
	return repo
}

func pullRepo(repo repoResult) repoResult {
	if _, err := inRepo(repo.name, "git", "ls-remote"); err != nil {
		repo.status, repo.failed, repo.details = "Remote unavailable", true, []string{err.Error()}
		return repo
	}
	out, err := execute(filepath.Join(repo.name), "git", "pull")
	if err != nil {
		repo.status, repo.failed = "Pull failed", true
		repo.details = append(repo.details, combined(out), err.Error())
		return repo
	}
	text := combined(out)
	if routinePull(text) {
		repo.status = "Already up to date"
	} else {
		repo.status, repo.details = "Pulled", nonEmptyLines(text)
	}
	return repo
}

func buildRepo(repo repoResult, color bool) repoResult {
	path := filepath.Join(repo.name, "build.sh")
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
		repo.status, repo.failed = "No build.sh", true
		repo.details = []string{path + " is missing or not executable; restore an executable build.sh and retry"}
		return repo
	}
	cmd := exec.Command("./build.sh")
	cmd.Dir = repo.name
	if color {
		if _, exists := os.LookupEnv("GOVERNA_FORCE_TTY"); !exists {
			cmd.Env = append(os.Environ(), "GOVERNA_FORCE_TTY=1")
		}
	}
	out, err := commandWith(cmd)
	if err != nil {
		repo.status, repo.failed = "Build failed", true
		repo.details = append(repo.details, nonEmptyLines(combined(out))...)
		repo.details = append(repo.details, err.Error())
		return repo
	}
	repo.status = "Built"
	return repo
}

func cloneRepo(repo repoResult) repoResult {
	if _, err := os.Stat(repo.name); err == nil {
		repo.status = "Skipped"
		return repo
	}
	out, err := execute(".", "git", "clone", repo.origin, repo.name)
	if err != nil {
		repo.status, repo.failed = "Clone failed", true
		repo.details = append(repo.details, nonEmptyLines(combined(out))...)
		repo.details = append(repo.details, err.Error())
		return repo
	}
	repo.status = "Cloned"
	return repo
}

func origin(repo string) string {
	out, err := inRepo(repo, "git", "remote", "get-url", "origin")
	if err != nil {
		return "<no origin>"
	}
	return strings.TrimSpace(out)
}

func scopeOwners() ([]string, error) {
	user, err := ghText("api", "user", "--jq", ".login")
	if err != nil {
		return nil, err
	}
	orgs, err := ghText("api", "user/orgs", "--jq", ".[].login")
	if err != nil {
		return nil, err
	}
	owners := []string{strings.TrimSpace(user)}
	owners = append(owners, nonEmptyLines(orgs)...)
	return owners, nil
}
func remoteRepos(owner string) ([]string, error) {
	text, err := ghText("repo", "list", owner, "--json", "name", "--jq", ".[].name")
	if err != nil {
		return nil, err
	}
	return nonEmptyLines(text), nil
}
func remoteRepoDetails(owner string) ([]remoteRepo, error) {
	text, err := ghText("repo", "list", owner, "--limit", "1000", "--json", "nameWithOwner,description,visibility,isArchived,updatedAt")
	if err != nil {
		return nil, err
	}
	var repos []remoteRepo
	if err := json.Unmarshal([]byte(text), &repos); err != nil {
		return nil, runtimeError("decode GitHub repositories for "+owner, err, "retry the GitHub query")
	}
	return repos, nil
}
func ghText(args ...string) (string, error) {
	out, err := execute(".", "gh", args...)
	if err != nil {
		return "", runtimeError("run gh "+strings.Join(args, " "), err, "authenticate with gh and retry")
	}
	return out.stdout, nil
}
func require(name, guidance string) error {
	if _, err := exec.LookPath(name); err != nil {
		return runtimeError("check "+name, err, guidance+" and retry")
	}
	return nil
}

type commandOutput struct {
	stdout, stderr string
}

func execute(dir, name string, args ...string) (commandOutput, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	return commandWith(cmd)
}
func commandWith(cmd *exec.Cmd) (commandOutput, error) {
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	out := commandOutput{stdout: stdout.String(), stderr: stderr.String()}
	if err != nil {
		return out, fmt.Errorf("%s: %w", strings.Join(cmd.Args, " "), err)
	}
	return out, nil
}
func inRepo(repo, name string, args ...string) (string, error) {
	out, err := execute(repo, name, args...)
	if err != nil {
		return "", fmt.Errorf("%s in %s: %w", name, repo, err)
	}
	return out.stdout, nil
}
func runtimeError(operation string, err error, recovery string) error {
	return &cliError{message: fmt.Sprintf("%s: %v; %s", operation, err, recovery), code: 1}
}
func combined(out commandOutput) string {
	text := strings.TrimSpace(out.stdout)
	if strings.TrimSpace(out.stderr) != "" {
		if text != "" {
			text += "\n"
		}
		text += strings.TrimSpace(out.stderr)
	}
	return strings.TrimSpace(text)
}
func nonEmptyLines(text string) []string {
	var lines []string
	for line := range strings.SplitSeq(text, "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, strings.TrimRight(line, "\r"))
		}
	}
	return lines
}
func routinePull(text string) bool {
	lines := nonEmptyLines(text)
	return len(lines) == 1 && (lines[0] == "Already up to date" || lines[0] == "Already up to date." || lines[0] == "Already up-to-date" || lines[0] == "Already up-to-date.")
}
func validName(name string) bool {
	return name != "" && name != "." && name != ".." && !strings.ContainsAny(name, `/\\`)
}
func validQualified(name string) bool {
	p := strings.Split(name, "/")
	return len(p) == 2 && p[0] != "" && p[1] != "" && !strings.Contains(p[1], "/")
}
func containsRepo(repos []repoResult, name string) bool {
	for _, repo := range repos {
		if repo.name == name {
			return true
		}
	}
	return false
}
func filterRepos(repos []repoResult, names []string) []repoResult {
	var out []repoResult
	for _, repo := range repos {
		if slices.Contains(names, repo.name) {
			out = append(out, repo)
		}
	}
	return out
}

type columnWidths struct{ repo, origin int }

func widths(repos []repoResult) columnWidths {
	var w columnWidths
	for _, repo := range repos {
		if len([]rune(repo.name)) > w.repo {
			w.repo = len([]rune(repo.name))
		}
		if len([]rune(displayOrigin(repo.origin))) > w.origin {
			w.origin = len([]rune(displayOrigin(repo.origin)))
		}
	}
	return w
}
func displayOrigin(origin string) string { return strings.TrimSuffix(origin, ".git") }
func processing(repo repoResult, action string, color bool, w columnWidths) string {
	return paint(fmt.Sprintf("==> %s%s%s%s%s\n", repo.name, spaces(w.repo-len([]rune(repo.name))+4), displayOrigin(repo.origin), spaces(w.origin-len([]rune(displayOrigin(repo.origin)))+4), action), color)
}
func resultRow(repo repoResult, color bool, w columnWidths) string {
	var b strings.Builder
	b.WriteString(paint(fmt.Sprintf("==> %s%s%s%s%s\n", repo.name, spaces(w.repo-len([]rune(repo.name))+4), displayOrigin(repo.origin), spaces(w.origin-len([]rune(displayOrigin(repo.origin)))+4), repo.status), color))
	for _, detail := range repo.details {
		b.WriteString("    " + detail + "\n")
	}
	return b.String()
}
func finalStatus(repo repoResult, color bool) string { return paint("    "+repo.status+"\n", color) }
func printDetails(w io.Writer, details []string) {
	for _, detail := range details {
		fmt.Fprintln(w, "    "+detail)
	}
}
func spaces(n int) string { return strings.Repeat(" ", n) }
func paint(text string, color bool) string {
	if !color {
		return text
	}
	return yellow + strings.TrimSuffix(text, "\n") + reset + "\n"
}
func colorEnabled(w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	f, ok := w.(*os.File)
	return ok && f != nil && isTerminal(f)
}
func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func helpText(color bool) string {
	name := paint(programName, color)
	return fmt.Sprintf("%s v%s\nControl a collection of local Git repositories.\n\nUsage\n  %s COMMAND [REPO ...]\n  %s clone NAME\n  %s clone OWNER REPO ...\n  %s clone OWNER/REPO\n\nCommands\n  s, status  Show repository status\n  p, pull    Pull selected repositories\n  c, clone   Clone a repository\n  l, list    List repositories in scope\n  b, build   Run ./build.sh in selected repositories\n\nOptions\n  -v, --version  Print version and exit\n  -h, -?, --help Print this help message and exit\n", name, programVersion, name, name, name, name)
}
