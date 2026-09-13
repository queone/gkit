## namehunt

Find free usernames on any site with a predictable profile URL. Give it a site and one name, and it tells you whether that name is free. Give it a pattern, and it prints every free name the pattern expands to.

`namehunt` sends one HEAD request per name to a site's check URL and reads the status: 404 means free, any 2xx means taken, anything else is reported as unknown. Free names are a strong hint; only signing up confirms one.

### Usage

```text
namehunt v1.2.0
Find free usernames on any site with a predictable profile URL
github.com/queone/gkit/tree/main/cmd/namehunt

Usage
  namehunt [flags] SITE CANDIDATE  Check one name, or list every free name a pattern yields

  SITE is a site name or a URL template with {} where the username goes.
  Built-in sites: github, lichess, archive. More come from ~/.config/namehunt/sites,
  one per line: NAME TEMPLATE [FREE_CODES] [PROFILE_TEMPLATE]. The file is written
  with the built-ins on first run. CANDIDATE is one name, a pattern like
  [qk][aeou][qk][aeou], or - for stdin. 404 means free, 2xx means taken, anything
  else is unknown; only signing up confirms a name.

Options
  -d, --distinct       Keep only names with no repeated character
  -r, --require CHARS  Keep only names containing every character in CHARS
  -n, --dry-run        Print the names and send no request
  -f, --free CODES     Status codes that mean free, comma-separated (default 404)
  -w, --wait MS        Pause MS milliseconds between requests (default 300)
  -l, --list           List known sites and exit
  -v, --version        Print namehunt v1.2.0 and exit
  -h, -?, --help       Show this help and exit

Examples
  namehunt github kaqe
  namehunt archive qoku
  namehunt -d lichess '[qk][aeiou][qk][aeiou]'
  namehunt 'https://www.reddit.com/user/{}' kaqe
```

Run `namehunt` with no arguments, or with `-h`, to see the help screen.

| Flag | Meaning |
|------|---------|
| `-d, --distinct` | Keep only names with no repeated character. |
| `-r, --require CHARS` | Keep only names containing every character in `CHARS`. |
| `-n, --dry-run` | Print the names and send no request. |
| `-f, --free CODES` | Status codes that mean free, comma-separated. Default `404`. |
| `-w, --wait MS` | Pause `MS` milliseconds between requests. Default `300`; `0` sends them back to back. |
| `-l, --list` | List known sites and exit. |
| `-v, --version` | Print the version and exit. |
| `-h, --help` | Show the help screen. |

`SITE` is one of:

- a built-in site name: `github`, `lichess`, or `archive` (archive.org)
- a name from your sites file (see below)
- a URL template with `{}` where the username goes, such as `https://www.reddit.com/user/{}`. The template must start with `http://` or `https://`.

`CANDIDATE` is one of:

- one name, such as `kaqe`
- a pattern, such as `[qk][aeou][qk][aeou]`. Each `[...]` class stands for one character from the class. Every other character is literal.
- `-` to read one name per line from stdin

### Examples

```bash
namehunt github kaqe                                  # is kaqe free on GitHub?
namehunt archive qoku                                 # is qoku free on archive.org?
namehunt -d lichess '[qk][aeiou][qk][aeiou]'          # free 4-letter names, no repeated letters
namehunt 'https://www.reddit.com/user/{}' kaqe        # any site, by template
cat names.txt | namehunt -w 200 github -              # names from a file, paced 200 ms apart
namehunt -n -d -r qk github '[qkaeou][qkaeou][qkaeou][qkaeou]'   # just list the candidates
```

Three scans worth keeping:

```bash
# GitHub, every three-letter name from the letters a e o u q k t p m (729 names)
namehunt github '[aeouqktpm][aeouqktpm][aeouqktpm]'

# GitHub, four distinct letters from q k a e o u that include both q and k (144 names)
namehunt -d -r qk github '[qkaeou][qkaeou][qkaeou][qkaeou]'

# Lichess, q or k alternating with a vowel, no repeats (40 names)
namehunt -d lichess '[qk][aeiou][qk][aeiou]'
```

### Sites file

Name your own sites in `~/.config/namehunt/sites` (or `$XDG_CONFIG_HOME/namehunt/sites` when that variable is set to an absolute path). The first run that needs the file writes it with the three built-in sites, so you can edit them or add your own. An existing file is never touched.

One site per line, as `NAME TEMPLATE [CODES] [PROFILE]`: the name, the URL template namehunt checks, and optionally the comma-separated status codes that mean free and a second URL template for the page the person would own, when that differs from the checked URL. The two optional fields are told apart by shape: one that starts with `http://` or `https://` is the PROFILE template, anything else is the CODES list. Blank lines and lines starting with `#` are skipped. A line with a built-in name replaces the built-in.

```
# name     URL template checked               codes   profile URL, when different
github     https://github.com/{}
lichess    https://lichess.org/@/{}
archive    https://archive.org/download/@{}           https://archive.org/details/@{}
reddit     https://www.reddit.com/user/{}     404
```

Internet Archive is why the profile template exists: its profile page answers 200 whether or not the account exists, while the account's download URL answers 404 until someone signs up.

`namehunt -l` prints every known site, whether it is built in or from the file, and its profile template. A bad line stops the tool and names the line number.

### How "free" is decided

For each name, `namehunt` sends a `HEAD` request to the site's checked URL with a 5 second timeout and follows redirects. In pattern and stdin mode it pauses between names for the `-w` default 300 ms unless `-w` says otherwise.

- A status in the free list (`-f`, then the site's file entry, then `404`) means free.
- Any other 2xx status means taken.
- A 405 is retried once with `GET`.
- A 429 or a network error is retried up to five times, waiting 100 ms and doubling each time.
- Anything else, or five failed attempts, means unknown, and the status or error is shown.

Output and exit codes:

- One name: one line, `NAME: free URL`, `NAME: taken`, or `NAME: unknown (HTTP 403)`, where `URL` is the profile page the name would own. Exit 0 free, 1 taken, 2 unknown.
- Pattern or stdin: free names one per line on stdout as `NAME URL`. Unknown names go to stderr with their reason, followed by `Checked N: F free, T taken, U unknown.` Exit 0 when nothing is unknown, else 2.
- Whenever at least one name is free, stderr ends with `Free names are a strong hint; only signing up confirms one.`
- Bad input: one line on stderr, exit 2, no request sent.

Limits: the answer comes from the status code only. A site that answers 200 for a missing profile cannot be told apart from a taken name. Pick a site whose missing-profile status differs, or set that status with `-f` or in the sites file. Even a clean 404 only shows that no profile answered; a name is confirmed yours when you finish signing up.
