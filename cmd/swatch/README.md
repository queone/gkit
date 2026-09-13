# swatch
Xterm 256-color palette and ramp inspector — prints the complete xterm palette plus 11 named color ramps (Gra, Red, Org, Yel, Grn, Cya, Blu, Pur, Mag, Whi, Heat), each as a bordered grid or background-swatch strip.

### Usage

```text
swatch v1.1.0
Inspect the xterm 256-color palette and the color ramps
github.com/queone/gkit/tree/main/cmd/swatch

Usage
  swatch COMMAND [options] [TOKEN]  Print one view of the palette, with TOKEN as the sample text

  TOKEN defaults to TOKEN when omitted or empty. Grid --foreground requires --reverse.

Commands
  p, palette      Print the complete xterm palette and color ramps
  g, grid         Print the bordered ramp-by-step grid
  b, backgrounds  Print background-ramp swatch rows
  version         Print swatch v1.1.0
  help            Show this help

Options
  -r, --reverse           Use ramp colors as grid cell backgrounds
  -f, --foreground INDEX  Set grid or swatch text to xterm INDEX (0-255)
  -v, --version           Print swatch v1.1.0 and exit
  -h, -?, --help          Show this help and exit

Examples
  swatch p
  swatch palette
  swatch g --reverse HEADER
  swatch b --foreground 15 LABEL
```

Color output is gated on terminal capability and `NO_COLOR`; set `NO_COLOR=1`, use `TERM=dumb`, or redirect output for plain text.
