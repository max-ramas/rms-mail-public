package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"rmsmail/internal/models"
)

type BulkActionRequest struct {
	Action   string   `json:"action"`
	IDs      []string `json:"ids"`
	FolderID string   `json:"folder_id,omitempty"`
	// SetFlagged: for action "flag", set all targets to this state (true=star, false=unstar).
	SetFlagged *bool `json:"set_flagged,omitempty"`
	// Filter mode (used when ids is empty):
	AccountID      string `json:"account_id,omitempty"`
	FilterFolderID string `json:"filter_folder_id,omitempty"`
	Unified        bool   `json:"unified,omitempty"`
}

func (h *Handler) BulkAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req BulkActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSONError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	ctx := r.Context()

	if len(req.IDs) == 0 && req.AccountID != "" {
		h.bulkActionByFilter(w, r, req)
		return
	}

	if len(req.IDs) == 0 {
		w.WriteHeader(http.StatusOK)
		return
	}
	if len(req.IDs) > maxBulkEmailIDs {
		WriteJSONError(w, http.StatusBadRequest, fmt.Sprintf("too many ids (max %d)", maxBulkEmailIDs))
		return
	}

	var err error

	switch req.Action {
	case "delete", "archive", "move":
		if err := h.verifyBulkEmailAccess(ctx, req.IDs); err != nil {
			WriteAccessError(w, err)
			return
		}
		emails, eErr := h.Store.GetEmailsByIDs(ctx, req.IDs)
		if eErr != nil {
			err = eErr
			break
		}

		// Group by AccountID
		batches := make(map[string][]models.Email)
		for _, e := range emails {
			batches[e.AccountID] = append(batches[e.AccountID], e)
		}

		var wg sync.WaitGroup
		var mu sync.Mutex
		var bulkErr error
		setBulkErr := func(e error) {
			if e == nil {
				return
			}
			mu.Lock()
			if bulkErr == nil {
				bulkErr = e
			}
			mu.Unlock()
		}
		detachedCtx := context.WithoutCancel(ctx)
		for accID, batch := range batches {
			wg.Add(1)
			go func(accountID string, emails []models.Email) {
				defer wg.Done()
				if chkErr := h.CheckAccountAccess(detachedCtx, accountID); chkErr != nil {
					setBulkErr(chkErr)
					return
				}
				folders, fErr := h.Store.GetFolders(detachedCtx, accountID)
				if fErr != nil {
					setBulkErr(fErr)
					return
				}

				var targetID, targetName string
				if req.Action == "move" {
					if req.FolderID == "" {
						return
					}
					var resolveErr error
					targetID, targetName, resolveErr = h.resolveBulkMoveTarget(detachedCtx, accountID, req.FolderID, folders)
					if resolveErr != nil {
						setBulkErr(resolveErr)
						return
					}
				} else if req.Action == "delete" {
					f, fErr := h.Store.GetFolderByName(detachedCtx, accountID, "Trash")
					if fErr == nil && f != nil {
						targetID, targetName = f.ID, f.Name
						// Split: emails already in Trash → hard delete, others → move to Trash
						var trashIDs, moveIDs []string
						var moveEmails []models.Email
						for _, e := range emails {
							if e.FolderID == targetID {
								trashIDs = append(trashIDs, e.ID)
							} else {
								moveIDs = append(moveIDs, e.ID)
								moveEmails = append(moveEmails, e)
							}
						}
						if len(trashIDs) > 0 {
							if delErr := h.Store.BulkDeleteEmails(detachedCtx, trashIDs); delErr != nil {
								setBulkErr(delErr)
								return
							}
						}
						if len(moveIDs) > 0 {
							moveErr := h.Store.BulkMoveAndEnqueue(detachedCtx, moveIDs, accountID, targetID, targetName, moveEmails)
							if moveErr != nil {
								slog.Info("BulkAction: BulkMoveAndEnqueue failed", "action", req.Action, "accountID", accountID, "error", moveErr)
								setBulkErr(moveErr)
								return
							}
						}
						return // already handled inside this block
					}
				} else if req.Action == "archive" {
					f, fErr := h.Store.GetFolderByName(detachedCtx, accountID, "Archive")
					if fErr == nil && f != nil {
						targetID, targetName = f.ID, f.Name
					}
				}

				var ids []string
				for _, e := range emails {
					ids = append(ids, e.ID)
				}

				if targetID != "" {
					moveErr := h.Store.BulkMoveAndEnqueue(detachedCtx, ids, accountID, targetID, targetName, emails)
					if moveErr != nil {
						slog.Info("BulkAction: BulkMoveAndEnqueue failed", "action", req.Action, "accountID", accountID, "error", moveErr)
						setBulkErr(moveErr)
						return
					}
				} else if req.Action == "delete" {
					if delErr := h.Store.BulkDeleteEmails(detachedCtx, ids); delErr != nil {
						setBulkErr(delErr)
					}
				}
			}(accID, batch)
		}
		wg.Wait()
		err = bulkErr

	case "read":
		if err := h.verifyBulkEmailAccess(ctx, req.IDs); err != nil {
			WriteAccessError(w, err)
			return
		}
		err = h.Store.BulkMarkEmailsRead(ctx, req.IDs)
	case "unread":
		if err := h.verifyBulkEmailAccess(ctx, req.IDs); err != nil {
			WriteAccessError(w, err)
			return
		}
		err = h.Store.BulkMarkEmailsUnread(ctx, req.IDs)
	case "flag":
		if err := h.verifyBulkEmailAccess(ctx, req.IDs); err != nil {
			WriteAccessError(w, err)
			return
		}
		setFlagged, fErr := h.resolveBulkSetFlagged(ctx, req.IDs, req.SetFlagged)
		if fErr != nil {
			WriteInternalError(w, r, fErr)
			return
		}
		err = h.Store.BulkSetFlagEmails(ctx, req.IDs, setFlagged)
	default:
		WriteJSONError(w, http.StatusBadRequest, "Unknown action")
		return
	}

	if err != nil {
		WriteInternalError(w, r, err)
		return
	}

	// Publish a single bulk-update event instead of N per-ID events.
	// Per-ID events were generating 1000+ SSE publications for large batches,
	// flooding the EventBus and triggering N frontend refetches.
	h.publishEvent(r.Context(), "emails_bulk_updated", fmt.Sprintf(`{"affected":%d,"action":"%s","account_id":"%s"}`, len(req.IDs), req.Action, req.AccountID))

	if h.SyncManager != nil {
		if req.AccountID != "" && req.AccountID != "unified" {
			h.SyncManager.WakeUpAccountNow(req.AccountID)
		} else {
			// Trigger a general refresh if it's unified or unknown
			h.SyncManager.TriggerRefresh()
		}
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) bulkActionByFilter(w http.ResponseWriter, r *http.Request, req BulkActionRequest) {
	ctx := r.Context()
	accountID := req.AccountID
	if req.Unified {
		accountID = "unified"
	}
	sourceFolderID := req.FilterFolderID
	accFolderID := sourceFolderID
	if accFolderID == "" {
		accFolderID = "INBOX"
	}

	var totalAffected int64

	switch req.Action {
	case "read":
		n, rErr := h.runBulkFilterOp(ctx, accountID, sourceFolderID, accFolderID,
			func(c context.Context, accID, folder string) (int64, error) {
				return h.Store.BulkReadByFilter(c, accID, folder)
			})
		if rErr != nil {
			WriteInternalError(w, r, rErr)
			return
		}
		totalAffected = n
	case "unread":
		n, rErr := h.runBulkFilterOp(ctx, accountID, sourceFolderID, accFolderID,
			func(c context.Context, accID, folder string) (int64, error) {
				return h.Store.BulkUnreadByFilter(c, accID, folder)
			})
		if rErr != nil {
			WriteInternalError(w, r, rErr)
			return
		}
		totalAffected = n
	case "flag":
		setFlagged, fErr := h.resolveFilterBulkSetFlagged(ctx, accountID, accFolderID, req.SetFlagged)
		if fErr != nil {
			WriteInternalError(w, r, fErr)
			return
		}
		n, rErr := h.runBulkFilterOp(ctx, accountID, sourceFolderID, accFolderID,
			func(c context.Context, accID, folder string) (int64, error) {
				return h.Store.BulkSetFlagByFilter(c, accID, folder, setFlagged)
			})
		if rErr != nil {
			WriteInternalError(w, r, rErr)
			return
		}
		totalAffected = n
	case "delete", "archive", "move":
		if req.Action == "move" && req.FolderID == "" {
			WriteJSONError(w, http.StatusBadRequest, "folder_id is required for move")
			return
		}

		if h.isMultiAccountBulkFilter(accountID) {
			accIDs, err := h.bulkFilterAccountIDs(ctx, accountID, sourceFolderID)
			if err != nil {
				WriteInternalError(w, r, err)
				return
			}
			for _, accID := range accIDs {
				if chkErr := h.CheckAccountAccess(ctx, accID); chkErr != nil {
					continue
				}
				// Resolve folder name → UUID for per-account queries.
				// accFolderID is a name (e.g. "INBOX"), but buildFilterWhere
				// for non-unified accounts compares against folder_id (UUID column).
				accFolderID := accFolderID
				if !h.isMultiAccountBulkFilter(accID) {
					if folders, fErr := h.Store.GetFolders(ctx, accID); fErr == nil {
						for _, fld := range folders {
							if strings.EqualFold(fld.Name, accFolderID) {
								accFolderID = fld.ID
								break
							}
						}
					}
				}
				accTargetID := ""
				accTargetName := ""
				if req.Action == "delete" || req.Action == "archive" {
					targetName := "Trash"
					if req.Action == "archive" {
						targetName = "Archive"
					}
					f, fErr := h.Store.GetFolderByName(ctx, accID, targetName)
					var foundID, foundName string
					if fErr == nil && f != nil {
						foundID, foundName = f.ID, f.Name
					}
					if foundID == "" {
						if req.Action == "delete" {
							ids, idErr := h.Store.GetEmailIDsByFilter(ctx, accID, accFolderID)
							if idErr == nil {
								h.Store.BulkDeleteEmails(ctx, ids)
							}
						}
						continue
					}
					accTargetID = foundID
					accTargetName = foundName
				} else {
					var resolveErr error
					accTargetID, accTargetName, resolveErr = h.resolveBulkMoveTarget(ctx, accID, req.FolderID, nil)
					if resolveErr != nil {
						slog.Info("BulkAction filter: resolve move target failed", "accountID", accID, "error", resolveErr)
						continue
					}
				}
				ids, idErr := h.Store.GetEmailIDsByFilter(ctx, accID, accFolderID)
				if idErr != nil {
					continue
				}
				preMove, pErr := h.fetchEmailsByIDsInChunks(ctx, ids)
				if pErr != nil {
					continue
				}
				n, mErr := h.Store.BulkMoveByFilter(ctx, accID, accFolderID, accTargetID)
				if mErr != nil {
					slog.Info("BulkAction filter BulkMoveByFilter failed", "action", req.Action, "accountID", accID, "error", mErr)
					continue
				}
				totalAffected += n
				h.enqueueIMAPMovesFromEmails(ctx, preMove, accID, accTargetID, accTargetName)
			}
		} else {
			if chkErr := h.CheckAccountAccess(ctx, accountID); chkErr != nil {
				WriteAccessError(w, chkErr)
				return
			}
			// Resolve folder name → UUID for per-account query.
			accFolderID := accFolderID
			if !h.isMultiAccountBulkFilter(accountID) {
				if folders, fErr := h.Store.GetFolders(ctx, accountID); fErr == nil {
					for _, fld := range folders {
						if strings.EqualFold(fld.Name, accFolderID) {
							accFolderID = fld.ID
							break
						}
					}
				}
			}
			if req.Action == "delete" || req.Action == "archive" {
				targetName := "Trash"
				if req.Action == "archive" {
					targetName = "Archive"
				}
				f, fErr := h.Store.GetFolderByName(ctx, accountID, targetName)
				if fErr != nil {
					WriteInternalError(w, r, fErr)
					return
				}
				var foundID string
				var foundName string
				if f != nil {
					foundID, foundName = f.ID, f.Name
				}
				if foundID == "" {
					// Try to create the target folder (same pattern as deleteEmail)
					created, createErr := h.Store.CreateFolder(ctx, accountID, targetName, targetName, true)
					if createErr == nil && created != nil {
						foundID, foundName = created.ID, created.Name
					}
				}
				if foundID == "" {
					if req.Action == "delete" {
						ids, idErr := h.Store.GetEmailIDsByFilter(ctx, accountID, accFolderID)
						if idErr == nil {
							h.Store.BulkDeleteEmails(ctx, ids)
						}
					}
					WriteJSONError(w, http.StatusBadRequest, req.Action+" folder not found")
					return
				}
				targetFolderID := foundID
				targetFolderName := foundName
				ids, idErr := h.Store.GetEmailIDsByFilter(ctx, accountID, accFolderID)
				if idErr != nil {
					WriteInternalError(w, r, idErr)
					return
				}
				preMove, pErr := h.fetchEmailsByIDsInChunks(ctx, ids)
				if pErr != nil {
					WriteInternalError(w, r, pErr)
					return
				}
				n, mErr := h.Store.BulkMoveByFilter(ctx, accountID, accFolderID, targetFolderID)
				if mErr != nil {
					WriteInternalError(w, r, mErr)
					return
				}
				totalAffected = n
				h.enqueueIMAPMovesFromEmails(ctx, preMove, accountID, targetFolderID, targetFolderName)
			} else {
				targetFolderID, targetFolderName, resolveErr := h.resolveBulkMoveTarget(ctx, accountID, req.FolderID, nil)
				if resolveErr != nil {
					WriteJSONError(w, http.StatusBadRequest, resolveErr.Error())
					return
				}
				ids, idErr := h.Store.GetEmailIDsByFilter(ctx, accountID, accFolderID)
				if idErr != nil {
					WriteInternalError(w, r, idErr)
					return
				}
				preMove, pErr := h.fetchEmailsByIDsInChunks(ctx, ids)
				if pErr != nil {
					WriteInternalError(w, r, pErr)
					return
				}
				n, mErr := h.Store.BulkMoveByFilter(ctx, accountID, accFolderID, targetFolderID)
				if mErr != nil {
					WriteInternalError(w, r, mErr)
					return
				}
				totalAffected = n
				h.enqueueIMAPMovesFromEmails(ctx, preMove, accountID, targetFolderID, targetFolderName)
			}
		}
	default:
		WriteJSONError(w, http.StatusBadRequest, "Unknown action")
		return
	}

	h.publishEvent(r.Context(), "emails_bulk_updated", fmt.Sprintf(`{"affected":%d,"action":"%s","account_id":"%s"}`, totalAffected, req.Action, req.AccountID))

	if h.SyncManager != nil {
		if req.AccountID != "" && req.AccountID != "unified" && !strings.HasPrefix(req.AccountID, "group:") {
			h.SyncManager.WakeUpAccountNow(req.AccountID)
		} else {
			h.SyncManager.TriggerRefresh()
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]int64{"affected": totalAffected})
}
