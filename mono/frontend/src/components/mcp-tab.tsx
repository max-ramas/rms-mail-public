"use client";

import React, { useState, useEffect } from "react";
import {
  Key,
  Trash,
  Power,
  PowerOff,
  Copy,
  Check,
  Link,
  Eye,
} from "lucide-react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import axios from "axios";

import { API_BASE } from "@/hooks/useEmailTypes";

interface MCPKey {
  id: string;
  name: string;
  account_id?: string;
  key_prefix: string;
  full_key?: string;
  is_active: boolean;
  created_at: string;
}

interface MCPConnectInfo {
  mcp_url: string;
  mcp_sse_url: string;
  sse_url: string;
  auth_header: string;
  tools: string[];
  config_json: Record<string, unknown>;
  config_claude: Record<string, unknown>;
}

export function MCPTab({ accountId }: { accountId: string }) {
  const t = useTranslations("settings");
  const [keys, setKeys] = useState<MCPKey[]>([]);
  const [connectInfo, setConnectInfo] = useState<MCPConnectInfo | null>(null);
  const [newKeyName, setNewKeyName] = useState("");
  const [generatedKey, setGeneratedKey] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const [selectedKeyId, setSelectedKeyId] = useState("");
  const [shownKeys, setShownKeys] = useState<Record<string, string>>({});
  const [error, setError] = useState("");

  const handleViewKey = async (id: string) => {
    try {
      const { data } = await axios.get(`${API_BASE}/api/mcp/keys/view/${id}`);
      setShownKeys((prev) => ({ ...prev, [id]: data.api_key }));
    } catch {}
  };

  const fetchKeys = async () => {
    try {
      const { data } = await axios.get(`${API_BASE}/api/mcp/keys`);
      setKeys(data || []);
    } catch {}
  };

  useEffect(() => {
    let active = true;
    axios
      .get(`${API_BASE}/api/mcp/keys`)
      .then(({ data }) => {
        if (active) setKeys(data || []);
      })
      .catch(() => {});
    return () => {
      active = false;
    };
  }, []);

  const accountKeys = keys.filter(
    (k) => !accountId || !k.account_id || k.account_id === accountId,
  );
  const activeAccountKeys = accountKeys.filter((k) => k.is_active);
  const resolvedSelectedKeyId = activeAccountKeys.some(
    (k) => k.id === selectedKeyId,
  )
    ? selectedKeyId
    : "";

  useEffect(() => {
    let active = true;
    const url = resolvedSelectedKeyId
      ? `${API_BASE}/api/mcp/connect?key_id=${resolvedSelectedKeyId}`
      : `${API_BASE}/api/mcp/connect`;
    axios
      .get(url)
      .then(({ data }) => {
        if (active) setConnectInfo(data);
      })
      .catch(() => {});
    return () => {
      active = false;
    };
  }, [resolvedSelectedKeyId]);

  const handleCreate = async () => {
    setError("");
    try {
      const { data } = await axios.post(`${API_BASE}/api/mcp/keys/create`, {
        name: newKeyName || t("mcp_key_default"),
        account_id: accountId,
      });
      setGeneratedKey(data.api_key);
      setNewKeyName("");
      fetchKeys();
    } catch (err: unknown) {
      const msg = err && typeof err === "object" && "response" in err
        ? (err as { response?: { data?: string } }).response?.data
        : undefined;
      setError(msg || t("toast_failed"));
    }
  };

  const handleDelete = async (id: string) => {
    try {
      await axios.delete(`${API_BASE}/api/mcp/keys/delete/${id}`);
      fetchKeys();
    } catch {}
  };

  const handleToggle = async (id: string) => {
    try {
      await axios.post(`${API_BASE}/api/mcp/keys/toggle/${id}`);
      fetchKeys();
    } catch {}
  };

  const handleCopy = (text: string) => {
    navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="space-y-6">
      <Card className="border">
        <CardHeader className="pb-3">
          <CardTitle className="text-base flex items-center gap-2">
            <Link className="w-4 h-4 text-primary" /> {t("mcp_connection_info")}
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="flex items-center gap-2">
            <span className="text-sm text-muted-foreground shrink-0">
              {t("mcp_key")}:
            </span>
            <select
              className="flex-1 h-8 rounded border bg-background px-2 text-sm"
              value={resolvedSelectedKeyId}
              onChange={(e) => setSelectedKeyId(e.target.value)}
            >
              <option value="">{t("mcp_no_key")}</option>
              {activeAccountKeys.map((k) => (
                  <option key={k.id} value={k.id}>
                    {k.name} ({k.key_prefix}...)
                  </option>
              ))}
            </select>
          </div>
          {connectInfo && (
            <>
              <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
                {/* SSE / IDE config */}
                <div>
                  <div className="flex items-center justify-between mb-1">
                    <span className="text-sm font-medium">
                      🔌 {t("mcp_config_ide")}
                    </span>
                    <Button
                      size="sm"
                      variant="ghost"
                      onClick={() =>
                        handleCopy(
                          JSON.stringify(connectInfo.config_json, null, 2),
                        )
                      }
                    >
                      {copied ? (
                        <Check className="w-3 h-3 me-1" />
                      ) : (
                        <Copy className="w-3 h-3 me-1" />
                      )}
                      {copied ? t("copied") : t("copy")}
                    </Button>
                  </div>
                  <pre className="text-[11px] bg-muted p-3 rounded overflow-auto max-h-52 whitespace-pre break-all">
                    {JSON.stringify(connectInfo.config_json, null, 2)}
                  </pre>
                </div>
                {/* Claude Desktop config */}
                <div>
                  <div className="flex items-center justify-between mb-1">
                    <span className="text-sm font-medium">
                      🖥 {t("mcp_config_claude")}
                    </span>
                    <Button
                      size="sm"
                      variant="ghost"
                      onClick={() =>
                        handleCopy(
                          JSON.stringify(connectInfo.config_claude, null, 2),
                        )
                      }
                    >
                      {copied ? (
                        <Check className="w-3 h-3 me-1" />
                      ) : (
                        <Copy className="w-3 h-3 me-1" />
                      )}
                      {copied ? t("copied") : t("copy")}
                    </Button>
                  </div>
                  <pre className="text-[11px] bg-muted p-3 rounded overflow-auto max-h-52 whitespace-pre break-all">
                    {JSON.stringify(connectInfo.config_claude, null, 2)}
                  </pre>
                </div>
              </div>
              <div className="grid grid-cols-2 gap-2 text-sm mt-3">
                <span className="text-muted-foreground">{t("mcp_url")}:</span>
                <code className="text-xs bg-muted px-2 py-0.5 rounded">
                  {connectInfo.mcp_sse_url}
                </code>
                <span className="text-muted-foreground">{t("mcp_auth")}:</span>
                <code className="text-xs bg-muted px-2 py-0.5 rounded">
                  {connectInfo.auth_header}
                </code>
              </div>
              <div className="text-sm text-muted-foreground">
                {t("mcp_tools")}: {connectInfo.tools.join(", ")}
              </div>
            </>
          )}
        </CardContent>
      </Card>

      <Card className="border">
        <CardHeader className="pb-3">
          <CardTitle className="text-base flex items-center gap-2">
            <Key className="w-4 h-4 text-primary" /> {t("generate_key")}
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="flex gap-2">
            <Input
              placeholder={t("key_name")}
              value={newKeyName}
              onChange={(e) => setNewKeyName(e.target.value)}
              className="flex-1"
            />
            <Button size="sm" onClick={handleCreate}>
              {t("mcp_generate")}
            </Button>
          </div>
          {error && <p className="text-xs text-red-400 mt-1">{error}</p>}
          {generatedKey && (
            <div className="p-3 bg-primary/10 border border-primary/30 rounded-lg">
              <p className="text-xs text-muted-foreground mb-1">
                {t("copy_warning")}
              </p>
              <div className="flex gap-2 items-center">
                <code className="flex-1 text-sm font-mono bg-muted px-3 py-1.5 rounded break-all">
                  {generatedKey}
                </code>
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={() => handleCopy(generatedKey)}
                >
                  {copied ? (
                    <Check className="w-3 h-3" />
                  ) : (
                    <Copy className="w-3 h-3" />
                  )}
                </Button>
              </div>
            </div>
          )}
        </CardContent>
      </Card>

      <Card className="border">
        <CardHeader className="pb-3">
          <CardTitle className="text-base">
            {t("active_keys")} ({keys.length})
          </CardTitle>
        </CardHeader>
        <CardContent>
          {keys.length === 0 ? (
            <p className="text-sm text-muted-foreground">{t("no_keys")}</p>
          ) : (
            <div className="space-y-2">
              {keys.map((k) => (
                <div
                  key={k.id}
                  className="flex items-center gap-3 p-2 rounded-lg bg-muted/30 border"
                >
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="text-sm font-medium">{k.name}</span>
                      <span
                        className={`w-2 h-2 rounded-full ${k.is_active ? "bg-green-400" : "bg-red-400"}`}
                      />
                      <span className="text-xs text-muted-foreground">
                        {k.is_active ? t("active") : t("disabled")}
                      </span>
                    </div>
                    <code className="text-xs text-muted-foreground break-all">
                      {shownKeys[k.id] || `${k.key_prefix}••••••`}
                    </code>
                  </div>
                  {!shownKeys[k.id] ? (
                    <Button
                      size="sm"
                      variant="ghost"
                      onClick={() => handleViewKey(k.id)}
                      title="Show full key"
                    >
                      <Eye className="w-3 h-3" />
                    </Button>
                  ) : (
                    <Button
                      size="sm"
                      variant="ghost"
                      onClick={() => handleCopy(shownKeys[k.id])}
                      title={t("copy")}
                    >
                      <Copy className="w-3 h-3" />
                    </Button>
                  )}
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() => handleToggle(k.id)}
                    title={k.is_active ? t("mcp_disable") : t("mcp_enable")}
                  >
                    {k.is_active ? (
                      <PowerOff className="w-3 h-3" />
                    ) : (
                      <Power className="w-3 h-3" />
                    )}
                  </Button>
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() => handleDelete(k.id)}
                    title={t("del")}
                  >
                    <Trash className="w-3 h-3 text-red-400" />
                  </Button>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
