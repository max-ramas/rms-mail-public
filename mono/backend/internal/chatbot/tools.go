package chatbot

import (
	"rmsmail/internal/ai"
)

// Define AI Tool Schemas
var FetchEmailsTool = ai.Tool{
	Type: "function",
	Function: ai.ToolFunction{
		Name:        "fetch_emails",
		Description: "Fetch recent emails from the mailbox. Use 'unified' as account_id to search across ALL connected accounts at once. Use a specific account or group name to filter. Call once with unified for broad queries — avoid multiple per-account calls.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"search_query": map[string]interface{}{
					"type":        "string",
					"description": "Optional text to search for in email subjects, senders, and snippets.",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Number of emails to fetch. Default 10, max 50.",
				},
				"account_id": map[string]interface{}{
					"type":        "string",
					"description": "Account to query. Use 'unified' for ALL accounts, a specific account email/ID, or 'group:name' for a project group. Default is user's primary account.",
				},
				"folder_name": map[string]interface{}{
					"type":        "string",
					"description": "Folder name, e.g. 'INBOX' or 'Sent'. Default 'INBOX'.",
				},
			},
		},
	},
}

var AvailableTools = []ai.Tool{FetchEmailsTool}
