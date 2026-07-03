package sanitizer

import (
	"regexp"
	"strings"
)

var trackingPatterns = []*regexp.Regexp{
	regexp.MustCompile(`<img[^>]+src=["']https?://[^"']*track(ing)?[^"']*["']`),
	regexp.MustCompile(`<img[^>]+src=["']https?://[^"']*pixel[^"']*["']`),
	regexp.MustCompile(`<img[^>]+src=["']https?://[^"']*beacon[^"']*["']`),
	regexp.MustCompile(`<img[^>]+width=["']?1["'][^>]*height=["']?1["'][^>]*>`),
	regexp.MustCompile(`<img[^>]+height=["']?1["'][^>]*width=["']?1["'][^>]*>`),
	regexp.MustCompile(`<img[^>]+style=["'][^"']*display:\s*none[^"']*["'][^>]*>`),
	regexp.MustCompile(`<img[^>]+style=["'][^"']*visibility:\s*hidden[^"']*["'][^>]*>`),
	regexp.MustCompile(`https?://[^"'\s>]+/(track|open|view|click|pixel|beacon)[^"'\s]*`),
}

var trackingDomains = []string{
	"mailtrack.io",
	"mailchimp.com/track",
	"sendgrid.net",
	"amazonses.com",
	"mandrillapp.com",
	"postmarkapp.com",
	"sparkpost.com",
	"mailgun.org",
}

type EmailSanitizer struct {
	removeTrackingPixels bool
	stripExternalImages  bool
	allowedDomains       []string
}

func NewEmailSanitizer() *EmailSanitizer {
	return &EmailSanitizer{
		removeTrackingPixels: true,
		stripExternalImages:  false,
		allowedDomains:       []string{},
	}
}

func (s *EmailSanitizer) WithRemoveTrackingPixels(enabled bool) *EmailSanitizer {
	s.removeTrackingPixels = enabled
	return s
}

func (s *EmailSanitizer) WithStripExternalImages(strip bool) *EmailSanitizer {
	s.stripExternalImages = strip
	return s
}

func (s *EmailSanitizer) WithAllowedDomains(domains []string) *EmailSanitizer {
	s.allowedDomains = domains
	return s
}

func (s *EmailSanitizer) Sanitize(html string) string {
	if s.removeTrackingPixels {
		html = s.removeTrackingPixelsFromHTML(html)
	}

	if s.stripExternalImages {
		html = s.stripExternalImagesFromHTML(html)
	}

	return html
}

func (s *EmailSanitizer) removeTrackingPixelsFromHTML(html string) string {
	for _, pattern := range trackingPatterns {
		html = pattern.ReplaceAllString(html, "")
	}

	for _, domain := range trackingDomains {
		pattern := regexp.MustCompile(`<img[^>]+src=["']https?://[^"']*` + regexp.QuoteMeta(domain) + `[^"']*["']`)
		html = pattern.ReplaceAllString(html, "")
	}

	html = s.removeOpenTrackingImage(html)

	return html
}

func (s *EmailSanitizer) removeOpenTrackingImage(html string) string {
	pattern := regexp.MustCompile(`<img[^>]*(?:width=["']?[01]["']?|height=["']?[01]["']?)[^>]*>`)
	return pattern.ReplaceAllString(html, "")
}

func (s *EmailSanitizer) stripExternalImagesFromHTML(html string) string {
	imgPattern := regexp.MustCompile(`<img([^>]+)src=["'](https?://[^"']+)["']([^>]*)>`)

	return imgPattern.ReplaceAllStringFunc(html, func(match string) string {
		for _, domain := range s.allowedDomains {
			if strings.Contains(match, domain) {
				return match
			}
		}
		return strings.Replace(match, `src="`, `src="data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7" original-src="`, 1)
	})
}

func (s *EmailSanitizer) HasTrackingPixel(html string) bool {
	for _, pattern := range trackingPatterns {
		if pattern.MatchString(html) {
			return true
		}
	}
	return false
}
