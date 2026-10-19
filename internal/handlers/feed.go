package handlers

import (
	"encoding/xml"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// --- RSS 2.0 feed (GET /feed, public) ---

type rssFeed struct {
	XMLName xml.Name    `xml:"rss"`
	Version string       `xml:"version,attr"`
	Channel rssChannel   `xml:"channel"`
}

type rssChannel struct {
	Title        string    `xml:"title"`
	Link         string    `xml:"link"`
	Description  string    `xml:"description"`
	LastBuildDate string   `xml:"lastBuildDate"`
	Items        []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string   `xml:"title"`
	Link        string   `xml:"link"`
	Description string   `xml:"description"`
	GUID        rssGUID  `xml:"guid"`
	PubDate     string   `xml:"pubDate"`
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
	Loc       string `xml:"loc"`
	LastMod   string `xml:"lastmod,omitempty"`
	ChangeFreq string `xml:"changefreq,omitempty"`
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
	desc := cfg.FooterText
	if desc == "" {
		desc = title
	}

	feed := rssFeed{
		Version: "2.0",
		Channel: rssChannel{
			Title:        title,
			Link:         base,
			Description:  desc,
			LastBuildDate: time.Now().UTC().Format(time.RFC1123Z),
			Items:        make([]rssItem, 0, len(posts)),
		},
	}
	for _, p := range posts {
		link := base + "/read/" + p.ID
		feed.Channel.Items = append(feed.Channel.Items, rssItem{
			Title:       p.Title,
			Link:        link,
			Description: feedExcerpt(p.Body, 320),
			GUID:        rssGUID{Value: link, PermaLink: "true"},
			PubDate:     p.CreatedAt.UTC().Format(time.RFC1123Z),
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
		{Loc: base + "/", ChangeFreq: "daily"},
		{Loc: base + "/archive", ChangeFreq: "weekly"},
	}
	for _, p := range posts {
		loc := base + "/read/" + p.ID
		last := p.UpdatedAt
		if last.IsZero() {
			last = p.CreatedAt
		}
		urls = append(urls, sitemapURL{
			Loc:        loc,
			LastMod:    last.UTC().Format("2006-01-02"),
			ChangeFreq: "weekly",
		})
	}
	writeXML(w, "application/xml", sitemap{
		Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9",
		URLs:  urls,
	})
}

// feedBaseURL returns the canonical site base URL. An explicitly configured
// site_url wins; otherwise it is derived from the request (honouring
// X-Forwarded-Proto behind a reverse proxy).
func feedBaseURL(r *http.Request, siteURL string) string {
	if u := strings.TrimSpace(siteURL); u != "" {
		return strings.TrimRight(u, "/")
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

func feedExcerpt(md string, n int) string {
	s := reMDLink.ReplaceAllString(md, "$1")
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
