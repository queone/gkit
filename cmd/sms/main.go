package main

// The textbelt.com website mentions below is all you really need for the Go calls:
// import (
//   "net/http"
//   "net/url"
// )

// func main() {
//   values := url.Values{
//     "phone": {"5555555555"},
//     "message": {"Hello world"},
//     "key": {"textbelt"},
//   }

//   http.PostForm("https://textbelt.com/text", values)
// }

import (
	"fmt"
	"github.com/queone/gkit/internal/help"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"syscall"

	"github.com/vaughan0/go-ini"
)

const (
	programName    = "sms"
	programVersion = "1.4.0"
)

// Global variables
var (
	svckey = "textbelt"
	svcurl = "https://textbelt.com/text"
)

// usageText returns the help message body. Pure function so tests can
// inspect output without intercepting stdout or hitting os.Exit.
func usageText() string {
	return help.Doc{
		Name:        programName,
		Version:     programVersion,
		Description: "Send an SMS message through textbelt.com",
		URL:         help.URL(programName),
		Sections: []help.Section{
			{Title: "Usage", Rows: []help.Row{
				{Form: programName + " NUMBER MESSAGE", Meaning: "Send MESSAGE to NUMBER with the key in ~/.config/" + programName + "/config.ini"},
				{Form: programName + " -y", Meaning: "Create a skeleton ~/.config/" + programName + "/config.ini"},
			}, Lines: []string{"Visit https://textbelt.com for more info."}},
			{Title: "Options", Rows: []help.Row{{Form: "-y", Meaning: "Write the skeleton config file and exit"}}},
		},
	}.String()
}

// Print usage information
func printUsage() {
	fmt.Print(usageText())
	os.Exit(0)
}

// xdgConfigDir returns the user's XDG config directory, honoring
// XDG_CONFIG_HOME when set to an absolute path and falling back to
// $HOME/.config otherwise.
func xdgConfigDir() (string, error) {
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" && filepath.IsAbs(v) {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config"), nil
}

// configPath returns the canonical config file path for sms, performing a
// one-shot lazy migration from the legacy $HOME/.smsrc location on first call.
// Returns the new XDG path in all cases (after migration if applicable).
func configPath() (string, error) {
	cfgDir, err := xdgConfigDir()
	if err != nil {
		return "", err
	}
	newPath := filepath.Join(cfgDir, programName, "config.ini")

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	oldPath := filepath.Join(home, "."+programName+"rc")

	if err := migrateIfNeeded(oldPath, newPath, 0600); err != nil {
		fmt.Fprintf(os.Stderr, "%s: migration warning: %v\n", programName, err)
	}
	return newPath, nil
}

// migrateIfNeeded moves oldPath to newPath when oldPath is a regular file and
// newPath does not exist. Symlinks at oldPath are preserved with a warning.
// When both paths exist, the new path is preferred and a warning is emitted.
// On successful migration the destination is chmod'd to mode.
func migrateIfNeeded(oldPath, newPath string, mode os.FileMode) error {
	oldInfo, oldErr := os.Lstat(oldPath)
	_, newErr := os.Stat(newPath)
	newExists := newErr == nil

	if oldErr != nil {
		return nil // nothing to migrate
	}

	if oldInfo.Mode()&os.ModeSymlink != 0 {
		fmt.Fprintf(os.Stderr, "%s: %s is a symlink; skipping auto-migration. Move it to %s manually.\n", programName, oldPath, newPath)
		return nil
	}

	if newExists {
		fmt.Fprintf(os.Stderr, "%s: both %s and %s exist; using %s. Delete the old file when ready.\n", programName, oldPath, newPath, newPath)
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(newPath), 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	if err := os.Rename(oldPath, newPath); err != nil {
		// Cross-device rename (EXDEV) — fall back to copy + delete.
		if linkErr, ok := err.(*os.LinkError); ok && linkErr.Err == syscall.EXDEV {
			if err := copyFile(oldPath, newPath); err != nil {
				return fmt.Errorf("cross-device migration copy: %w", err)
			}
			if err := os.Remove(oldPath); err != nil {
				return fmt.Errorf("cross-device migration cleanup: %w", err)
			}
		} else {
			return fmt.Errorf("rename: %w", err)
		}
	}

	if err := os.Chmod(newPath, mode); err != nil {
		return fmt.Errorf("chmod: %w", err)
	}

	fmt.Fprintf(os.Stderr, "%s: migrated %s -> %s\n", programName, oldPath, newPath)
	return nil
}

// copyFile copies src to dst (used as EXDEV fallback for os.Rename).
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}

// Set up global variables as per values in configuration file
func processConfigFile() {
	cfgfile, err := configPath()
	if err != nil {
		fmt.Printf("Error. Cannot resolve config path: %v\n", err)
		os.Exit(1)
	}
	if _, err := os.Stat(cfgfile); os.IsNotExist(err) {
		fmt.Printf("Error. Missing '%s' file. Run '%s -y' to create a new one.\n", cfgfile, programName)
		os.Exit(1)
	}

	f, _ := ini.LoadFile(cfgfile)
	v1, _ := f.Get("global", "svcurl")
	if v1 == "" {
		fmt.Printf("Error. svcurl not defined in '%s' file.\n", cfgfile)
		os.Exit(1)
	}
	svcurl = v1

	v2, _ := f.Get("global", "svckey")
	if v2 == "" {
		fmt.Printf("Error. svckey not defined in '%s' file.\n", cfgfile)
		os.Exit(1)
	}
	svckey = v2
}

// Create a skeleton configuration file with default hard-coded values
func createSkeletonConfigFile() {
	cfgfile, err := configPath()
	if err != nil {
		fmt.Printf("Error. Cannot resolve config path: %v\n", err)
		os.Exit(1)
	}

	// Check if file already exists
	if _, err := os.Stat(cfgfile); err == nil {
		fmt.Printf("There's already a '%s' file.\n", cfgfile)
		return
	} else if !os.IsNotExist(err) {
		panic(err.Error())
	}

	if err := os.MkdirAll(filepath.Dir(cfgfile), 0755); err != nil {
		panic(err.Error())
	}

	// Build configuration file content
	content := "# Edit below values accordingly\n"
	content += "[global]\n"
	content += "svcurl = https://textbelt.com/text\n"
	content += "svckey = textbelt\n"

	// Create the file with 0600 (holds an API key)
	f, err := os.OpenFile(cfgfile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		panic(err.Error())
	}
	defer f.Close()

	if _, err := f.Write([]byte(content)); err != nil {
		panic(err.Error())
	}
}

func main() {

	tel, msg := "", ""
	argCount := len(os.Args[1:])

	if argCount < 1 || argCount > 2 {
		printUsage()
	}

	if argCount == 1 {
		switch os.Args[1] {
		case "-v", "--version":
			fmt.Printf("%s v%s\n", programName, programVersion)
			return
		case "-?", "-h", "--help":
			printUsage()
		case "-y":
			createSkeletonConfigFile()
			return
		default:
			printUsage()
		}
	}

	processConfigFile()

	tel = os.Args[1]
	msg = os.Args[2]

	values := url.Values{
		"key":     {svckey},
		"phone":   {tel},
		"message": {msg},
	}
	fmt.Printf("%s  %s  %s\n", svckey, tel, msg)
	resp, err := http.PostForm(svcurl, values)
	if err != nil {
		panic(err)
	}
	if resp.StatusCode != 200 {
		fmt.Printf("Error. HTTP error code = %s\n", resp.Status)
		os.Exit(1)
	}
	os.Exit(0)
}

// func main() {
// 	args := len(os.Args[1:])
// 	numWords := 3 // default

// 	if args == 1 {
// 		arg1 := os.Args[1]
// 		switch arg1 {
// 		case "-?", "-h", "--help":
// 			printUsage()
// 		default:
// 			n, err := strconv.Atoi(arg1)
// 			if err != nil || n < 1 || n > 9 {
// 				fmt.Println("NUMBER must be 1 thru 9.")
// 				os.Exit(1)
// 			}
// 			numWords = n
// 		}
// 	}

// 	// 1. Original diceware password
// 	dicewareWords := GenerateDiceware(numWords)
// 	fmt.Println(strings.Join(dicewareWords, delimiter))

// 	// 2. Strong memorable password
// 	fmt.Println(GenerateStrongMemorable(dicewareWords))

// 	// 3. Random alphanumeric password 16 chars
// 	fmt.Println(GenerateRandomAlphaNumeric(16))
// }
