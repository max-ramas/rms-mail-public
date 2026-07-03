package sync

import (
	"strings"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

// resolveDraftsFolder returns the IMAP mailbox path for drafts on this connection.
func resolveDraftsFolder(c *imapclient.Client) string {
	listCmd := c.List("", "*", &imap.ListOptions{ReturnSpecialUse: true})
	mailboxes, err := listCmd.Collect()
	if err == nil {
		for _, mbox := range mailboxes {
			for _, attr := range mbox.Attrs {
				if attr == imap.MailboxAttrDrafts {
					return mbox.Mailbox
				}
			}
		}
		for _, mbox := range mailboxes {
			nameLower := strings.ToLower(mbox.Mailbox)
			if strings.Contains(nameLower, "drafts") || strings.Contains(nameLower, "черновики") {
				return mbox.Mailbox
			}
		}
	}
	return "Drafts"
}
