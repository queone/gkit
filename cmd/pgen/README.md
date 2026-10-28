## pgen
Memorable password generator a la [xkcd.com/936](https://xkcd.com/936/), using [sethvargo/go-diceware](https://github.com/sethvargo/go-diceware).

### Why
Generates strong passwords that are easy to remember and type, in the style of xkcd 936.

### Usage

```text
pgen v1.4.0
Generate memorable passwords from diceware words
github.com/queone/gkit/tree/main/cmd/pgen

Usage
  pgen         Print a 3-word phrase, a capitalized variant with a digit, and a 16-character random password
  pgen NUMBER  Use NUMBER words, from 1 to 9

Options
  -v, --version   Print pgen v1.4.0 and exit
  -h, -?, --help  Show this help and exit

Examples
  pgen 6  A 6-word phrase
```
