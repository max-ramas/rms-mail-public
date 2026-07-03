-- Migration 016: AI Categories — configurable taxonomy + tag index for filtering

-- Default AI category taxonomy (stored in system_settings as JSON)
INSERT INTO system_settings (key, value) VALUES ('ai_categories', '[
  {"name": "Invoice",   "color": "#3b82f6", "icon": "receipt",    "move_to": "", "auto_read": false},
  {"name": "Support",   "color": "#10b981", "icon": "headset",    "move_to": "", "auto_read": false},
  {"name": "Urgent",    "color": "#ef4444", "icon": "alert",      "move_to": "", "auto_read": true},
  {"name": "Newsletter","color": "#8b5cf6", "icon": "newspaper",  "move_to": "", "auto_read": true},
  {"name": "Personal",  "color": "#f59e0b", "icon": "user",       "move_to": "", "auto_read": false},
  {"name": "Business",  "color": "#06b6d4", "icon": "briefcase",  "move_to": "", "auto_read": false},
  {"name": "Official",  "color": "#6b7280", "icon": "building",   "move_to": "", "auto_read": false}
]')
ON CONFLICT (key) DO NOTHING;

-- Index for filtering emails by AI tag (GET /api/emails?tag=Invoice)
CREATE INDEX IF NOT EXISTS idx_email_tags_email_tag ON email_tags (email_id, tag);
CREATE INDEX IF NOT EXISTS idx_email_tags_tag_email ON email_tags (tag, email_id);
