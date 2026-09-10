package main

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"regexp"
	"strconv"
	"syscall"

	"github.com/queone/gkit/internal/lockbox"
)

// bindingFor resolves the host a new entry binds to: -H, -g, or this Mac.
func (a *app) bindingFor(host string, explicit, global bool) (string, error) {
	switch {
	case global:
		return "", nil
	case explicit:
		return host, nil
	case a.host == "":
		return "", errors.New("this Mac's hostname is unknown; pass -H HOST or -g")
	}
	return a.host, nil
}

// bindingOf names a binding for output: "host np10" or "global".
func bindingOf(host string) string {
	if host == "" {
		return "global"
	}
	return "host " + host
}

// fileOwnership captures a live file's owner and group, by id and by name.
// A name that cannot be looked up falls back to the numeric id.
func fileOwnership(info os.FileInfo) lockbox.Ownership {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return lockbox.UnknownOwnership
	}
	own := lockbox.Ownership{UID: int64(st.Uid), GID: int64(st.Gid)}
	own.Owner = strconv.FormatInt(own.UID, 10)
	own.Group = strconv.FormatInt(own.GID, 10)
	if u, err := user.LookupId(own.Owner); err == nil && u.Username != "" {
		own.Owner = u.Username
	}
	if g, err := user.LookupGroupId(own.Group); err == nil && g.Name != "" {
		own.Group = g.Name
	}
	return own
}

var modePattern = regexp.MustCompile(`^0?[0-7]{3}$`)

// parseMode reads a file mode given as three or four octal digits.
func parseMode(s string) (os.FileMode, error) {
	if !modePattern.MatchString(s) {
		return 0, fmt.Errorf("mode %q is not three or four octal digits", s)
	}
	n, err := strconv.ParseUint(s, 8, 32)
	if err != nil {
		return 0, err
	}
	return os.FileMode(n), nil
}

// cmdSet changes one existing entry's Mac binding or stored mode.
func (a *app) cmdSet(ref storeRef, args []string) int {
	flags, pos, err := parseArgs(args, []flagSpec{
		{"-H", "--host", true}, {"-g", "--global", false}, {"-m", "--mode", true}, {"-F", "--from", true},
	})
	newHost, hasHost := flags["--host"]
	global := flags["--global"] == "true"
	modeStr, hasMode := flags["--mode"]
	from, hasFrom := flags["--from"]
	if err != nil || len(pos) != 1 || (hasHost && global) || (!hasHost && !global && !hasMode) {
		a.errorf("set: usage: macfit set TARGET [-H HOST | -g] [-m MODE] [-F HOST|global]")
		return 2
	}
	var mode os.FileMode
	if hasMode {
		if mode, err = parseMode(modeStr); err != nil {
			a.errorf("set: %s", err)
			return 2
		}
	}
	st, err := a.openStore(ref.path)
	if err != nil {
		a.errorf("set: %s", err)
		return 1
	}
	defer st.Close()
	all, err := st.Entries()
	if err != nil {
		a.errorf("set: %s", err)
		return 1
	}
	var pick *lockbox.Entry
	if hasFrom {
		fromHost := from
		if from == "global" {
			fromHost = ""
		}
		pick = a.pickEntry(all, pos[0], fromHost, true)
	} else {
		pick = a.pickEntry(all, pos[0], "", false)
	}
	if pick == nil {
		if hasFrom {
			a.errorf("set: %s is not registered as %s", pos[0], bindingOf(map[bool]string{true: "", false: from}[from == "global"]))
		} else {
			a.errorf("set: %s is not registered", pos[0])
		}
		return 1
	}
	if hasHost || global {
		target := ""
		if hasHost {
			target = newHost
		}
		if target != pick.Host {
			err := st.SetHost(pick.ID, target)
			if errors.Is(err, lockbox.ErrExists) {
				a.errorf("set: %s already has an entry that is %s", pick.Target, bindingOf(target))
				return 1
			}
			if err != nil {
				a.errorf("set: %s", err)
				return 1
			}
			pick.Host = target
		}
	}
	if hasMode {
		if err := st.SetMode(pick.ID, mode); err != nil {
			a.errorf("set: %s", err)
			return 1
		}
		pick.Mode = mode.Perm()
	}
	if err := st.Save(); err != nil {
		a.errorf("set: %s", err)
		return 1
	}
	fmt.Fprintln(a.stdout, status("set", fmt.Sprintf("%s (%s, mode %04o)", pick.Target, bindingOf(pick.Host), pick.Mode)))
	return 0
}
