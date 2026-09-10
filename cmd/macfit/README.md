## macfit
Keep Mac config files in one encrypted store and restore them on any Mac. macOS only.

The store is a single sealed file. By default it lives in `~/.local/share/macfit/`, which stays on one Mac. Point it at a synced folder once with `init -s PATH` and every Mac that sees the folder can open it. A folder inside iCloud Drive, such as `~/Library/Mobile Documents/com~apple~CloudDocs/etc/`, is a good choice; Dropbox, Google Drive, or any other synced folder works the same way.

```text
macfit init -N -s ~/data/etc/macfit.store     # first Mac: create the store and its key, remember the path
macfit add -g ~/.bash_logout ~/.bashrc         # register files for every Mac and capture them
macfit add ~/.ssh/config                      # register a file for this Mac only
macfit                                        # status: store, key, host, entries, conflicts, drift verdict
macfit push                                   # send changed live files into the store
macfit diff                                   # show what differs between the store and this Mac
macfit pull                                   # plan the restore: what would be written, nothing touched
macfit pull -f ~/.bashrc                      # write one file, or all of them with no target
macfit init -s ~/data/etc/macfit.store        # another Mac: unlock with the passphrase, remember the path
macfit render                                 # browse the stored files: a private temp directory, path printed
macfit cat ~/.bashrc | diff - ~/.bashrc       # one stored file on stdout
```

`push` sends live files up into the store. `pull` brings the store down onto the Mac. The store is the remote, as in git. Every entry belongs to the Mac it was added on unless `-g` made it global; on a Mac, its own entry wins over a global one for the same target.

### Where the store is

Each command finds the store in this order: `-s PATH` on the command line, then `MACFIT_STORE`, then the path that `init -s` remembered in `$XDG_CONFIG_HOME/macfit/store`, then `$XDG_DATA_HOME/macfit/macfit.store`. Only `init` writes the remembered path, and only when it was run with `-s`. `init -N` creates the default folder when needed; for any other location the folder must already exist, so a missing synced folder is never faked.

### What a store holds

The store is a SQLite database sealed inside an authenticated encryption envelope (XChaCha20-Poly1305). It is always encrypted on disk. A command decrypts it into memory, does its work, seals it again, and replaces the file atomically. Nothing but the store file is written. That is what makes a synced folder safe: the sync client only ever sees whole files.

Every registered file is stored as a target template, a file mode, an optional host binding, and every version captured so far. Targets use the directory variables of this machine, so a file at `~/.config/git/config` is stored as `$XDG_CONFIG_HOME/git/config` and lands wherever that variable points on the Mac that pulls it. The variables recognized are `$XDG_CONFIG_HOME`, `$XDG_DATA_HOME`, `$XDG_STATE_HOME`, `$XDG_CACHE_HOME`, `$CLAUDE_CONFIG_DIR`, and `~`, with the standard XDG fallbacks when a variable is unset. `add -l` keeps the path spelled under `~` instead.

The list of files is data inside your store, created by your `add` commands. macfit itself carries no path of yours.

### The key

`init -N` draws a random 256-bit key, saves it in the login keychain through the `security` command, and asks for a recovery passphrase. A copy of the key wrapped with that passphrase (Argon2id, then XChaCha20-Poly1305) sits in the store header. On another Mac that sees the same store file, `init` asks for the passphrase once and saves the key to that Mac's keychain. From then on every command works without a prompt.

`macfit key` maintains that keychain item: `key show` reports the store path, the key id, whether the item exists, and whether it opens the store, without printing the secret; `key restore` puts the key back with the passphrase; `key rm` deletes the item after a `[y/N]` prompt (`-f` skips it); `key passphrase` changes the recovery passphrase.

The key is protected exactly as well as your login keychain: any process running as your user can read it with the same `security` command, without a prompt. iCloud Keychain does not carry it; the passphrase is the cross-Mac path. During `init -N` and `key restore` the key passes to `security` as a command-line argument, so it is visible in the process list for that instant.

If the passphrase or the key ever leaks, create a new store with `init -N -s NEWPATH`, `add` your files again, and delete the old store and its keychain item with `key rm`.

### Daily use

- `macfit add PATH... [-H HOST|-g] [-l]` registers regular files and captures their content, mode, owner, and group. Each entry binds to this Mac's LocalHostName; `-H` binds it to another Mac, `-g` makes it global. A bound entry wins over a global one on its Mac and is ignored elsewhere.
- `macfit set TARGET [-H HOST|-g] [-m MODE] [-F HOST|global]` changes an existing entry's binding or stored mode without touching its versions. It picks the entry the way `rm` does, or the one `-F` names. Four files added without `-g` by mistake are four `set ... -H thismac` commands.
- `macfit push [TARGET...]` captures live files whose content or mode changed as new versions. Older versions stay in the store.
- `macfit pull [TARGET...]` prints the plan and writes nothing: `unchanged`, `would write` for a missing file, `would overwrite` for a differing one, `symlink` for a live symlink. `macfit pull -f` writes the plan, creating parent directories (0700 for a 0600 file, else 0755) and setting the mode; it never writes through a symlink. `-n` is accepted and means the plan, even next to `-f`.
- `macfit diff [TARGET...] [-V]` prints `=` same, `M` differs, `?` live file missing, one line per entry, and exits 1 when anything drifted. `-V` adds a unified diff block, set off by blank lines.
- Colors on a terminal, plain when piped: grey for `=` and `unchanged`, yellow for `M`, `differs`, `would overwrite`, and `missing`, orange for `?`, green for `would write`, `restored`, `added`, and `updated`, red for `symlink`. Inside a diff block the headers are dark grey, unchanged lines grey, and removed and added lines light yellow.
- `macfit ls` lists every entry: host (`<global>` for a global one), owner and group as recorded on the Mac that added or last pushed it, mode, last capture time, and target. `macfit rm TARGET [-H HOST]` removes one.
- `macfit st`, or just `macfit`, prints one status screen: the store path and where it came from, the remembered path, the store file's size, generation, and modification time (to see whether the other Mac's push has arrived), the key id and whether the keychain key opens the store, this Mac's hostname as `-H` sees it, entry counts, sync conflict copies, and the drift verdict: green `none`, or red `M` and `?` counts. Exit 0 with no drift, 1 with drift or when the store does not open, so `macfit && echo clean` works. On a terminal the values are dark grey, `store opens` is green or red, and conflict copies are yellow.
- `push` and `diff` refuse a live path that has become a symlink, as `pull` does, so a link is never read into the store or compared as if it were the file.
- `macfit render [-o DIR] [-a] [-f]` writes the latest stored copy of every entry into a directory tree shaped like the targets: `~/.bashrc` lands at `any/HOME/.bashrc`, `$XDG_CONFIG_HOME/git/config` at `any/XDG_CONFIG_HOME/git/config`, and an entry bound to `np11` under `np11/`. Files keep their stored mode, directories are 0700, and `MANIFEST.txt` at the top lists every file with its target, host, mode, generation, capture time, and digest. By default the tree goes into a fresh private temp directory whose path is printed; `-o DIR` chooses a place and refuses a non-empty one unless `-f`; `-a` adds every stored version under `versions/`. The copies are plaintext, so delete the directory when done.
- `macfit cat TARGET [-H HOST]` prints one entry's latest stored content to stdout, byte for byte, selecting the entry the way `rm` does.

A TARGET is either the template as `ls` shows it (`$XDG_CONFIG_HOME/git/config`) or the live path.

### Things to know

- Stores written by macfit 1.x are upgraded in memory when opened (the entries gain owner and group columns) and written back by the next command that saves; until a `push` refreshes them, older entries show `?:?` in `ls`. `pull` always writes files as the current user and never changes ownership.

- Two Macs writing the store while offline produce a sync conflict copy, named `macfit 2.store`, `macfit (1).store`, `macfit-DEVICE.store`, or similar depending on the sync client. macfit warns when one sits beside the store, and a stale writer that opened an older generation is refused rather than allowed to overwrite the newer file.
- A synced folder is not end-to-end encrypted unless the provider says so. The store is encrypted before it reaches the folder, so that does not matter for its contents.
- `init` and the `key` prompts need a terminal.
- Files only: no directories, globs, or symlinks. macOS `defaults` settings are a planned addition.

### Usage

```text
macfit v1.5.0
Keep Mac config files in one encrypted store and restore them on any Mac.

Overview
  The store is a single sealed file. Keep it in a synced folder and every Mac
  that sees the folder can open it. push sends live files into the store,
  pull restores them from the store, and diff shows what differs. The key
  lives in the login keychain, with a passphrase-wrapped copy in the store
  for other Macs.

Usage
  macfit [st]                         status of the store, key, and drift; no command means st
  macfit init [-N]                    unlock an existing store, or create one with -N
  macfit add PATH... [-H HOST|-g]     register live files for this Mac and capture them
  macfit set TARGET [flags]           change an entry's Mac binding or mode (-H, -g, -m, -F)
  macfit rm TARGET [-H HOST]          forget a file and its stored versions
  macfit ls                           list entries
  macfit push [TARGET...]             send changed live files into the store
  macfit pull [TARGET...] [-f]        plan the restore, or write it with -f
  macfit diff [TARGET...] [-V]        show drift between the store and this Mac
  macfit render [-o DIR] [-a] [-f]    write the stored files into a browsable directory
  macfit cat TARGET [-H HOST]         print one stored file
  macfit key show                     store path, key id, keychain and store state
  macfit key restore                  put the key back in the keychain with the passphrase
  macfit key rm [-f]                  delete the keychain item after a prompt
  macfit key passphrase               change the recovery passphrase

Options
  -s, --store PATH   Store file for this command; init -s also remembers it
  -N, --new          Create a new store (init)
  -H, --host NAME    Bind the entry to another Mac (add, set, rm, cat)
  -g, --global       Make the entry apply on every Mac (add, set)
  -m, --mode MODE    Store a new mode, three or four octal digits (set)
  -F, --from WHICH   Pick the entry to change by its binding: a host name or global (set)
  -l, --literal      Keep the path under ~ instead of an XDG variable (add)
  -n, --dry-run      Print the pull plan; the default, kept for scripts
  -f, --force        Write the pull plan, overwriting live files that differ; skip the key rm prompt
  -V, --verbose      Add a unified diff to diff output
  -o, --out DIR      Render into DIR instead of a fresh private temp directory (render)
  -a, --all          Render every stored version too, under versions/ (render)
  -v, --version      Print macfit v1.5.0 and exit
  -h, -?, --help     Show this help message and exit

Notes
  Store path order: -s, then MACFIT_STORE, then the path init -s remembered in
  $XDG_CONFIG_HOME/macfit/store, then $XDG_DATA_HOME/macfit/macfit.store.
  A TARGET is the template ls shows ($XDG_CONFIG_HOME/git/config) or the live path.
  add binds a file to this Mac unless -g; on a Mac, its own entry wins over a global one.
  init needs a terminal for the passphrase prompt and creates only the default folder.
  render writes plaintext copies of the store; delete the directory when done.
  Files only: no directories, globs, or symlinks. macOS defaults settings are a planned addition.
```
