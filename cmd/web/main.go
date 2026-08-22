package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/alexflint/go-arg"
	fzf "github.com/koki-develop/go-fzf"
	"github.com/mattn/go-runewidth"
	"github.com/queone/gkit/internal/color"
	"github.com/skratchdot/open-golang/open"
)

const (
	programName    = "web"
	programVersion = "1.0.0"
)

type Options struct {
	Json       bool     `arg:"-j, --json" help:"output results in JSON format"`
	TimeoutSec int      `arg:"-t, --timeout" help:"timeout seconds" env:"DUCKGO_TIMEOUT"`
	UserAgent  string   `arg:"-u, --user-agent" help:"User-Agent value" env:"DUCKGO_USER_AGENT"`
	Referrer   string   `arg:"-r, --referrer" help:"Referrer value" env:"DUCKGO_REFERRER"`
	Browser    string   `arg:"-b, --browser" help:"the command of Web browser to open URL"`
	Query      []string `arg:"positional" help:"keywords to search"`
	Version    bool     `arg:"-v, --version" help:"show version"`
}

func printUsage() {
	n := color.Whi10(programName)
	v := programVersion
	usage := fmt.Sprintf("%s v%s\n"+
		"DuckDuckGo search utility with fuzzy finder\n"+
		"\n"+
		"%s\n"+
		"  %s [options] [query]\n"+
		"\n"+
		"%s\n"+
		"  -j, --json         Output results in JSON format\n"+
		"  -t, --timeout      Timeout in seconds (default: 5)\n"+
		"  -u, --user-agent   Custom User-Agent header\n"+
		"  -r, --referrer     Custom Referrer header\n"+
		"  -b, --browser      Browser command to open URLs\n"+
		"  -v, --version      Show version and exit\n"+
		"\n"+
		"%s\n"+
		"  %s golang\n"+
		"  %s -j golang\n"+
		"  %s -t 10 -b firefox golang\n",
		n, v, color.Whi10("Usage"), n, color.Whi10("Options"), color.Whi10("Examples"), n, n, n)
	fmt.Print(usage)
}

func parseArgs(args []string) (*Options, error) {
	var opts Options

	p, err := arg.NewParser(arg.Config{
		Program:   programName,
		IgnoreEnv: false,
	}, &opts)
	if err != nil {
		return &opts, err
	}

	if err := p.Parse(args); err != nil {
		switch {
		case errors.Is(err, arg.ErrHelp):
			printUsage()
			return &opts, nil
		case errors.Is(err, arg.ErrVersion):
			fmt.Fprintf(os.Stdout, "%s v%s\n", programName, programVersion)
			return &opts, nil
		default:
			return &opts, err
		}
	}

	return &opts, nil
}

func run(opts *Options) error {
	param, err := NewSearchParam(strings.Join(opts.Query, " "))
	if err != nil {
		return err
	}

	result, err := SearchWithOption(param, &ClientOption{
		Timeout:   time.Duration(opts.TimeoutSec) * time.Second,
		UserAgent: opts.UserAgent,
		Referrer:  opts.Referrer,
	})
	if err != nil {
		return err
	}

	if opts.Json {
		if err := json.NewEncoder(os.Stdout).Encode(&result); err != nil {
			return err
		}

		return nil
	}

	selected, err := find(*result)
	if err != nil {
		return err
	}

	for _, idx := range selected {
		if opts.Browser == "" {
			if err := open.Run(((*result)[idx]).Link); err != nil {
				return err
			}

			return nil
		} else {
			if err := open.RunWith((*result)[idx].Link, opts.Browser); err != nil {
				return err
			}

			return nil
		}
	}

	return nil
}

func find(result []SearchResult) ([]int, error) {
	f, err := fzf.New(
		fzf.WithInputPlaceholder("Filter..."),
	)
	if err != nil {
		panic(err)
	}

	return f.Find(
		result,
		func(i int) string {
			return result[i].Title
		},
		fzf.WithPreviewWindow(func(idx int, width int, height int) string {
			content := fmt.Sprintf(
				"\n\n%s\n\n%s\n\n%s\n",
				result[idx].Title, result[idx].Snippet, result[idx].Link,
			)

			return runewidth.Wrap(content, width-2)
		}),
	)
}

func main() {
	opts, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if opts.Version {
		fmt.Fprintf(os.Stdout, "%s v%s\n", programName, programVersion)
		return
	}

	if err := run(opts); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
