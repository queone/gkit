## mdview

Render a local Markdown file as GitHub Flavored Markdown and open it in the default browser.

### Usage

```bash
mdview [-o FILE] FILE
```

```text
-o, --output FILE  write HTML to FILE without opening a browser
-v, --version      show the full help screen and exit
-h, -?, --help     show the full help screen and exit
```

Use `--` before an input filename that begins with `-`.

### Examples

```bash
mdview README.md
mdview -o README.html README.md
mdview -- --notes.md
```

Without `-o`, `mdview` writes a protected `mdview-*.html` file in the operating system's temporary directory and opens it in the default browser. Successful temporary files remain until the operating system or the user removes them.

With `-o`, `mdview` writes the named file and does not open a browser. The parent directory must already exist. `mdview` refuses to overwrite any existing destination.

### Rendering

`mdview` supports GitHub Flavored Markdown tables, strikethrough, task lists, and autolinks. It embeds `github-markdown-css`, follows the system light or dark color scheme, and needs no network connection or local server at runtime.

`<details>` and `<summary>` are the only supported raw HTML disclosure elements. They are rendered as collapsed-by-default controls, and Markdown—including GFM tables—continues to render inside them. Disclosure attributes are discarded, so an input `open` attribute does not open the control. Other raw HTML elements, disclosure markup outside a `<details>` block, malformed disclosure markup, event-handler and style attributes, and unsafe URLs are omitted or neutralized. Disclosure tags inside code fences and inline code remain code literals.

Relative links and images resolve from the Markdown source's directory. For a symlinked input, they resolve from the target file's directory. Linked resources remain separate files and are not copied into the HTML output.

Raw HTML is omitted. GitHub-specific server features such as Mermaid diagrams, issue references, emoji expansion, and repository-aware links are not supported. Code fences are styled but do not receive syntax highlighting.

### Getting Started

This utility is part of a collection of Go utilities. To compile and install, follow the **Getting Started** instructions in the [utils repository](https://github.com/queone/utils).
