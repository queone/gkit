# mdv fixture

This paragraph contains ~~removed text~~ and an autolink: https://example.com.

## Task list

- [x] Render Markdown
- [ ] Inspect the result

## Table

| Feature | Status |
| --- | --- |
| GFM | Ready |
| Relative image | Ready |

## Code

```go
fmt.Println("mdv")
```

![Relative fixture](fixture.svg)

<script>alert("raw HTML must not render")</script>

## Diagram

```mermaid
flowchart LR
    Source["Markdown<br/>source"] --> mdv --> Page["Browser<br/>page"]
```

<details>
<summary>Diagram inside details</summary>

```mermaid
sequenceDiagram
    mdv->>Browser: open page
    Browser-->>mdv: tab shown
```

</details>
