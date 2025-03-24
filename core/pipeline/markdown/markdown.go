package markdown

import (
	"bytes"
	"github.com/yuin/goldmark/renderer/html"

	"github.com/yuin/goldmark"
)

// ConvertMarkdownToHTML converts a markdown string to HTML
func ConvertMarkdownToHTML(markdown string) (string, error) {
	var buf bytes.Buffer
	md := goldmark.New(
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
		))

	if err := md.Convert([]byte(markdown), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}
