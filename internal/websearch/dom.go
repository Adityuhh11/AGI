package websearch

import (
	"slices"
	"strings"

	"golang.org/x/net/html"
)

// walk calls visit for n and then, if it returns true, for everything under it.
func walk(n *html.Node, visit func(*html.Node) bool) {
	if !visit(n) {
		return
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walk(c, visit)
	}
}

// find returns the first node under n (n included) that match accepts.
func find(n *html.Node, match func(*html.Node) bool) *html.Node {
	var found *html.Node
	walk(n, func(x *html.Node) bool {
		if found != nil {
			return false
		}
		if x.Type == html.ElementNode && match(x) {
			found = x
			return false
		}
		return true
	})
	return found
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func hasClass(n *html.Node, class string) bool {
	return n.Type == html.ElementNode && slices.Contains(strings.Fields(attr(n, "class")), class)
}

// textOf is all the text under n.
func textOf(n *html.Node) string {
	var b strings.Builder
	walk(n, func(x *html.Node) bool {
		if x.Type == html.TextNode {
			b.WriteString(x.Data)
		}
		return true
	})
	return b.String()
}

// squash turns every run of white space into one space.
func squash(s string) string { return strings.Join(strings.Fields(s), " ") }
