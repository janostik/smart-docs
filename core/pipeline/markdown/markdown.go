package markdown

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"

	"github.com/yuin/goldmark"
)

// RemoveLatexTags removes LaTeX commands while keeping their content
func RemoveLatexTags(input string) string {
	// 1. Remove LaTeX environments like \begin{aligned} ... \end{aligned}
	reEnv := regexp.MustCompile(`\\begin\{[a-zA-Z*]+\}([\s\S]*?)\\end\{[a-zA-Z*]+\}`)
	output := reEnv.ReplaceAllString(input, "$1")

	// 2. Remove LaTeX commands like \mathrm{mg} but keep the content inside {}
	reCmd := regexp.MustCompile(`\\[a-zA-Z]+\{([^}]*)\}`)
	output = reCmd.ReplaceAllString(output, "$1")

	// 3. Remove inline math delimiters ($...$)
	reDollar := regexp.MustCompile(`\$(.*?)\$`)
	output = reDollar.ReplaceAllStringFunc(output, func(match string) string {
		return strings.Trim(match, "$") // Remove surrounding $
	})

	// 4. Remove unnecessary LaTeX-related backslashes (like "\ &") and extra spaces
	output = regexp.MustCompile(`\\\s*&?`).ReplaceAllString(output, " ")

	// 5. Remove inline LaTeX brackets \( ... \) and \[ ... \]
	output = strings.ReplaceAll(output, `\(`, "")
	output = strings.ReplaceAll(output, `\)`, "")
	output = strings.ReplaceAll(output, `\[`, "")
	output = strings.ReplaceAll(output, `\]`, "")

	// Trim extra spaces
	output = strings.TrimSpace(output)

	return output
}

// ConvertMarkdownToHTML converts a markdown string to HTML
func ConvertMarkdownToHTML(markdown string) (string, error) {
	var buf bytes.Buffer
	md := goldmark.New(
		goldmark.WithExtensions(extension.Table),
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
		))

	if err := md.Convert([]byte(markdown), &buf); err != nil {
		return "", err
	}
	return RemoveLatexTags(buf.String()), nil
}
