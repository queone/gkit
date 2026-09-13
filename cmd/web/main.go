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
	"github.com/queone/gkit/internal/help"
	"github.com/skratchdot/open-golang/open"
)

const (
	programName    = "web"
	programVersion = "1.1.0"
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

func usage() string {
	return help.Doc{
		Name:        programName,
		Version:     programVersion,
		Description: "Search DuckDuckGo and open a result picked with a fuzzy finder",
		URL:         help.URL(programName),
		Sections: []help.Section{
			{Title: "Usage", Rows: []help.Row{{Form: programName + " [options] QUERY...", Meaning: "Search, pick a result, and open it in the browser"}}},
			{Title: "Options", Rows: []help.Row{
				{Form: "-j, --json", Meaning: "Print the results as JSON instead of opening one"},
				{Form: "-t, --timeout N", Meaning: "Give up after N seconds (default 5)"},
				{Form: "-u, --user-agent UA", Meaning: "Send UA as the User-Agent header"},
				{Form: "-r, --referrer URL", Meaning: "Send URL as the Referer header"},
				{Form: "-b, --browser CMD", Meaning: "Open the result with CMD instead of the default browser"},
			}},
			{Title: "Examples", Rows: []help.Row{
				{Form: programName + " golang", Meaning: ""},
				{Form: programName + " -j golang", Meaning: ""},
				{Form: programName + " -t 10 -b firefox golang", Meaning: ""},
			}},
		},
	}.String()
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
			fmt.Print(usage())
			os.Exit(0)
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
	for _, a := range os.Args[1:] {
		if a == "-h" || a == "-?" || a == "--help" {
			fmt.Print(usage())
			return
		}
	}
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
