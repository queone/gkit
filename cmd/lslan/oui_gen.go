//go:build ignore

// oui_gen downloads the IEEE MA-L registry and writes oui.tsv.gz, one
// PREFIX<TAB>VENDOR line per prefix, sorted by prefix. Run it with
// go generate in this directory.
package main

import (
	"compress/gzip"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"
)

const (
	registryURL = "https://standards-oui.ieee.org/oui/oui.csv"
	outFile     = "oui.tsv.gz"
)

var prefixRE = regexp.MustCompile(`^[0-9A-F]{6}$`)

func main() {
	req, err := http.NewRequest(http.MethodGet, registryURL, nil)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("User-Agent", "gkit-lslan-oui-generator (github.com/queone/gkit)")
	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("download %s: %v", registryURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Fatalf("download %s: %s", registryURL, resp.Status)
	}
	lines, err := parse(resp.Body)
	if err != nil {
		log.Fatalf("parse %s: %v", registryURL, err)
	}
	if err := write(outFile, lines); err != nil {
		log.Fatalf("write %s: %v", outFile, err)
	}
	fmt.Printf("wrote %d prefixes to %s\n", len(lines), outFile)
}

// parse reads the registry CSV (Registry, Assignment, Organization Name, ...)
// and returns sorted PREFIX<TAB>VENDOR lines.
func parse(r io.Reader) ([]string, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	seen := map[string]bool{}
	var out []string
	for first := true; ; first = false {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if first || len(rec) < 3 || rec[0] != "MA-L" {
			continue
		}
		prefix := strings.ToUpper(strings.TrimSpace(rec[1]))
		vendor := strings.Join(strings.Fields(strings.ReplaceAll(rec[2], "\t", " ")), " ")
		if !prefixRE.MatchString(prefix) || vendor == "" || seen[prefix] {
			continue
		}
		seen[prefix] = true
		out = append(out, prefix+"\t"+vendor)
	}
	slices.Sort(out)
	return out, nil
}

func write(path string, lines []string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	zw, err := gzip.NewWriterLevel(f, gzip.BestCompression)
	if err != nil {
		f.Close()
		return err
	}
	if _, err := io.WriteString(zw, strings.Join(lines, "\n")+"\n"); err != nil {
		f.Close()
		return err
	}
	if err := zw.Close(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
