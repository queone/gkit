package main

import (
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"os"
	"strconv"
	"strings"

	"github.com/queone/gkit/internal/help"
	"github.com/sethvargo/go-diceware/diceware"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

const (
	programName    = "pgen"
	programVersion = "1.3.0"
	delimiter      = "_" // Use underscore for diceware password
)

func usage() string {
	return help.Doc{
		Name:        programName,
		Version:     programVersion,
		Description: "Generate memorable passwords from diceware words",
		URL:         help.URL(programName),
		Sections: []help.Section{
			{Title: "Usage", Rows: []help.Row{
				{Form: programName, Meaning: "Print a 3-word phrase, a capitalized variant with a digit, and a 16-character random password"},
				{Form: programName + " NUMBER", Meaning: "Use NUMBER words, from 1 to 9"},
			}},
			{Title: "Examples", Rows: []help.Row{{Form: programName + " 6", Meaning: "A 6-word phrase"}}},
		},
	}.String()
}

// printUsage prints the help and exits 0.
func printUsage() {
	fmt.Print(usage())
	os.Exit(0)
}

// Generate diceware password
func GenerateDiceware(words int) []string {
	list, err := diceware.Generate(words)
	if err != nil {
		log.Fatal(err)
	}
	return list
}

// Create a "strong memorable password" from diceware words
func GenerateStrongMemorable(words []string) string {
	if len(words) == 0 {
		return ""
	}

	parts := make([]string, len(words))
	c := cases.Title(language.English)

	// Capitalize only the first word
	parts[0] = c.String(words[0])
	for i := 1; i < len(words); i++ {
		parts[i] = words[i]
	}

	// Pick a random index for appending a digit
	max := big.NewInt(int64(len(parts)))
	randIndex, err := rand.Int(rand.Reader, max)
	if err != nil {
		panic(err)
	}

	// Append a random digit 0–9
	digit := strconv.Itoa(int(randIndex.Int64()) % 10)
	parts[randIndex.Int64()] += digit

	return strings.Join(parts, "-")
}

// Generate a random alphanumeric password of given length (first char capital consonant)
func GenerateRandomAlphaNumeric(length int) string {
	if length < 1 {
		return ""
	}

	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	const consonants = "BCDFGHJKLMNPQRSTVWXYZ"

	result := make([]byte, length)

	// First character: random capital consonant
	firstIndex, err := rand.Int(rand.Reader, big.NewInt(int64(len(consonants))))
	if err != nil {
		log.Fatal(err)
	}
	result[0] = consonants[firstIndex.Int64()]

	// Remaining characters: random alphanumeric
	buf := make([]byte, length-1)
	_, err = rand.Read(buf)
	if err != nil {
		log.Fatal(err)
	}
	for i := 1; i < length; i++ {
		result[i] = charset[int(buf[i-1])%len(charset)]
	}

	return string(result)
}

func main() {
	args := len(os.Args[1:])
	numWords := 3 // default

	if args == 1 {
		arg1 := os.Args[1]
		switch arg1 {
		case "-v", "--version":
			fmt.Printf("%s v%s\n", programName, programVersion)
			return
		case "-?", "-h", "--help":
			printUsage()
		default:
			n, err := strconv.Atoi(arg1)
			if err != nil || n < 1 || n > 9 {
				fmt.Println("NUMBER must be 1 thru 9.")
				os.Exit(1)
			}
			numWords = n
		}
	}

	// 1. Original diceware password
	dicewareWords := GenerateDiceware(numWords)
	fmt.Println(strings.Join(dicewareWords, delimiter))

	// 2. Strong memorable password
	fmt.Println(GenerateStrongMemorable(dicewareWords))

	// 3. Random alphanumeric password 16 chars
	fmt.Println(GenerateRandomAlphaNumeric(16))
}
