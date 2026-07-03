"use client";

import React, { useState } from "react";
import { Folder, Trash2, Edit2, Lock, Plus, Save, X } from "lucide-react";
import { useTranslations } from "next-intl";
import { useAccounts, useFolders } from "@/hooks/useEmailQueries";
import { useCreateFolder, useRenameFolder, useDeleteFolder } from "@/hooks/useEmailMutations";
import { editionLetter } from "@/hooks/useEmails";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import { useToast } from "@/hooks/useToast";

function isSystemFolder(name: string): boolean {
  const lowerName = name.toLowerCase();
  const systemNames = ["inbox", "sent", "trash", "junk", "spam", "drafts", "[gmail]"];
  return systemNames.some((sys) => lowerName.startsWith(sys));
}

export function FoldersManagementTab() {
  const t = useTranslations("settings");
  const { data: accounts = [] } = useAccounts();
  const defaultAccountId = accounts.length > 0 ? accounts[0].id : "";
  const [selectedAccountId, setSelectedAccountId] = useState(defaultAccountId);
  const toast = useToast();
  
  // If we're in unified mode and have multiple accounts, we should show the selector.
  // We'll rely on the editionLetter() logic if needed, but it's simpler to just check if there are multiple accounts.
  const isUnified = editionLetter() === "U";
  const activeAccountId = isUnified ? selectedAccountId : defaultAccountId;

  const { data: folders = [], isLoading } = useFolders(activeAccountId);
  const createFolderMut = useCreateFolder();
  const renameFolderMut = useRenameFolder();
  const deleteFolderMut = useDeleteFolder();

  const [newFolderName, setNewFolderName] = useState("");
  const [editingFolderId, setEditingFolderId] = useState<string | null>(null);
  const [editingName, setEditingName] = useState("");

  const handleCreate = () => {
    if (!newFolderName.trim() || !activeAccountId) return;
    createFolderMut.mutate(
      { accountId: activeAccountId, name: newFolderName.trim() },
      {
        onSuccess: () => {
          setNewFolderName("");
          toast.addToast(t("folder_created", { defaultMessage: "Folder created successfully" }), "success");
        },
        onError: (err: Error) => {
          toast.addToast(err.message || t("folder_create_error", { defaultMessage: "Failed to create folder" }), "error");
        },
      }
    );
  };

  const handleRename = (folderId: string) => {
    if (!editingName.trim() || !activeAccountId) return;
    renameFolderMut.mutate(
      { accountId: activeAccountId, folderId, name: editingName.trim() },
      {
        onSuccess: () => {
          setEditingFolderId(null);
          setEditingName("");
          toast.addToast(t("folder_renamed", { defaultMessage: "Folder renamed successfully" }), "success");
        },
        onError: (err: Error) => {
          toast.addToast(err.message || t("folder_rename_error", { defaultMessage: "Failed to rename folder" }), "error");
        },
      }
    );
  };

  const handleDelete = (folderId: string) => {
    if (!activeAccountId) return;
    deleteFolderMut.mutate(
      { accountId: activeAccountId, folderId },
      {
        onSuccess: () => {
          toast.addToast(t("folder_deleted", { defaultMessage: "Folder deleted successfully" }), "success");
        },
        onError: (err: Error) => {
          toast.addToast(err.message || t("folder_delete_error", { defaultMessage: "Failed to delete folder" }), "error");
        },
      }
    );
  };


  return (
    <div className="space-y-6">
      {isUnified && accounts.length > 0 && (
        <div className="flex items-center space-x-4">
          <span className="text-sm font-medium">{t("select_account", { defaultMessage: "Select Account:" })}</span>
          <Select value={activeAccountId} onValueChange={setSelectedAccountId}>
            <SelectTrigger className="w-[280px]">
              <SelectValue placeholder={t("select_account_placeholder", { defaultMessage: "Select account" })} />
            </SelectTrigger>
            <SelectContent>
              {accounts.map((acc) => (
                <SelectItem key={acc.id} value={acc.id}>
                  {acc.email}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      )}

      <div className="flex gap-2">
        <Input
          placeholder={t("new_folder_placeholder", { defaultMessage: "New folder name..." })}
          value={newFolderName}
          onChange={(e) => setNewFolderName(e.target.value)}
          onKeyDown={(e) => e.key === "Enter" && handleCreate()}
          className="max-w-sm"
        />
        <Button onClick={handleCreate} disabled={createFolderMut.isPending || !newFolderName.trim()}>
          <Plus className="w-4 h-4 mr-2" />
          {t("create_folder", { defaultMessage: "Create Folder" })}
        </Button>
      </div>

      {isLoading ? (
        <div className="text-sm text-muted-foreground animate-pulse">{t("loading_folders", { defaultMessage: "Loading folders..." })}</div>
      ) : (
        <div className="border rounded-md divide-y">
          {folders.length === 0 && (
            <div className="p-4 text-sm text-muted-foreground text-center">{t("no_folders", { defaultMessage: "No folders found." })}</div>
          )}
          {folders.map((folder) => {
            const isSystem = isSystemFolder(folder.name);
            const isEditing = editingFolderId === folder.id;

            return (
              <div key={folder.id} className="flex items-center justify-between p-3 hover:bg-muted/50 transition-colors">
                <div className="flex items-center gap-3 overflow-hidden">
                  {isSystem ? (
                    <Lock className="w-4 h-4 text-muted-foreground shrink-0" />
                  ) : (
                    <Folder className="w-4 h-4 text-primary shrink-0" />
                  )}
                  
                  {isEditing ? (
                    <Input
                      autoFocus
                      value={editingName}
                      onChange={(e) => setEditingName(e.target.value)}
                      onKeyDown={(e) => {
                        if (e.key === "Enter") handleRename(folder.id);
                        if (e.key === "Escape") setEditingFolderId(null);
                      }}
                      className="h-8 max-w-[200px]"
                    />
                  ) : (
                    <span className="truncate text-sm font-medium">{folder.name}</span>
                  )}
                </div>

                <div className="flex items-center gap-2 shrink-0">
                  {isSystem ? (
                    <span className="text-xs px-2 py-1 bg-muted text-muted-foreground rounded-full">
                      {t("system", { defaultMessage: "System" })}
                    </span>
                  ) : (
                    <>
                      {isEditing ? (
                        <>
                          <Button size="icon" variant="ghost" onClick={() => handleRename(folder.id)} disabled={renameFolderMut.isPending}>
                            <Save className="w-4 h-4 text-green-500" />
                          </Button>
                          <Button size="icon" variant="ghost" onClick={() => setEditingFolderId(null)}>
                            <X className="w-4 h-4" />
                          </Button>
                        </>
                      ) : (
                        <Button size="icon" variant="ghost" onClick={() => {
                          setEditingFolderId(folder.id);
                          setEditingName(folder.name);
                        }}>
                          <Edit2 className="w-4 h-4" />
                        </Button>
                      )}

                      <AlertDialog>
                        <AlertDialogTrigger asChild>
                          <Button size="icon" variant="ghost" className="hover:text-red-500 hover:bg-red-50 dark:hover:bg-red-950/50">
                            <Trash2 className="w-4 h-4" />
                          </Button>
                        </AlertDialogTrigger>
                        <AlertDialogContent>
                          <AlertDialogHeader>
                            <AlertDialogTitle>{t("delete_confirm_title", { defaultMessage: "Are you absolutely sure?" })}</AlertDialogTitle>
                            <AlertDialogDescription className="text-red-600 font-medium">
                              {t("delete_confirm_desc", { 
                                folder: folder.name, 
                                defaultMessage: "This action will permanently delete the folder \"{folder}\" and all emails inside it, both locally and on the remote IMAP server. This action cannot be undone."
                              })}
                            </AlertDialogDescription>
                          </AlertDialogHeader>
                          <AlertDialogFooter>
                            <AlertDialogCancel>{t("cancel", { defaultMessage: "Cancel" })}</AlertDialogCancel>
                            <AlertDialogAction 
                              onClick={() => handleDelete(folder.id)} 
                              className="bg-red-600 hover:bg-red-700 text-white"
                            >
                              {deleteFolderMut.isPending ? t("deleting", { defaultMessage: "Deleting..." }) : t("delete_permanently", { defaultMessage: "Delete Permanently" })}
                            </AlertDialogAction>
                          </AlertDialogFooter>
                        </AlertDialogContent>
                      </AlertDialog>
                    </>
                  )}
                </div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
