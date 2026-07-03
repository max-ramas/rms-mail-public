package smtp

import (
	"crypto/tls"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/aymerick/douceur/inliner"
	mail "github.com/wneessen/go-mail"
)

type Client struct {
	host       string
	port       int
	from       string
	username   string
	password   string
	oauthToken string
	useOAuth2  bool
}

func NewClient(host string, port int, username, password, from string) *Client {
	return &Client{
		host:      host,
		port:      port,
		from:      from,
		username:  username,
		password:  password,
		useOAuth2: false,
	}
}

func (c *Client) From() string {
	return c.from
}

func NewOAuthClient(host string, port int, username, oauthToken, from string) *Client {
	return &Client{
		host:       host,
		port:       port,
		from:       from,
		username:   username,
		oauthToken: oauthToken,
		useOAuth2:  true,
	}
}

type Attachment struct {
	Filename string
	Path     string
}

type Email struct {
	To          []string
	Cc          []string
	Subject     string
	Body        string
	HTML        string
	From        string
	MessageID   string
	Date        time.Time
	InReplyTo   string
	References  string
	Attachments []Attachment
}

// cssVarPattern matches CSS variable references like var(--primary, fallback)
var cssVarPattern = regexp.MustCompile(`var\(--[a-zA-Z0-9_-]+(?:,\s*[^)]+)?\)`)

// modernLayoutPattern matches display values not supported in Outlook
var flexGridPattern = regexp.MustCompile(`display\s*:\s*(flex|inline-flex|grid|inline-grid)`)

func (c *Client) Send(email *Email) error {
	msg, err := email.ToMsg(c.from)
	if err != nil {
		return err
	}
	return c.SendMsg(msg)
}

func (c *Client) SendMsg(msg *mail.Msg) error {
	var client *mail.Client
	var err error

	opts := []mail.Option{
		mail.WithPort(c.port),
		mail.WithUsername(c.username),
	}

	if c.useOAuth2 {
		opts = append(opts, mail.WithPassword(c.oauthToken), mail.WithSMTPAuth(mail.SMTPAuthXOAUTH2))
	} else {
		opts = append(opts, mail.WithPassword(c.password), mail.WithSMTPAuth(mail.SMTPAuthAutoDiscover))
	}

	if c.port == 465 {
		opts = append(opts, mail.WithSSL())
	} else {
		opts = append(opts, mail.WithTLSPolicy(mail.TLSMandatory))
	}
	opts = append(opts, mail.WithTLSConfig(&tls.Config{InsecureSkipVerify: os.Getenv("ALLOW_INSECURE_TLS") == "true"}))

	client, err = mail.NewClient(c.host, opts...)
	if err != nil {
		return fmt.Errorf("failed to create mail client: %w", err)
	}
	defer client.Close()

	if err := client.DialAndSend(msg); err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

func (email *Email) ToMsg(defaultFrom string) (*mail.Msg, error) {
	msg := mail.NewMsg()

	from := email.From
	if from == "" {
		from = defaultFrom
	}
	if err := msg.From(from); err != nil {
		return nil, fmt.Errorf("failed to set From: %w", err)
	}

	if err := msg.To(email.To...); err != nil {
		return nil, fmt.Errorf("failed to set To: %w", err)
	}
	if len(email.Cc) > 0 {
		if err := msg.Cc(email.Cc...); err != nil {
			return nil, fmt.Errorf("failed to set Cc: %w", err)
		}
	}

	msg.Subject(email.Subject)

	if email.MessageID != "" {
		msg.SetMessageIDWithValue(email.MessageID)
	}
	if !email.Date.IsZero() {
		msg.SetDateWithValue(email.Date)
	}
	if email.InReplyTo != "" {
		msg.SetHeader(mail.HeaderInReplyTo, email.InReplyTo)
	}
	if email.References != "" {
		msg.SetHeader(mail.HeaderReferences, email.References)
	}

	if email.HTML != "" {
		html := email.HTML
		if strings.Contains(html, "class=") {
			if inlined, err := inliner.Inline(html); err == nil {
				html = inlined
			}
		}
		html = simplifyEmailHTML(html)
		html = wrapEmailHTML(html)

		msg.SetBodyString(mail.TypeTextHTML, html)
		if email.Body != "" {
			msg.AddAlternativeString(mail.TypeTextPlain, email.Body)
		}
	} else {
		msg.SetBodyString(mail.TypeTextPlain, email.Body)
	}

	for _, att := range email.Attachments {
		if att.Path != "" {
			msg.AttachFile(att.Path, mail.WithFileName(att.Filename))
		}
	}

	return msg, nil
}

func (email *Email) Bytes() ([]byte, error) {
	msg, err := email.ToMsg(email.From)
	if err != nil {
		return nil, err
	}
	var buf strings.Builder
	_, err = msg.WriteTo(&buf)
	if err != nil {
		return nil, err
	}
	return []byte(buf.String()), nil
}

// simplifyEmailHTML strips modern CSS that Outlook's Word renderer doesn't support.
// Outlook (all versions up to 365) uses MS Word for HTML rendering, which does NOT
// support: CSS variables (var(--...)), flexbox (display: flex), grid (display: grid),
// and many modern CSS features.
func simplifyEmailHTML(html string) string {
	// Replace CSS variable references with fallback values or empty string
	html = cssVarPattern.ReplaceAllStringFunc(html, func(match string) string {
		// var(--primary, #333) → #333 or empty
		if idx := strings.Index(match, ","); idx != -1 {
			fallback := strings.TrimSpace(match[idx+1:])
			fallback = strings.TrimRight(fallback, ")")
			return fallback
		}
		return ""
	})

	// Replace flex/grid with block (widely supported in email clients)
	html = flexGridPattern.ReplaceAllString(html, "display:block")

	// Remove unsupported CSS properties from inline styles
	// These are properties that Outlook's Word renderer will ignore or choke on
	unsupported := []string{
		"gap:",
		"row-gap:",
		"column-gap:",
		"align-items:",
		"justify-content:",
		"justify-items:",
		"place-items:",
		"place-content:",
		"object-fit:",
		"object-position:",
		"backdrop-filter:",
		"filter:",
		"transition:",
		"transform:",
		"animation:",
		"box-shadow:",
		"text-shadow:",
		"border-radius:", // supported partially in Outlook, keep but clip
		"outline:",
		"opacity:",
	}

	lines := strings.Split(html, "\n")
	var result []string
	for _, line := range lines {
		cleaned := line
		lowerLine := strings.ToLower(line)
		for _, prop := range unsupported {
			if strings.Contains(lowerLine, prop) {
				// Remove the property from inline style="..."
				cleaned = removeCSSProperty(cleaned, prop)
			}
		}
		result = append(result, cleaned)
	}

	return strings.Join(result, "\n")
}

// removeCSSProperty removes a CSS property (e.g., "gap:") from style attribute values.
func removeCSSProperty(line, property string) string {
	// Match style="...prop..." and remove that property
	styleRe := regexp.MustCompile(`(style="[^"]*)` + regexp.QuoteMeta(property) + `[^;]*;?\s*`)
	return styleRe.ReplaceAllString(line, "$1")
}

// wrapEmailHTML wraps the body in a table-based layout for Outlook compatibility.
// Outlook uses MS Word's renderer which handles tables best.
func wrapEmailHTML(html string) string {
	// If already wrapped in a table/body structure, skip
	if strings.Contains(strings.ToLower(html), "<table") && strings.Contains(strings.ToLower(html), "</table>") {
		return html
	}

	// Simple wrapper: <body> → <body><table width="100%"><tr><td>
	bodyClose := "</body>"

	// Close our table wrapper before </body>
	html = strings.Replace(html, bodyClose, "</td></tr></table>\n"+bodyClose, 1)

	// Insert opening table after <body> or <body ...>
	openBodyRe := regexp.MustCompile(`<body[^>]*>`)
	matches := openBodyRe.FindString(html)
	if matches != "" {
		// Insert after <body> tag
		tableWrapper := `<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0" style="width:100%%;max-width:600px;margin:0 auto;font-family:Arial,sans-serif;font-size:16px;line-height:1.5;color:#333;">`
		tableWrapper += `<tr><td style="padding:20px;">`
		html = strings.Replace(html, matches, matches+"\n"+tableWrapper, 1)
	}

	return html
}
