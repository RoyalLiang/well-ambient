package server

import (
	"fmt"
	"strings"

	nethtml "golang.org/x/net/html"
)

var emailTemplateAllowedTags = map[string]struct{}{
	"a": {}, "article": {}, "aside": {}, "b": {}, "blockquote": {}, "br": {},
	"caption": {}, "center": {}, "code": {}, "col": {}, "colgroup": {}, "dd": {},
	"div": {}, "dl": {}, "dt": {}, "em": {}, "font": {}, "footer": {}, "h1": {},
	"h2": {}, "h3": {}, "h4": {}, "h5": {}, "h6": {}, "header": {}, "hr": {},
	"i": {}, "img": {}, "li": {}, "main": {}, "nav": {}, "ol": {}, "p": {},
	"pre": {}, "section": {}, "small": {}, "span": {}, "strong": {}, "sub": {},
	"sup": {}, "table": {}, "tbody": {}, "td": {}, "tfoot": {}, "th": {},
	"thead": {}, "tr": {}, "u": {}, "ul": {},
}

var emailTemplateGlobalAttributes = map[string]struct{}{
	"align": {}, "aria-hidden": {}, "aria-label": {}, "bgcolor": {},
	"class": {}, "dir": {}, "height": {}, "id": {}, "lang": {}, "role": {},
	"style": {}, "title": {}, "valign": {}, "width": {},
}

var emailTemplateTagAttributes = map[string]map[string]struct{}{
	"a":        {"href": {}, "rel": {}, "target": {}},
	"col":      {"span": {}},
	"colgroup": {"span": {}},
	"img":      {"alt": {}, "border": {}, "src": {}},
	"table":    {"border": {}, "cellpadding": {}, "cellspacing": {}, "summary": {}},
	"td":       {"colspan": {}, "headers": {}, "rowspan": {}, "scope": {}},
	"th":       {"colspan": {}, "headers": {}, "rowspan": {}, "scope": {}},
}

func sanitizeEmailTemplateHTML(input string) (string, error) {
	doc, err := nethtml.Parse(strings.NewReader(input))
	if err != nil {
		return "", fmt.Errorf("invalid email template html")
	}
	var body *nethtml.Node
	var findBody func(*nethtml.Node)
	findBody = func(node *nethtml.Node) {
		if body != nil {
			return
		}
		if node.Type == nethtml.ElementNode && node.Data == "body" {
			body = node
			return
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			findBody(child)
			if body != nil {
				return
			}
		}
	}
	findBody(doc)
	if body == nil {
		return "", fmt.Errorf("email template html has no body")
	}
	var out strings.Builder
	for child := body.FirstChild; child != nil; child = child.NextSibling {
		sanitized := sanitizeEmailTemplateNode(child)
		if sanitized == nil {
			continue
		}
		if err := nethtml.Render(&out, sanitized); err != nil {
			return "", fmt.Errorf("cannot sanitize email template html")
		}
	}
	if strings.TrimSpace(out.String()) == "" {
		return "", fmt.Errorf("email template html is empty")
	}
	return out.String(), nil
}

func sanitizeEmailTemplateNode(node *nethtml.Node) *nethtml.Node {
	switch node.Type {
	case nethtml.TextNode:
		return &nethtml.Node{Type: nethtml.TextNode, Data: node.Data}
	case nethtml.ElementNode:
		if _, ok := emailTemplateAllowedTags[strings.ToLower(node.Data)]; !ok {
			return nil
		}
		tag := strings.ToLower(node.Data)
		clone := &nethtml.Node{
			Type:     nethtml.ElementNode,
			Data:     node.Data,
			DataAtom: node.DataAtom,
			Attr:     sanitizeEmailTemplateAttributes(tag, node.Attr),
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if sanitized := sanitizeEmailTemplateNode(child); sanitized != nil {
				clone.AppendChild(sanitized)
			}
		}
		return clone
	default:
		return nil
	}
}

func sanitizeEmailTemplateAttributes(tag string, attrs []nethtml.Attribute) []nethtml.Attribute {
	tagAttrs := emailTemplateTagAttributes[tag]
	out := make([]nethtml.Attribute, 0, len(attrs))
	for _, attr := range attrs {
		key := strings.ToLower(attr.Key)
		if _, ok := emailTemplateGlobalAttributes[key]; !ok {
			if tagAttrs == nil {
				continue
			}
			if _, ok := tagAttrs[key]; !ok {
				continue
			}
		}
		value := attr.Val
		if key == "href" || key == "src" {
			if !safeEmailTemplateURL(value) {
				continue
			}
		}
		if key == "style" && !safeEmailTemplateStyle(value) {
			continue
		}
		out = append(out, nethtml.Attribute{Key: attr.Key, Val: value})
	}
	return out
}

func safeEmailTemplateURL(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	return !strings.HasPrefix(lower, "javascript:") &&
		!strings.HasPrefix(lower, "vbscript:") &&
		!strings.HasPrefix(lower, "data:text/html")
}

func safeEmailTemplateStyle(value string) bool {
	lower := strings.ToLower(value)
	return !strings.Contains(lower, "javascript:") && !strings.Contains(lower, "expression(")
}
