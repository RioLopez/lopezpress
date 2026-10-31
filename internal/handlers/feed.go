package handlers

import (
	"encoding/xml"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// --- RSS 2.0 feed (GET /feed, public) ---

type rssFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title         string    `xml:"title"`
	Link          string    `xml:"link"`
	Description   string    `xml:"description"`
	LastBuildDate string    `xml:"lastBuildDate"`
	Items         []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string  `xml:"title"`
	Link        string  `xml:"link"`
	Description string  `xml:"description"`
	GUID        rssGUID `xml:"guid"`
	PubDate     string  `xml:"pubDate"`
}

// rssGUID marks the item link as a permanent identifier.
type rssGUID struct {
	Value     string `xml:",chardata"`
	PermaLink string `xml:"isPermaLink,attr"`
}

// --- sitemap (GET /sitemap.xml, public) ---

type sitemap struct {
	XMLName xml.Name     `xml:"urlset"`
	Xmlns   string       `xml:"xmlns,attr"`
	URLs    []sitemapURL `xml:"url"`
}

type sitemapURL struct {
	Loc        string `xml:"loc"`
	LastMod    string `xml:"lastmod,omitempty"`
	ChangeFreq string `xml:"changefreq,omitempty"`
	Priority   string `xml:"priority,omitempty"`
}

// FeedRSS serves an RSS 2.0 feed of the most recent published posts.
// GET /feed (public)
func FeedRSS(w http.ResponseWriter, r *http.Request) {
	cfg, err := svc.GetSiteConfig(r.Context(), false)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	posts, err := svc.FeedPosts(r.Context(), 50)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	base := feedBaseURL(r, cfg.SiteURL)
	title := cfg.SiteTitle
	if title == "" {
		title = cfg.SiteName
	}
	if title == "" {
		title = "Echo"
	}
	// A feed <description> is the channel's tagline, not the page footer.
	// Footer text is copyright/site-notice copy and reads out of place in an
	// RSS reader ("Copyright 0001 test" is the kind of thing nobody wants in
	// their reader's channel header). Prefer an explicit description, then the
	// site title; only fall back to the footer if both are empty.
	desc := cfg.SiteDescription
	if desc == "" {
		desc = title
	}

	feed := rssFeed{
		Version: "2.0",
		Channel: rssChannel{
			Title:         title,
			Link:          base,
			Description:   desc,
			LastBuildDate: time.Now().UTC().Format(time.RFC1123Z),
			Items:         make([]rssItem, 0, len(posts)),
		},
	}
	for _, p := range posts {
		link := permalink(base, p.Slug, p.ID)
		// pubDate is the last edit, not the create date: an edited article
		// should re-sync in a reader, mirroring the sitemap's lastmod.
		pub := p.UpdatedAt
		if pub.IsZero() {
			pub = p.CreatedAt
		}
		feed.Channel.Items = append(feed.Channel.Items, rssItem{
			Title:       p.Title,
			Link:        link,
			Description: feedExcerpt(p.Body, 320),
			GUID:        rssGUID{Value: link, PermaLink: "true"},
			PubDate:     pub.UTC().Format(time.RFC1123Z),
		})
	}
	writeXML(w, "application/rss+xml", feed)
}

// Sitemap serves a sitemap XML of the public pages and every published post.
// GET /sitemap.xml (public)
func Sitemap(w http.ResponseWriter, r *http.Request) {
	cfg, err := svc.GetSiteConfig(r.Context(), false)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	posts, err := svc.FeedPosts(r.Context(), 1000)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	base := feedBaseURL(r, cfg.SiteURL)
	urls := []sitemapURL{
		{Loc: base + "/", ChangeFreq: "daily", Priority: "1.0"},
		{Loc: base + "/archive", ChangeFreq: "weekly", Priority: "0.5"},
	}
	for _, p := range posts {
		loc := permalink(base, p.Slug, p.ID)
		last := p.UpdatedAt
		if last.IsZero() {
			last = p.CreatedAt
		}
		urls = append(urls, sitemapURL{
			Loc:        loc,
			LastMod:    last.UTC().Format("2006-01-02"),
			ChangeFreq: "weekly",
			Priority:   "0.8",
		})
	}
	writeXML(w, "application/xml", sitemap{
		Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9",
		URLs:  urls,
	})
}

// feedBaseURL returns the canonical site base URL as a bare origin
// (scheme://host[:port]) — no path, query, or trailing slash.
//
// An explicitly configured site_url wins, but only its origin survives: every
// link in the feed and sitemap is built by appending to this string, so a
// stored path is not a cosmetic problem, it rewrites every URL. Production once
// held ".../admin" and both files spent their whole life advertising
// ".../admin/p/<slug>" — 404s that also handed crawlers the admin path.
// Truncating here rather than trusting the stored value means links are correct
// even before the settings form is re-saved.
//
// With no usable site_url the base is derived from the request, honouring
// X-Forwarded-Proto behind the reverse proxy.
func feedBaseURL(r *http.Request, siteURL string) string {
	if u := strings.TrimSpace(siteURL); u != "" {
		if parsed, err := url.Parse(u); err == nil && parsed.Host != "" &&
			(parsed.Scheme == "http" || parsed.Scheme == "https") {
			return parsed.Scheme + "://" + parsed.Host
		}
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

// feedExcerpt turns markdown-ish body text into a short plain-text snippet
// for the RSS <description>: inline links keep their text, markup chars are
// dropped, and the result is truncated to n runes.
var reMDLink = regexp.MustCompile(`\[([^\]]+)\]\([^)]*\)`)

// An image "![alt](url)" contains the bracketed form "[alt](url)" that
// reMDLink matches, so stripping links first leaves the leading "!" behind and
// a photo-heavy post's excerpt reads "!个人简历 !机房 …". Strip the whole image
// first (keeping only its alt text as the snippet) so no bare "!" ever ships.
var reMDImage = regexp.MustCompile(`!\[([^\]]*)\]\([^)]*\)`)

func feedExcerpt(md string, n int) string {
	s := reMDImage.ReplaceAllString(md, "$1")
	s = reMDLink.ReplaceAllString(s, "$1")
	s = strings.Map(func(r rune) rune {
		switch r {
		case '#', '*', '_', '`', '>', '~':
			return -1 // drop markdown punctuation
		default:
			return r
		}
	}, s)
	s = strings.Join(strings.Fields(s), " ")
	runes := []rune(s)
	if len(runes) > n {
		s = string(runes[:n]) + "…"
	}
	return s
}

// writeXML marshals v to XML with an XML declaration and the given content
// type. encoding/xml escapes all content, so titles/bodies cannot inject
// markup (no XSS via the feed). On a marshal failure it returns 500 with a
// JSON error body consistent with the rest of the API (the caller would
// otherwise receive a 200 with a non-feed body).
func writeXML(w http.ResponseWriter, contentType string, v any) {
	w.Header().Set("Content-Type", contentType+"; charset=utf-8")
	out, err := xml.Marshal(v)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"failed to render feed"}`))
		return
	}
	w.Write([]byte(xml.Header))
	w.Write(out)
}

// permalink prefers the slug form and falls back to the UUID route
// for rows created before slugs existed. One helper so RSS and sitemap can
// never advertise two different shapes of the same URL.
//
// The slug is percent-encoded: a blog that allows Chinese (or emoji, or any
// non-ASCII) titles will have Chinese slugs, and a raw Chinese byte in a
// <link> / <loc> / <guid> is an illegal URL under RFC 3986 — Google will
// refuse to index it, and several RSS readers will drop the item outright.
// The in-app browser already does this on its own (wouter + encodeURIComponent
// in api.ts), so the only place that still emitted a bare Chinese URL was
// this feed, which made the RSS/sitemap the SEO dead end of the site.
func permalink(base, slug, id string) string {
	if slug != "" {
		return base + "/p/" + url.PathEscape(slug)
	}
	return base + "/read/" + id
}

// Robots serves /robots.txt.
//
// A blog without a robots.txt is not broken, but the file is also the only
// place to declare the sitemap and to keep /admin + /api out of crawlers. It
// follows the public_read switch: a locked site tells crawlers to stay away
// instead of advertising URLs that answer 401.
func Robots(w http.ResponseWriter, r *http.Request) {
	cfg, err := svc.GetSiteConfig(r.Context(), false)
	base := feedBaseURL(r, "")
	locked := false
	if err == nil {
		base = feedBaseURL(r, cfg.SiteURL)
		locked = !cfg.PublicRead
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if locked {
		_, _ = w.Write([]byte("User-agent: *\nDisallow: /\n"))
		return
	}
	_, _ = w.Write([]byte("User-agent: *\nAllow: /\nDisallow: /admin\nDisallow: /api/\nDisallow: /login\n\nSitemap: " + base + "/sitemap.xml\n"))
}
