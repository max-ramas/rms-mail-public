"use client";

import React, { useState } from "react";
import { Plus, Trash2, FileText } from "lucide-react";
import { useTranslations } from "next-intl";
import { useAccounts } from "@/hooks/useEmailQueries";
import { editionLetter } from "@/hooks/useEmails";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import axios from "axios";
import { Account } from "@/hooks/useEmailTypes";

import { API_BASE } from "@/hooks/useEmailTypes";

function useTemplates(accountId: string) {
  return useQuery({
    queryKey: ["templates", accountId],
    queryFn: async () => { if (!accountId) return []; const r = await axios.get(`${API_BASE}/api/templates?account_id=${accountId}`); return r.data; },
    enabled: !!accountId,
  });
}

function useCreateTemplate() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (d: { account_id: string; name: string; subject: string; body: string }) => { const r = await axios.post(`${API_BASE}/api/templates/create`, d); return r.data; },
    onSuccess: () => qc.invalidateQueries({ queryKey: ["templates"] }),
  });
}

function useDeleteTemplate() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (id: string) => { await axios.delete(`${API_BASE}/api/templates/delete/${id}`); },
    onSuccess: () => qc.invalidateQueries({ queryKey: ["templates"] }),
  });
}

interface Template {
  id: string;
  account_id: string;
  name: string;
  subject?: string;
  body?: string;
}

export function TemplatesTab() {
  const t = useTranslations("settings");
  const { data: accounts } = useAccounts();
  const [selectedAccountId, setSelectedAccountId] = useState<string | null>(null);
  const [name, setName] = useState("");
  const [subject, setSubject] = useState("");
  const [body, setBody] = useState("");

  const accountId = selectedAccountId || (accounts?.length ? accounts[0].id : "");

  const tempQuery = useTemplates(accountId);
  const createTemp = useCreateTemplate();
  const deleteTemp = useDeleteTemplate();

  const handleSave = async () => {
    if (!name || !accountId) return;
    try { await createTemp.mutateAsync({ account_id: accountId, name, subject, body }); setName(""); setSubject(""); setBody(""); }
    catch {}
  };

  return (
    <div className="space-y-6">
      <Card className="border">
        <CardHeader className="pb-3"><CardTitle className="text-base">{t("new_template")}</CardTitle></CardHeader>
        <CardContent className="space-y-3">
          {editionLetter() !== 'M' && (
          <div className="text-sm text-muted-foreground">
            {t("tab_accounts")}: <select value={accountId} onChange={e => setSelectedAccountId(e.target.value)} className="h-9 rounded-md border bg-background px-2 py-1 text-sm text-foreground shadow-sm ms-1">
              {accounts?.map((a: Account) => <option key={a.id} value={a.id}>{a.email}</option>)}
            </select>
          </div>
          )}
          <Input value={name} onChange={e => setName(e.target.value)} placeholder={t("placeholder_template_name")} />
          <Input value={subject} onChange={e => setSubject(e.target.value)} placeholder={t("placeholder_template_subject")} />
          <textarea value={body} onChange={e => setBody(e.target.value)} placeholder={t("placeholder_template_body")} className="w-full bg-muted border rounded px-3 py-2 text-sm resize-none h-32" />
          <Button size="sm" onClick={handleSave} disabled={createTemp.isPending}><Plus className="w-3 h-3 me-1" /> {t("save")}</Button>
        </CardContent>
      </Card>

      <Card className="border">
        <CardHeader className="pb-3"><CardTitle className="text-base">{t("saved")}</CardTitle></CardHeader>
        <CardContent>
          {tempQuery.data?.length === 0 && <p className="text-sm text-muted-foreground">{t("no_templates")}</p>}
          {(tempQuery.data || []).map((template: Template) => (
            <div key={template.id} className="flex items-start justify-between py-2 border-b border-border-muted/50 last:border-0">
              <div className="flex-1 min-w-0">
                <div className="text-sm font-medium flex items-center gap-2"><FileText className="w-3 h-3" /> {template.name}</div>
                {template.subject && <div className="text-xs text-muted-foreground">{t("subj")}: {template.subject}</div>}
                <div className="text-xs text-muted-foreground truncate mt-1">{template.body?.slice(0, 80)}</div>
              </div>
              <Button variant="ghost" size="sm" className="shrink-0" onClick={() => deleteTemp.mutate(template.id)}><Trash2 className="w-3 h-3" /></Button>
            </div>
          ))}
        </CardContent>
      </Card>
    </div>
  );
}
