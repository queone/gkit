# web
Based on [duckgo](https://github.com/sheepla/duckgo) and [ddgr](https://github.com/jarun/ddgr).
Just a CLI utility to query DuckDuckGo and open items from search result with Web browser quickly.

### Usage

```text
web v1.1.0
Search DuckDuckGo and open a result picked with a fuzzy finder
github.com/queone/gkit/tree/main/cmd/web

Usage
  web [options] QUERY...  Search, pick a result, and open it in the browser

Options
  -j, --json           Print the results as JSON instead of opening one
  -t, --timeout N      Give up after N seconds (default 5)
  -u, --user-agent UA  Send UA as the User-Agent header
  -r, --referrer URL   Send URL as the Referer header
  -b, --browser CMD    Open the result with CMD instead of the default browser
  -v, --version        Print web v1.1.0 and exit
  -h, -?, --help       Show this help and exit

Examples
  web golang
  web -j golang
  web -t 10 -b firefox golang
```
