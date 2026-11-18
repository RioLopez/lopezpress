package services

import (
	"net/url"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// --- server-side HTML sanitizer ---------------------------------------------
//
// HTML posts are authored by root/admin only, but that is a permission rule,
// not a rendering rule: the API returns the stored body to every reader, and
// RSS / backups / future clients do not necessarily run the SPA's sanitizer.
// The bytes that land in posts.body must therefore already be safe.
//
// The allowlist mirrors web/src/core/md.ts (DROP_TAGS / DROP_ATTRS / URL
// policy) so the server and the reader agree on what an HTML post may contain.
// The renderer keeps its own pass as defence in depth; this one is the
// authoritative gate.
//
// Parsing uses golang.org/x/net/html rather than a regex. Regex tag stripping
// is exactly how "safe" sanitizers get bypassed: malformed nesting, attribute
// quoting and entity tricks all survive a pattern match and are resolved by
// the browser's parser. We parse with the same grammar family the browser uses
// and rebuild only the nodes on the allowlist.

// htmlDropSubtree lists elements that are removed together with their
// contents. Anything not in htmlAllowedTags but also not here is unwrapped:
// its children survive, the tag does not.
var htmlDropSubtree = map[string]bool{
	"script": true, "style": true, "iframe": true, "object": true,
	"embed": true, "svg": true, "math": true, "template": true,
	"noscript": true, "form": true, "input": true, "button": true,
	"select": true, "option": true, "textarea": true, "link": true,
	"meta": true, "base": true, "frame": true, "frameset": true,
	"applet": true, "title": true,
}

// htmlAllowedTags is the content vocabulary. It deliberately excludes
// script/style/iframe/form, and also svg/math (foreign content has its own
// parser paths and its own history of sanitizer bypasses).
var htmlAllowedTags = map[string]bool{
	"a": true, "abbr": true, "audio": true, "b": true, "blockquote": true,
	"br": true, "caption": true, "cite": true, "code": true, "col": true,
	"colgroup": true, "dd": true, "del": true, "details": true, "dfn": true,
	"div": true, "dl": true, "dt": true, "em": true, "figcaption": true,
	"figure": true, "h1": true, "h2": true, "h3": true, "h4": true,
	"h5": true, "h6": true, "hr": true, "i": true, "img": true, "ins": true,
	"kbd": true, "li": true, "mark": true, "ol": true, "p": true,
	"picture": true, "pre": true, "q": true, "rp": true, "rt": true,
	"ruby": true, "s": true, "samp": true, "small": true, "source": true,
	"span": true, "strike": true, "strong": true, "sub": true, "summary": true,
	"sup": true, "table": true, "tbody": true, "td": true, "tfoot": true,
	"th": true, "thead": true, "time": true, "tr": true, "track": true,
	"u": true, "ul": true, "var": true, "video": true, "wbr": true,
}

var htmlVoidTags = map[string]bool{
	"br": true, "col": true, "hr": true, "img": true, "source": true,
	"track": true, "wbr": true,
}

var htmlGlobalAttrs = map[string]bool{
	"class": true, "id": true, "title": true, "lang": true, "dir": true,
}

// htmlTagAttrs is the per-tag attribute allowlist. Event handlers (on*) and
// inline style are not present anywhere, so they can never survive.
var htmlTagAttrs = map[string]map[string]bool{
	"a":          {"href": true, "hreflang": true},
	"img":        {"src": true, "alt": true, "width": true, "height": true, "loading": true, "srcset": true, "sizes": true},
	"source":     {"src": true, "srcset": true, "sizes": true, "type": true, "media": true},
	"video":      {"src": true, "poster": true, "width": true, "height": true, "controls": true, "autoplay": true, "loop": true, "muted": true, "playsinline": true, "preload": true},
	"audio":      {"src": true, "controls": true, "autoplay": true, "loop": true, "muted": true, "preload": true},
	"track":      {"src": true, "kind": true, "srclang": true, "label": true, "default": true},
	"ol":         {"start": true, "reversed": true, "type": true},
	"li":         {"value": true},
	"td":         {"colspan": true, "rowspan": true, "headers": true},
	"th":         {"colspan": true, "rowspan": true, "headers": true, "scope": true, "abbr": true},
	"time":       {"datetime": true},
	"details":    {"open": true},
	"del":        {"cite": true, "datetime": true},
	"ins":        {"cite": true, "datetime": true},
	"blockquote": {"cite": true},
	"q":          {"cite": true},
	"col":        {"span": true},
	"colgroup":   {"span": true},
}

// htmlURLAttrs maps an attribute to the policy that governs it. href accepts
// mailto; fetched resources (src/poster) do not.
var htmlURLAttrs = map[string]string{
	"href":   "href",
	"cite":   "href",
	"src":    "src",
	"poster": "src",
	"srcset": "srcset",
}

// SanitizeHTML parses an HTML fragment and returns the substring of it that
// the allowlist permits. The result is canonical (re-serialized), so running
// it twice produces the same text.
func SanitizeHTML(src string) (string, error) {
	ctx := &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div}
	nodes, err := html.ParseFragment(strings.NewReader(src), ctx)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, n := range nodes {
		writeSanitizedNode(&b, n)
	}
	return b.String(), nil
}

func writeSanitizedNode(b *strings.Builder, n *html.Node) {
	switch n.Type {
	case html.TextNode:
		b.WriteString(html.EscapeString(n.Data))
	case html.ElementNode:
		tag := strings.ToLower(n.Data)
		if htmlDropSubtree[tag] {
			return
		}
		if !htmlAllowedTags[tag] {
			// Unwrap unknown but harmless elements (font, center, custom
			// inline tags): their text belongs to the article.
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				writeSanitizedNode(b, c)
			}
			return
		}
		if tag == "img" && !hasSafeImageSource(n) {
			return
		}
		b.WriteString("<")
		b.WriteString(tag)
		writeSanitizedAttrs(b, tag, n.Attr)
		b.WriteString(">")
		if htmlVoidTags[tag] {
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			writeSanitizedNode(b, c)
		}
		b.WriteString("</")
		b.WriteString(tag)
		b.WriteString(">")
	case html.DocumentNode:
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			writeSanitizedNode(b, c)
		}
	}
}

func writeSanitizedAttrs(b *strings.Builder, tag string, attrs []html.Attribute) {
	allow := htmlTagAttrs[tag]
	for _, a := range attrs {
		name := strings.ToLower(a.Key)
		if name == "" || strings.HasPrefix(name, "on") {
			continue
		}
		if !htmlGlobalAttrs[name] && !(allow != nil && allow[name]) {
			continue
		}
		value := a.Val
		switch htmlURLAttrs[name] {
		case "href":
			if !isSafeHTMLURL(value, true) {
				continue
			}
		case "src":
			if !isSafeHTMLURL(value, false) {
				continue
			}
		case "srcset":
			if !isSafeHTMLSrcset(value) {
				continue
			}
		}
		b.WriteString(" ")
		b.WriteString(name)
		b.WriteString(`="`)
		b.WriteString(html.EscapeString(value))
		b.WriteString(`"`)
	}
}

// hasSafeImageSource keeps an <img> only when it has a source the browser may
// actually fetch. A blank/broken image is noise; an unsafe one is a leak.
func hasSafeImageSource(n *html.Node) bool {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, "src") && isSafeHTMLURL(a.Val, false) && strings.TrimSpace(a.Val) != "" {
			return true
		}
	}
	return false
}

// isSafeHTMLURL accepts relative references and the http(s) schemes, plus
// mailto when allowMail is set. It rejects protocol-relative URLs (//host),
// backslash forms, control characters and every other scheme (javascript:,
// data:, vbscript:).
func isSafeHTMLURL(raw string, allowMail bool) bool {
	v := strings.TrimSpace(raw)
	if v == "" {
		return true
	}
	if strings.HasPrefix(v, "//") || strings.HasPrefix(v, "\\") {
		return false
	}
	for _, r := range v {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	u, err := url.Parse(v)
	if err != nil {
		return false
	}
	if u.Scheme == "" {
		return true
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		return true
	case "mailto":
		return allowMail
	default:
		return false
	}
}

func isSafeHTMLSrcset(v string) bool {
	if strings.TrimSpace(v) == "" {
		return true
	}
	for _, candidate := range strings.Split(v, ",") {
		fields := strings.Fields(candidate)
		if len(fields) == 0 {
			continue
		}
		if !isSafeHTMLURL(fields[0], false) {
			return false
		}
	}
	return true
}

// HTMLToText flattens an HTML fragment into plain text, skipping script/style
// content. It is used for the RSS excerpt and for the "did sanitizing leave
// anything visible" check.
func HTMLToText(src string) string {
	ctx := &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div}
	nodes, err := html.ParseFragment(strings.NewReader(src), ctx)
	if err != nil {
		return ""
	}
	var parts []string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch strings.ToLower(n.Data) {
			case "script", "style", "template":
				return
			}
		}
		if n.Type == html.TextNode {
			parts = append(parts, n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	for _, n := range nodes {
		walk(n)
	}
	return strings.Join(strings.Fields(strings.Join(parts, " ")), " ")
}
