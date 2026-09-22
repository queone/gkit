## mdv

Render local Markdown files as GitHub Flavored Markdown and open each one in the default browser.

### Usage

```text
mdv v0.3.0
View GitHub Flavored Markdown in a browser or write it as HTML
github.com/queone/gkit/tree/main/cmd/mdv

Usage
  mdv FILE...       Render each FILE and open it in its own browser tab
  mdv -o PATH FILE  Write FILE as HTML to PATH without opening a browser

Options
  -o, --output PATH  Write the HTML to PATH without opening a browser
  -v, --version      Print mdv v0.3.0 and exit
  -h, -?, --help     Show this help and exit
```

Use `--` before an input filename that begins with `-`.

### Examples

```bash
mdv README.md
mdv README.md CHANGELOG.md plan.md
mdv *.md
mdv -o README.html README.md
mdv -- --notes.md
```

Without `-o`, `mdv` checks, reads, and renders every FILE before it opens anything. Each FILE gets its own protected `mdv-*.html` page in the operating system's temporary directory, and the pages open in the default browser in command-line order, one tab each. Duplicate FILE arguments open duplicate tabs. Successful temporary files remain until the operating system or the user removes them.

If any FILE is missing, unreadable, or fails to render, `mdv` opens no tab, removes the pages it made, and names the failing FILE. If the browser fails to open a page, `mdv` stops there and removes that page and the pages not yet opened. Tabs that already opened stay open.

With `-o`, `mdv` writes one FILE to PATH and does not open a browser. `-o` takes exactly one FILE. The parent directory must already exist. `mdv` refuses to overwrite any existing destination.

### Rendering

`mdv` supports GitHub Flavored Markdown tables, strikethrough, task lists, and autolinks. It embeds `github-markdown-css`, follows the system light or dark color scheme, and needs no network connection or local server at runtime.

`<details>` and `<summary>` are the only supported raw HTML disclosure elements. They are rendered as collapsed-by-default controls, and Markdown—including GFM tables—continues to render inside them. Disclosure attributes are discarded, so an input `open` attribute does not open the control. Other raw HTML elements, disclosure markup outside a `<details>` block, malformed disclosure markup, event-handler and style attributes, and unsafe URLs are omitted or neutralized. Disclosure tags inside code fences and inline code remain code literals.

Relative links and images resolve from the Markdown source's directory. For a symlinked input, they resolve from the target file's directory. Linked resources remain separate files and are not copied into the HTML output.

Raw HTML is omitted. GitHub-specific server features such as Mermaid diagrams, issue references, emoji expansion, and repository-aware links are not supported. Code fences are styled but do not receive syntax highlighting.

### Getting Started

This utility is part of a collection of Go utilities. To compile and install, follow the **Getting Started** instructions in the [gkit repository](https://github.com/queone/gkit).
