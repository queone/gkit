package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	goyaml "github.com/goccy/go-yaml"
	"github.com/mattn/go-isatty"
	icolor "github.com/queone/gkit/internal/color"
	"gopkg.in/yaml.v3"
)

const (
	programName    = "jy"
	programVersion = "1.6.0"
)

// die prints an error message to stderr and exits with status 1.
func die(format string, args ...any) {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, format)
	} else {
		fmt.Fprintf(os.Stderr, format, args...)
	}
	os.Exit(1)
}

// fileUsable reports whether filePath exists and has content.
func fileUsable(filePath string) bool {
	info, err := os.Stat(filePath)
	if err != nil {
		return false
	}
	return info.Size() > 0
}

// loadFileText reads filePath into a byte slice.
func loadFileText(filePath string) (rawBytes []byte, err error) {
	rawBytes, err = os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	return rawBytes, nil
}

// loadFileYamlBytes reads filePath as YAML (or JSON) bytes after validating with goccy/go-yaml.
func loadFileYamlBytes(filePath string) (yamlBytes []byte, err error) {
	yamlBytes, err = os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	var yamlObject any
	err = goyaml.Unmarshal(yamlBytes, &yamlObject)
	if err != nil {
		return nil, err
	}
	return yamlBytes, nil
}

func printUsage() {
	n := icolor.Whi10(programName)
	v := programVersion
	usage := fmt.Sprintf("%s v%s\n"+
		"JSON / YAML converter - https://github.com/queone/gkit/blob/main/cmd/jy/README.md\n"+
		"%s\n"+
		"  %s [options] [file]\n"+
		"\n"+
		"  Options can be specified in any order. The file can be piped into the utility, or it\n"+
		"  can be referenced as an argument. If the file is YAML, the output will be JSON, or\n"+
		"  vice versa.\n"+
		"\n"+
		"%s\n"+
		"  -c                     Colorize the output for the specified file.\n"+
		"  -d                     Decolorize the output for piped input or file.\n"+
		"  -v, --version          Print version and exit.\n"+
		"  -?, --help, -h         Show this help message and exit.\n"+
		"\n"+
		"%s\n"+
		"  cat file | %s\n"+
		"  %s /path/to/file\n"+
		"  %s /path/to/file -d\n"+
		"  %s file.yaml -c        Prints a colorized version of the file. Does not convert.\n"+
		"  %s -h\n",
		n, v, icolor.Whi10("Usage"), n, icolor.Whi10("Options"), icolor.Whi10("Examples"), n, n, n, n, n)
	fmt.Print(usage)
	os.Exit(0)
}

func isGitBashOnWindows() bool {
	return runtime.GOOS == "windows" && strings.HasPrefix(os.Getenv("MSYSTEM"), "MINGW")
}

func hasPipedInput() bool {
	stat, _ := os.Stdin.Stat() // Check if anything was piped in
	if isGitBashOnWindows() {
		// Git Bash on Windows handles input redirection differently than other shells. When a program
		// is run without any input or arguments, it still treats the input as if it were piped from an
		// empty stream, causing the program to consider it as piped input and hang. This works around that.
		if !isatty.IsCygwinTerminal(os.Stdin.Fd()) {
			return true
		}
	} else {
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			return true
		}
	}
	return false
}

func printOut(rawBytes []byte, option string) {
	// Check if raw bytes are either a JSON or YAML object
	// JSON must be checked first because it is a subset of the YAML standard
	var rawObject any
	if err := json.Unmarshal(rawBytes, &rawObject); err == nil && rawObject != nil {
		// It is JSON, print in YAML
		if option == "decolor_output" {
			if err := printYaml(rawObject); err != nil {
				die("yaml encode: %v\n", err)
			}
		} else {
			if err := printYamlColor(rawObject); err != nil {
				die("yaml encode: %v\n", err)
			}
		}
		return
	}

	// Try YAML
	rawObject = nil
	if err := yaml.Unmarshal(rawBytes, &rawObject); err != nil || rawObject == nil {
		die("Not JSON nor YAML\n")
	}
	// It is YAML, print in JSON
	jsonBytes, err := goyaml.YAMLToJSON(rawBytes)
	if err != nil {
		die("yaml→json: %v\n", err)
	}
	jsonBytes2, err := jsonBytesReindent(jsonBytes, 2) // Two space indent
	if err != nil {
		die("json reindent: %v\n", err)
	}
	if option == "decolor_output" {
		jsonObj, err := jsonBytesToJsonObj(jsonBytes2)
		if err != nil {
			die("json decode: %v\n", err)
		}
		printJson(jsonObj)
	} else {
		printJsonBytesColor(jsonBytes2)
	}
}

func processPipedInput(option string) {
	// Read piped input and convert to decolorized raw bytes
	rawBytes, err := io.ReadAll(os.Stdin)
	if err != nil {
		die("read stdin: %v\n", err)
	}

	// Remove color codes in piped input
	stringSansColor := icolor.ClearCode(string(rawBytes))
	rawBytes = []byte(stringSansColor)

	printOut(rawBytes, option)
}

func processFileInput(filePath, option string) {
	// Read file input and convert to decolorized raw bytes
	if !fileUsable(filePath) {
		die("File is unusable\n")
	}

	rawBytes, err := loadFileText(filePath)
	if err != nil {
		die("Couln't read file.\n")
	}

	// Remove color codes in file
	stringSansColor := icolor.ClearCode(string(rawBytes))
	rawBytes = []byte(stringSansColor)

	printOut(rawBytes, option)
}

func printInColor(filePath string) {
	// Print given JSON or YAML file in color
	// JSON must be checked first because it is a subset of the YAML standard
	jsonBytes, err := loadFileYamlBytes(filePath)
	if err == nil {
		printJsonBytesColor(jsonBytes) // Print colorized JSON
	} else {
		// Load as raw YAML byte slice that can include comments
		yamlBytes, err := loadFileYamlBytes(filePath)
		if err == nil {
			printYamlBytesColor(yamlBytes) // Print colorized YAML
		} else {
			die("File is neither JSON nor YAML\n")
		}
	}
}

func main() {
	var filePath string
	var decolorize bool
	var colorize bool

	args := os.Args[1:] // Get all command-line arguments excluding the program name
	if len(args) > 0 {
		for _, arg := range args {
			switch arg {
			case "-d":
				decolorize = true // Set the decolorize flag
			case "-c":
				colorize = true // Set the colorize flag
			case "-v", "--version":
				fmt.Printf("%s v%s\n", programName, programVersion)
				return
			case "-?", "--help", "-h":
				printUsage()
				return
			default:
				// Treat any other argument as a file path
				filePath = arg
			}
		}
	}

	// If a file path was provided, process it
	if filePath != "" {
		if colorize {
			// Print the file in its current format with colorization
			printInColor(filePath)
			return
		} else if decolorize {
			processFileInput(filePath, "decolor_output") // Process with decolorization
			return
		} else {
			processFileInput(filePath, "") // Process normally (convert format)
			return
		}
	} else if hasPipedInput() {
		// If there is piped input, check if we need to decolorize or colorize
		if decolorize {
			processPipedInput("decolor_output") // Process piped input with decolorization
		} else {
			processPipedInput("") // Process piped input normally
		}
	} else {
		printUsage()
	}
}
