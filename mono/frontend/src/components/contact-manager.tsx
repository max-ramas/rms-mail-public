"use client";

import React, { useState } from "react";
import { useContacts } from "@/hooks/useEmailQueries";
import { useCreateContact, useUpdateContact, useDeleteContact } from "@/hooks/useAdminQueries";
import { type Contact } from "@/hooks/useEmailTypes";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card, CardContent } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";

export function ContactManager() {
  const t = useTranslations("settings");
  const { data: contacts, isLoading } = useContacts();
  const createContact = useCreateContact();
  const updateContact = useUpdateContact();
  const deleteContact = useDeleteContact();

  const [address, setAddress] = useState("");
  const [name, setName] = useState("");
  const [phone, setPhone] = useState("");
  const [notes, setNotes] = useState("");
  const [company, setCompany] = useState("");
  const [position, setPosition] = useState("");
  const [tags, setTags] = useState("");
  const [editingId, setEditingId] = useState<string | null>(null);
  const [filter, setFilter] = useState("");
  const [deleteConfirmId, setDeleteConfirmId] = useState<string | null>(null);

  const handleSave = async () => {
    if (!address.trim()) return;
    
    // Parse comma-separated tags into a JSON array string before saving
    let formattedTags = "[]";
    if (tags.trim()) {
      const parsed = tags.split(",").map(t => t.trim()).filter(t => t);
      formattedTags = JSON.stringify(parsed);
    }

    if (editingId) {
      await updateContact.mutateAsync({
        id: editingId,
        contact: { address, name, phone, notes, company, position, tags: formattedTags },
      });
      setEditingId(null);
    } else {
      await createContact.mutateAsync({ address, name, phone, notes, company, position, tags: formattedTags });
    }
    setAddress("");
    setName("");
    setPhone("");
    setNotes("");
    setCompany("");
    setPosition("");
    setTags("");
  };

  const handleEdit = (c: Contact) => {
    setEditingId(c.id || null);
    setAddress(c.address);
    setName(c.name);
    setPhone(c.phone || "");
    setNotes(c.notes || "");
    setCompany(c.company || "");
    setPosition(c.position || "");
    
    // Parse tags JSON array string back to comma-separated
    let displayTags = "";
    if (c.tags && c.tags !== "[]") {
      try {
        const parsed = JSON.parse(c.tags);
        if (Array.isArray(parsed)) {
          displayTags = parsed.join(", ");
        }
      } catch {
        displayTags = c.tags;
      }
    }
    setTags(displayTags);
  };

  const handleCancel = () => {
    setEditingId(null);
    setAddress("");
    setName("");
    setPhone("");
    setNotes("");
    setCompany("");
    setPosition("");
    setTags("");
  };

  const handleDelete = async () => {
    if (deleteConfirmId) {
      await deleteContact.mutateAsync(deleteConfirmId);
      setDeleteConfirmId(null);
    }
  };

  const filteredContacts = filter
    ? (contacts || []).filter(
        (c) =>
          c.address.toLowerCase().includes(filter.toLowerCase()) ||
          c.name.toLowerCase().includes(filter.toLowerCase()) ||
          (c.company && c.company.toLowerCase().includes(filter.toLowerCase())) ||
          (c.position && c.position.toLowerCase().includes(filter.toLowerCase())) ||
          (c.tags && c.tags.toLowerCase().includes(filter.toLowerCase()))
      )
    : contacts || [];

  if (isLoading)
    return <div className="text-muted-foreground text-sm">{t("loading")}</div>;

  return (
    <div className="space-y-4">
      {/* Search */}
      <Input
        value={filter}
        onChange={(e) => setFilter(e.target.value)}
        placeholder={t("contacts_search")}
        className="w-full"
      />

      {/* Add / Edit Form */}
      <Card className="border bg-muted/30">
        <CardContent className="pt-4">
          <div className="space-y-4">
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="text-xs text-muted-foreground font-medium mb-1 block">
                  {t("contacts_name")}
                </label>
                <Input
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder={t("contacts_name")}
                />
              </div>
              <div>
                <label className="text-xs text-muted-foreground font-medium mb-1 block">
                  {t("contacts_position")}
                </label>
                <Input
                  value={position}
                  onChange={(e) => setPosition(e.target.value)}
                  placeholder={t("contacts_position")}
                />
              </div>
              <div>
                <label className="text-xs text-muted-foreground font-medium mb-1 block">
                  {t("contacts_email")}
                </label>
                <Input
                  value={address}
                  onChange={(e) => setAddress(e.target.value)}
                  placeholder={t("contacts_email")}
                />
              </div>
              <div>
                <label className="text-xs text-muted-foreground font-medium mb-1 block">
                  {t("contacts_phone")}
                </label>
                <Input
                  value={phone}
                  onChange={(e) => setPhone(e.target.value)}
                  placeholder={t("contacts_phone")}
                />
              </div>
              <div>
                <label className="text-xs text-muted-foreground font-medium mb-1 block">
                  {t("contacts_company")}
                </label>
                <Input
                  value={company}
                  onChange={(e) => setCompany(e.target.value)}
                  placeholder={t("contacts_company")}
                />
              </div>
              <div>
                <label className="text-xs text-muted-foreground font-medium mb-1 block">
                  {t("contacts_tags")}
                </label>
                <Input
                  value={tags}
                  onChange={(e) => setTags(e.target.value)}
                  placeholder={t("contacts_tags_placeholder")}
                />
              </div>
              <div className="col-span-2">
                <label className="text-xs text-muted-foreground font-medium mb-1 block">
                  {t("contacts_notes")}
                </label>
                <textarea
                  value={notes}
                  onChange={(e) => setNotes(e.target.value)}
                  placeholder={t("contacts_notes")}
                  className="flex min-h-[80px] w-full rounded-md border border-input bg-card-bg px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
                />
              </div>
            </div>
            
            <div className="flex gap-2 pt-2 border-t mt-4 border-border/50">
              <Button size="sm" onClick={handleSave} className="mt-4">
                {editingId ? t("update") : t("contacts_add")}
              </Button>
              {editingId && (
                <Button variant="ghost" size="sm" onClick={handleCancel} className="mt-4">
                  {t("cancel")}
                </Button>
              )}
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Contacts List */}
      <div className="space-y-2">
        {filteredContacts.length === 0 && (
          <div className="text-muted-foreground text-sm py-8 text-center">
            {t("contacts_empty")}
          </div>
        )}
        {filteredContacts.map((c) => {
          let parsedTags: string[] = [];
          if (c.tags && c.tags !== "[]") {
            try {
              const p = JSON.parse(c.tags);
              if (Array.isArray(p)) parsedTags = p;
            } catch {}
          }

          return (
            <Card key={c.address + (c.id || "")} className="border hover:bg-muted/10 transition-colors">
              <CardContent className="py-3 px-4">
                <div className="flex items-start gap-3 justify-between">
                  <div className="flex-1 min-w-0 space-y-1">
                    <div className="flex items-center gap-2 flex-wrap">
                      <div className="text-sm font-medium truncate">
                        {c.name || c.address}
                      </div>
                      {c.company && (
                        <span className="text-xs font-semibold bg-primary/10 text-primary px-2 py-0.5 rounded">
                          {c.company}
                        </span>
                      )}
                      {c.position && (
                        <span className="text-xs text-muted-foreground">
                          {c.position}
                        </span>
                      )}
                    </div>
                    
                    <div className="text-xs text-muted-foreground flex gap-3 flex-wrap">
                      <span>{c.address}</span>
                      {c.phone && <span>• {c.phone}</span>}
                    </div>

                    {parsedTags.length > 0 && (
                      <div className="flex gap-1 flex-wrap pt-1">
                        {parsedTags.map((tag) => (
                          <span key={tag} className="text-[10px] bg-secondary text-secondary-foreground px-1.5 py-0.5 rounded-sm border border-border/50">
                            {tag}
                          </span>
                        ))}
                      </div>
                    )}

                    {c.notes && (
                      <div className="text-xs text-muted-foreground italic pt-1 border-t border-border/30 mt-2 line-clamp-2">
                        {c.notes}
                      </div>
                    )}
                  </div>
                  {c.id && (
                    <div className="flex gap-1 shrink-0 pt-0.5 ml-4">
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => handleEdit(c)}
                      >
                        {t("edit")}
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        className="text-red-500 hover:text-red-600 hover:bg-red-50 dark:hover:bg-red-950/30"
                        onClick={() => setDeleteConfirmId(c.id!)}
                      >
                        {t("contacts_delete")}
                      </Button>
                    </div>
                  )}
                </div>
              </CardContent>
            </Card>
          );
        })}
      </div>

      {/* Delete Confirmation Dialog */}
      <Dialog
        open={!!deleteConfirmId}
        onOpenChange={(open) => {
          if (!open) setDeleteConfirmId(null);
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("contacts_delete")}</DialogTitle>
            <DialogDescription>
              {t("contacts_confirm_delete")}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setDeleteConfirmId(null)}
            >
              {t("cancel")}
            </Button>
            <Button variant="destructive" size="sm" onClick={handleDelete}>
              {t("contacts_delete")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
