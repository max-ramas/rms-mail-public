package api

import (
	"bytes"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

// coreStyles — minimal reset injected into the iframe srcdoc.
// Backend normalizer handles width/align/bgcolor.
const coreStyles = `
:where(html, body) { margin:0; padding:0; background-color:transparent; font-family: system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif, 'Apple Color Emoji', 'Segoe UI Emoji', 'Segoe UI Symbol'; }
:where(table) { border-collapse:collapse; border-spacing:0; }
:where(img) { max-width:100% !important; height:auto !important; cursor: zoom-in; }
:where(pre, code) { white-space:pre !important; overflow-x:auto !important; }
:where(#rms-mail-wrapper) { width: 100%; box-sizing: border-box; padding: 16px; }
:where(#rms-mail-body-surrogate) { width: 100%; margin: 0; padding: 0; box-sizing: border-box; }
:where(a[href], button, [role="button"], input[type="button"], input[type="submit"]) { cursor: pointer; }
`

// wrapEmailForIframe wraps cleaned HTML into a self-contained iframe srcdoc:
// - Injects core styles into <head>
// - Sets <base target="_blank">
// - Wraps <body> content into <div id="rms-mail-wrapper">
// This replaces processEmailHtml() on the frontend.
func wrapEmailForIframe(sanitizedHTML string) string {
	return `<!DOCTYPE html>
<html>
<head>
	    <meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline' https:; img-src 'self' data: https:; font-src 'self' data: https:;">
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <style id="rmsmail-core">` + coreStyles + `</style>
</head>
<body>
    <div id="rms-mail-wrapper">
    ` + sanitizedHTML + `
    </div>
</body>
</html>`
}

// normalizeEmailHTML converts legacy HTML attributes to inline styles
// Ensures emails render consistently in standards-mode browsers.
func normalizeEmailHTML(rawHTML string) string {
	const marker = "data-rms-normalized"
	if strings.Contains(rawHTML, marker) {
		return rawHTML
	}

	doc, err := html.Parse(strings.NewReader(rawHTML))
	if err != nil {
		return rawHTML
	}

	setAttr(doc, marker, "1")
	walkAndNormalize(doc)

	var buf bytes.Buffer
	if err := html.Render(&buf, doc); err != nil {
		return rawHTML
	}
	return buf.String()
}

func walkAndNormalize(n *html.Node) {
	if n.Type == html.ElementNode {
		sanitizeNode(n)
		if n.Parent == nil {
			return // node was removed by sanitizer
		}
		normalizeNode(n)
	}
	for c := n.FirstChild; c != nil; {
		next := c.NextSibling // grab next before 'c' might be removed
		walkAndNormalize(c)
		c = next
	}
}

var removedEmailElements = map[string]bool{
	"script": true, "iframe": true, "frame": true, "frameset": true,
	"object": true, "embed": true, "applet": true,
	"form": true, "input": true, "button": true, "textarea": true, "select": true,
	"base": true, // hijacks relative URLs inside iframe
}

func sanitizeNode(n *html.Node) {
	if n.Type != html.ElementNode {
		return
	}
	tag := strings.ToLower(n.Data)
	if removedEmailElements[tag] {
		if n.Parent != nil {
			n.Parent.RemoveChild(n)
		}
		return
	}
	if tag == "meta" {
		// Meta in email fragments (viewport, charset, refresh, …) are invalid inside
		// our iframe body and trigger browser console warnings when malformed.
		if n.Parent != nil {
			n.Parent.RemoveChild(n)
		}
		return
	}
	filtered := n.Attr[:0]
	for _, a := range n.Attr {
		key := strings.ToLower(a.Key)
		if strings.HasPrefix(key, "on") {
			continue
		}
		if key == "href" || key == "src" || key == "xlink:href" {
			if !isSafeEmailURL(a.Val, key == "src") {
				continue
			}
		}
		filtered = append(filtered, a)
	}
	n.Attr = filtered
}

func isSafeEmailURL(raw string, isSrc bool) bool {
	val := strings.TrimSpace(raw)
	if val == "" || strings.HasPrefix(val, "#") {
		return true
	}
	low := strings.ToLower(val)
	if strings.HasPrefix(low, "javascript:") || strings.HasPrefix(low, "vbscript:") {
		return false
	}
	if strings.HasPrefix(low, "data:") {
		if !isSrc {
			return false
		}
		return strings.HasPrefix(low, "data:image/")
	}
	if strings.HasPrefix(low, "cid:") {
		return isSrc
	}
	if strings.HasPrefix(val, "/") {
		return true
	}
	return strings.HasPrefix(low, "http://") || strings.HasPrefix(low, "https://") || strings.HasPrefix(low, "mailto:")
}

func normalizeNode(n *html.Node) {
	attrs := getAttrs(n)
	existing := attrs["style"]
	var add []string

	switch n.Data {
	case "img":
		w, h := attrs["width"], attrs["height"]
		if w != "" && !strings.HasSuffix(w, "%") {
			if wp := parsePixel(w); wp >= 0 {
				add = append(add, fmt.Sprintf("width:%dpx", wp))
			}
		} else if strings.HasSuffix(w, "%") {
			add = append(add, fmt.Sprintf("width:%s", w))
		}

		if h != "" && !strings.HasSuffix(h, "%") {
			if hp := parsePixel(h); hp >= 0 {
				add = append(add, fmt.Sprintf("height:%dpx", hp))
			}
		} else if strings.HasSuffix(h, "%") {
			add = append(add, fmt.Sprintf("height:%s", h))
		}

		// Rewrite src: external URLs → Camo proxy, bare UUIDs/paths → attachment API
		for i, a := range n.Attr {
			if strings.ToLower(a.Key) == "src" {
				val := strings.TrimSpace(a.Val)
				lowVal := strings.ToLower(val)
				if strings.HasPrefix(lowVal, "http") {
					val = sanitizeImageURL(val)
					n.Attr[i].Val = fmt.Sprintf("/api/media/proxy?url=%s&sig=%s", url.QueryEscape(val), camoSign(val))
				} else if !strings.HasPrefix(lowVal, "data:") && !strings.HasPrefix(lowVal, "cid:") && val != "" {
					// Relative path or bare UUID — route through attachment API
					n.Attr[i].Val = "/api/attachments/" + val
				}
				break
			}
		}

	case "table":
		if w := attrs["width"]; w != "" {
			if strings.HasSuffix(w, "%") {
				add = append(add, fmt.Sprintf("width:%s", w))
			} else if wp := parsePixel(w); wp >= 0 {
				add = append(add,
					fmt.Sprintf("width:%dpx", wp),
					"max-width:100%")
			}
		}
		if h := attrs["height"]; h != "" {
			if hp := parsePixel(h); hp >= 0 {
				add = append(add, fmt.Sprintf("height:%dpx", hp))
			}
		}
		if a := attrs["align"]; a != "" {
			if a == "center" {
				add = append(add, "margin-left:auto", "margin-right:auto")
			} else if a == "right" {
				add = append(add, "margin-left:auto", "margin-right:0")
			} else if a == "left" {
				add = append(add, "margin-left:0", "margin-right:auto")
			}
		}
		if b := attrs["bgcolor"]; b != "" {
			add = append(add, fmt.Sprintf("background-color:%s", b))
		}

	case "td", "th":
		if w := attrs["width"]; w != "" {
			if strings.HasSuffix(w, "%") {
				add = append(add, fmt.Sprintf("width:%s", w))
			} else if wp := parsePixel(w); wp >= 0 {
				add = append(add,
					fmt.Sprintf("width:%dpx", wp),
					"max-width:100%")
			}
		}
		if h := attrs["height"]; h != "" {
			if hp := parsePixel(h); hp >= 0 {
				add = append(add, fmt.Sprintf("height:%dpx", hp))
			}
		}
		if a := attrs["align"]; a != "" {
			if a == "center" {
				// Same HTML4→CSS3 gap as div/p/hN (see above).
				// A <td> may carry display:block in its existing style,
				// losing table-cell centering of block children.
				add = append(add,
					"text-align:center",
					"text-align:-webkit-center",
					"text-align:-moz-center",
				)
			} else {
				add = append(add, fmt.Sprintf("text-align:%s", a))
			}
		}
		if v := attrs["valign"]; v != "" {
			add = append(add, fmt.Sprintf("vertical-align:%s", v))
		}
		if b := attrs["bgcolor"]; b != "" {
			add = append(add, fmt.Sprintf("background-color:%s", b))
		}
		// nowrap is a deprecated HTML4 attribute; convert to CSS.
		if _, ok := attrs["nowrap"]; ok {
			add = append(add, "white-space:nowrap")
		}
	case "p", "div", "h1", "h2", "h3", "h4", "h5", "h6":
		if a := attrs["align"]; a != "" {
			if a == "center" {
				// HTML4: align="center" on a block container centers both inline
				// and block children.  CSS text-align:center only handles inline
				// content, so block children (tables, nested divs) snap to the
				// left edge when the <!DOCTYPE html> forces standards mode.
				//
				// The non-standard but universally supported -webkit-center and
				// -moz-center values replicate the exact <center> tag behaviour:
				// they centre block-level descendants without breaking inline
				// children.  Unknown values are silently dropped by each engine,
				// so the cascade safely falls back to plain text-align:center.
				add = append(add,
					"text-align:center",
					"text-align:-webkit-center",
					"text-align:-moz-center",
				)
			} else {
				add = append(add, fmt.Sprintf("text-align:%s", a))
			}
		}

	case "body":
		n.Data = "div"
		setAttr(n, "id", "rms-mail-body-surrogate")
		if b := attrs["bgcolor"]; b != "" {
			add = append(add, fmt.Sprintf("background-color:%s", b))
		}
	}

	if len(add) > 0 {
		ns := strings.Join(add, ";")
		if existing != "" {
			existing = strings.TrimSpace(existing)
			existing = strings.TrimRight(existing, ";")
			ns = existing + ";" + ns
		}
		setAttr(n, "style", ns)
	}
}

func getAttrs(n *html.Node) map[string]string {
	m := make(map[string]string, len(n.Attr))
	for _, a := range n.Attr {
		m[strings.ToLower(a.Key)] = a.Val
	}
	return m
}

func setAttr(n *html.Node, key, val string) {
	for i, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			n.Attr[i].Val = val
			return
		}
	}
	n.Attr = append(n.Attr, html.Attribute{Key: key, Val: val})
}

func parsePixel(s string) int {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "")
	s = strings.TrimSuffix(s, "px")
	v, _ := strconv.Atoi(s)
	return v
}
