# repoctl

Control a collection of local Git repositories.

```text
repoctl 0.4.0
repoctl s, status [REPO ...]
repoctl p, pull [REPO ...]
repoctl c, clone NAME
repoctl c, clone OWNER REPO ...
repoctl c, clone OWNER/REPO
repoctl l, list
repoctl b, build [REPO ...]
```

`repoctl` discovers immediate, non-hidden subdirectories containing a `.git`
directory. Status, pull, and build operations process repositories in origin
order; a requested subset must name discovered repositories. Failed operations
continue through the remaining repositories and return a non-zero exit status.

`clone NAME` bulk-clones repositories when `NAME` matches the authenticated
GitHub user or an organization returned by `gh`. Otherwise it searches those
owners for one exact repository name. Use `OWNER REPO ...` or `OWNER/REPO` to
clone directly without the scope check. `list` prints repositories from the
authenticated user's and organizations' scopes without cloning. Its aligned
`REPO`, `DESCRIPTION`, `INFO`, and `UPDATED` columns show the short
`github.com/<owner>/<repo>` path, GitHub description, visibility and archive
state, and relative update age. When terminal color is enabled, `private` and
`archived` labels are yellow; `NO_COLOR` and non-terminal output remain plain.
Rows are ordered by repository path from A to Z, then from newest to oldest
update when repository paths match. Each owner query retrieves up to 1,000
repositories.

The former `git-cloneall`, `git-pullall`, `git-remotev`, and `git-statall`
commands are replaced by `repoctl clone`, `repoctl pull`, and `repoctl status`;
the latter two status-oriented commands are both covered by `repoctl status`.
Git is required for local operations;
GitHub CLI is required for scoped clone and list operations.
