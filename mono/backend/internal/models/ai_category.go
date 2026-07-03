package models

// AICategory defines a single AI categorization tag with optional automation rules.
// Stored as JSON array in system_settings key "ai_categories".
type AICategory struct {
	Name     string `json:"name"`
	Color    string `json:"color"`
	Icon     string `json:"icon"`
	MoveTo   string `json:"move_to"` // folder_id or empty
	AutoRead bool   `json:"auto_read"`
}
