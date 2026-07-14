"use client";

import React, { useState, useEffect } from "react";
import { useTranslations } from "next-intl";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { useToast } from "@/hooks/useToast";
import { getEnvOnlyFlags } from "@/hooks/useEmails";
import axios from "axios";
import "@/lib/api-client";
import {
  Send,
  Sparkles,
  MessageSquare,
  Key,
  HelpCircle,
  Loader2,
  CheckCircle2,
  Shield,
  ExternalLink,
  ChevronDown,
  ChevronUp,
  Info,
} from "lucide-react";

import { API_BASE } from "@/hooks/useEmailTypes";

export function TelegramTab() {
  const envOnly = getEnvOnlyFlags().tg;
  const t = useTranslations("settings");
  const toast = useToast();

  const [telegramUserId, setTelegramUserId] = useState<string>("");
  const [telegramEnabled, setTelegramEnabled] = useState<boolean>(false);
  const [telegramAiNotif, setTelegramAiNotif] = useState<boolean>(false);
  const [telegramAiChat, setTelegramAiChat] = useState<boolean>(false);

  const [botConfigured, setBotConfigured] = useState<boolean>(false);
  const [botEnvConfigured, setBotEnvConfigured] = useState<boolean>(false);
  const [botUsername, setBotUsername] = useState<string>("");
  const [botToken, setBotToken] = useState<string>("");

  const [loading, setLoading] = useState<boolean>(true);
  const [saving, setSaving] = useState<boolean>(false);
  const [showInstructions, setShowInstructions] = useState<boolean>(true);

  useEffect(() => {
    const fetchSettings = async () => {
      try {
        const response = await axios.get(`${API_BASE}/api/user/telegram`);

        if (response.data) {
          const {
            telegram_user_id,
            telegram_enabled,
            telegram_ai_notifications,
            telegram_ai_chat,
            telegram_bot_token,
            bot_configured,
            bot_env_configured,
            bot_username,
          } = response.data;
          setTelegramUserId(telegram_user_id ? String(telegram_user_id) : "");
          setTelegramEnabled(Boolean(telegram_enabled));
          setTelegramAiNotif(Boolean(telegram_ai_notifications));
          setTelegramAiChat(Boolean(telegram_ai_chat));
          setBotToken(telegram_bot_token ? String(telegram_bot_token) : "");
          setBotConfigured(Boolean(bot_configured));
          setBotEnvConfigured(Boolean(bot_env_configured));
          setBotUsername(bot_username ? String(bot_username) : "");
        }
      } catch (err) {
        if (process.env.NODE_ENV === "development") console.error("Failed to load Telegram settings:", err);
        toast.addToast(t("telegram_load_error"), "error");
      } finally {
        setLoading(false);
      }
    };

    fetchSettings();
  }, [t, toast]);

  const handleSave = async () => {
    const trimmedId = telegramUserId.trim();
    const userIdParsed = trimmedId ? parseInt(trimmedId, 10) : 0;

    if (trimmedId && (isNaN(userIdParsed) || !/^\d+$/.test(trimmedId))) {
      toast.addToast(t("telegram_validation_invalid_id"), "error");
      return;
    }

    setSaving(true);
    try {
      await axios.post(`${API_BASE}/api/user/telegram`, {
        telegram_user_id: userIdParsed,
        telegram_enabled: telegramEnabled,
        telegram_ai_notifications: telegramAiNotif,
        telegram_ai_chat: telegramAiChat,
        telegram_bot_token: botToken,
      });
      toast.addToast(t("telegram_saved_success"), "success");
    } catch (err) {
      if (process.env.NODE_ENV === "development") console.error("Failed to save Telegram settings:", err);
      toast.addToast(t("telegram_save_error"), "error");
    } finally {
      setSaving(false);
    }
  };

  if (loading) {
    return (
      <Card className="border border-border/40 bg-background/50 backdrop-blur-md">
        <CardContent className="flex flex-col items-center justify-center py-16 space-y-4">
          <Loader2 className="w-10 h-10 animate-spin text-primary" />
          <p className="text-sm text-muted-foreground">{t("loading")}</p>
        </CardContent>
      </Card>
    );
  }

  const isConfigured =
    telegramUserId.trim() !== "" && /^\d+$/.test(telegramUserId);

  return (
    <div className="space-y-6">
      {envOnly && (
        <div className="bg-primary/10 text-primary px-4 py-3 rounded-lg flex items-center gap-2 mb-4 border border-primary/20">
          <span className="text-xl">🔒</span>
          <div>
            <p className="font-medium text-sm">{t("managed_by_admin")}</p>
            <p className="text-xs opacity-80">{t("managed_by_admin_desc")}</p>
          </div>
        </div>
      )}

      {/* Bot Info / Configuration Card */}
      {botEnvConfigured && botUsername ? (
        /* Bot is configured globally by admin — show bot name as link */
        <Card className="border border-emerald-500/20 bg-emerald-500/5 backdrop-blur-md shadow-md">
          <CardContent className="p-5 flex items-start gap-4">
            <div className="p-2 bg-emerald-500/10 rounded-xl text-emerald-500 shrink-0">
              <CheckCircle2 className="w-6 h-6" />
            </div>
            <div className="space-y-2 flex-1 min-w-0">
              <h4 className="font-bold text-sm text-emerald-200">
                {t("telegram_bot_configured_title")}
              </h4>
              <p className="text-xs text-muted-foreground leading-relaxed">
                {t("telegram_bot_configured_desc")}
              </p>
              <a
                href={`https://telegram.dog/${botUsername}`}
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-primary hover:bg-primary/90 text-primary-foreground font-semibold text-xs shadow-md transition-all hover:scale-[1.03] mt-1"
              >
                <Send className="w-3.5 h-3.5 fill-current" />
                <span>{t("telegram_open_bot", { username: botUsername })}</span>
                <ExternalLink className="w-3 h-3" />
              </a>
            </div>
          </CardContent>
        </Card>
      ) : !botConfigured && !botEnvConfigured ? (
        /* Bot NOT configured at all — show setup instructions or token input */
        <Card className="border border-amber-500/20 bg-amber-500/5 backdrop-blur-md shadow-md">
          <CardContent className="p-5 flex items-start gap-4">
            <div className="p-2 bg-amber-500/10 rounded-xl text-amber-500 shrink-0">
              <Info className="w-6 h-6 animate-pulse" />
            </div>
            <div className="space-y-2 flex-1 min-w-0">
              <h4 className="font-bold text-sm text-amber-200">
                {t("telegram_bot_not_configured_title")}
              </h4>
              <p className="text-xs text-muted-foreground leading-relaxed">
                {t("telegram_bot_not_configured_desc")}
              </p>
              <div className="pt-2 space-y-3">
                <p className="text-xs font-semibold text-foreground">
                  {t("telegram_bot_creation_title")}
                </p>
                <div className="grid sm:grid-cols-3 gap-3">
                  <div className="p-3 rounded-lg bg-background/40 border border-border/20 text-xs">
                    <span className="font-bold text-amber-400">
                      {t("telegram_bot_step1_title")}
                    </span>
                    <p className="text-[11px] text-muted-foreground mt-1">
                      {t.rich("telegram_bot_step1_desc", {
                        link: (chunks) => (
                          <a
                            href="https://telegram.dog/BotFather"
                            target="_blank"
                            rel="noopener noreferrer"
                            className="text-primary hover:underline font-medium inline-flex items-center gap-0.5"
                          >
                            {chunks} <ExternalLink className="w-2.5 h-2.5" />
                          </a>
                        ),
                      })}
                    </p>
                  </div>
                  <div className="p-3 rounded-lg bg-background/40 border border-border/20 text-xs">
                    <span className="font-bold text-amber-400">
                      {t("telegram_bot_step2_title")}
                    </span>
                    <p className="text-[11px] text-muted-foreground mt-1">
                      {t.rich("telegram_bot_step2_desc", {
                        code: (chunks) => (
                          <code className="bg-muted px-1.5 py-0.5 rounded font-mono font-bold text-[10px]">
                            {chunks}
                          </code>
                        ),
                      })}
                    </p>
                  </div>
                  <div className="p-3 rounded-lg bg-background/40 border border-border/20 text-xs">
                    <span className="font-bold text-amber-400">
                      {t("telegram_bot_step3_title")}
                    </span>
                    <p className="text-[11px] text-muted-foreground mt-1">
                      {t("telegram_bot_step3_desc")}
                    </p>
                  </div>
                </div>
              </div>
              {/* Bot Token Input */}
              <div className="space-y-2 pt-3 max-w-md">
                <label className="text-sm font-semibold flex items-center gap-1.5">
                  <Key className="w-4 h-4 text-amber-400" />
                  <span>{t("telegram_bot_token_label")}</span>
                </label>
                <Input
                  disabled={envOnly}
                  type="password"
                  placeholder={t("telegram_bot_token_placeholder")}
                  value={botToken}
                  onChange={(e) => setBotToken(e.target.value)}
                  className="w-full font-mono text-xs"
                />
                <p className="text-[11px] text-muted-foreground">
                  {t("telegram_bot_token_desc")}
                </p>
              </div>
            </div>
          </CardContent>
        </Card>
      ) : null}

      {/* Premium Header Card */}
      <Card className="relative overflow-hidden border border-primary/20 bg-gradient-to-br from-background via-background to-primary/5 shadow-lg">
        <div className="absolute top-0 right-0 p-8 opacity-10 pointer-events-none">
          <Send className="w-36 h-36 text-primary rotate-12" />
        </div>
        <CardHeader className="pb-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <div className="p-2.5 bg-primary/10 rounded-xl text-primary ring-4 ring-primary/5">
                <Send className="w-6 h-6" />
              </div>
              <div>
                <CardTitle className="text-xl font-bold tracking-tight">
                  {t("tab_telegram")}
                </CardTitle>
                <p className="text-sm text-muted-foreground mt-1">
                  {t("telegram_header_desc")}
                </p>
              </div>
            </div>
            <div>
              <span
                className={`inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-medium ring-1 transition-all duration-300 ${
                  isConfigured && telegramEnabled && botConfigured
                    ? "bg-emerald-500/10 text-emerald-400 ring-emerald-500/30"
                    : "bg-amber-500/10 text-amber-400 ring-amber-500/30"
                }`}
              >
                <span
                  className={`w-2 h-2 rounded-full ${
                    isConfigured && telegramEnabled && botConfigured
                      ? "bg-emerald-500 animate-pulse"
                      : "bg-amber-500"
                  }`}
                />
                {isConfigured && telegramEnabled && botConfigured
                  ? t("telegram_status_connected")
                  : !botConfigured
                    ? t("telegram_status_bot_not_configured")
                    : t("telegram_status_pending")}
              </span>
            </div>
          </div>
        </CardHeader>
      </Card>

      {/* Interactive Setup Guide */}
      <Card className="border border-border/40 bg-background/50 backdrop-blur-md transition-all duration-300">
        <button
          onClick={() => setShowInstructions(!showInstructions)}
          className="flex items-center justify-between w-full p-5 text-left font-medium text-sm border-none hover:bg-muted/30 transition-colors rounded-t-xl"
        >
          <div className="flex items-center gap-2">
            <HelpCircle className="w-4.5 h-4.5 text-primary" />
            <span>{t("telegram_bot_instructions")}</span>
          </div>
          {showInstructions ? (
            <ChevronUp className="w-4 h-4 text-muted-foreground" />
          ) : (
            <ChevronDown className="w-4 h-4 text-muted-foreground" />
          )}
        </button>

        {showInstructions && (
          <CardContent className="p-6 pt-0 border-t border-border/40">
            <div className="grid md:grid-cols-3 gap-6 pt-5">
              {/* Step 1 */}
              <div className="flex items-start gap-4 p-5 rounded-xl bg-muted/20 border border-border/30 hover:border-border/60 transition-all duration-200 shadow-sm">
                <div className="flex-shrink-0 flex items-center justify-center w-8 h-8 rounded-full bg-primary text-primary-foreground font-bold text-sm shadow-md ring-4 ring-primary/15">
                  1
                </div>
                <div className="space-y-2 flex-1 min-w-0">
                  <h4 className="font-semibold text-sm text-foreground">
                    {t("telegram_guide_step1_title")}
                  </h4>
                  <p className="text-xs text-muted-foreground leading-relaxed">
                    {t("telegram_step1")}
                  </p>
                  <div className="pt-2 flex flex-col gap-1.5">
                    <a
                      href="https://telegram.dog/userinfobot"
                      target="_blank"
                      rel="noopener noreferrer"
                      className="inline-flex items-center gap-1 text-xs text-primary font-medium hover:underline"
                    >
                      {t("telegram_open_userinfobot")}{" "}
                      <ExternalLink className="w-3 h-3" />
                    </a>
                    <a
                      href="https://telegram.dog/KeepSilenceBot"
                      target="_blank"
                      rel="noopener noreferrer"
                      className="inline-flex items-center gap-1 text-xs text-primary font-medium hover:underline"
                    >
                      {t("telegram_open_keepsilencebot")}{" "}
                      <ExternalLink className="w-3 h-3" />
                    </a>
                  </div>
                </div>
              </div>

              {/* Step 2 */}
              <div className="flex items-start gap-4 p-5 rounded-xl bg-muted/20 border border-border/30 hover:border-border/60 transition-all duration-200 shadow-sm">
                <div className="flex-shrink-0 flex items-center justify-center w-8 h-8 rounded-full bg-primary text-primary-foreground font-bold text-sm shadow-md ring-4 ring-primary/15">
                  2
                </div>
                <div className="space-y-2 flex-1 min-w-0">
                  <h4 className="font-semibold text-sm text-foreground">
                    {t("telegram_guide_step2_title")}
                  </h4>
                  <p className="text-xs text-muted-foreground leading-relaxed">
                    {t("telegram_step2")}
                  </p>
                  <p className="text-[11px] text-muted-foreground/75 italic">
                    {t("telegram_guide_step2_hint")}
                  </p>
                </div>
              </div>

              {/* Step 3 */}
              <div className="flex items-start gap-4 p-5 rounded-xl bg-muted/20 border border-border/30 hover:border-border/60 transition-all duration-200 shadow-sm">
                <div className="flex-shrink-0 flex items-center justify-center w-8 h-8 rounded-full bg-primary text-primary-foreground font-bold text-sm shadow-md ring-4 ring-primary/15">
                  3
                </div>
                <div className="space-y-2 flex-1 min-w-0">
                  <h4 className="font-semibold text-sm text-foreground">
                    {t("telegram_guide_step3_title")}
                  </h4>
                  <p className="text-xs text-muted-foreground leading-relaxed">
                    {t("telegram_step3")}
                  </p>
                  <div className="pt-2">
                    {botConfigured && botUsername ? (
                      <a
                        href={`https://telegram.dog/${botUsername}`}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-primary hover:bg-primary/90 text-primary-foreground font-semibold text-xs shadow-md transition-all hover:scale-[1.03]"
                      >
                        <Send className="w-3.5 h-3.5 fill-current" />
                        <span>
                          {t("telegram_open_bot", { username: botUsername })}
                        </span>
                        <ExternalLink className="w-3 h-3" />
                      </a>
                    ) : (
                      <span className="inline-flex items-center gap-1 text-[11px] bg-primary/10 text-primary-foreground text-primary px-2 py-0.5 rounded font-mono font-bold">
                        /start
                      </span>
                    )}
                  </div>
                </div>
              </div>
            </div>
          </CardContent>
        )}
      </Card>

      {/* Main Settings Panel */}
      <Card className="border border-border/40 bg-background/50 backdrop-blur-md shadow-md">
        <CardHeader className="pb-4">
          <CardTitle className="text-base font-bold flex items-center gap-2">
            <Shield className="w-4.5 h-4.5 text-primary" />
            <span>{t("telegram_integration_params_title")}</span>
          </CardTitle>
          <p className="text-xs text-muted-foreground">
            {t("telegram_integration_params_desc")}
          </p>
        </CardHeader>

        <CardContent className="space-y-6">
          {/* User ID Field */}
          <div className="space-y-2 max-w-md">
            <label className="text-sm font-semibold flex items-center gap-1.5">
              <Key className="w-4 h-4 text-muted-foreground" />
              <span>{t("telegram_user_id")}</span>
            </label>
            <div className="relative max-w-xs">
              <Input
                disabled={envOnly}
                type="text"
                pattern="\d*"
                placeholder={t("telegram_user_id_placeholder")}
                value={telegramUserId}
                onChange={(e) => setTelegramUserId(e.target.value)}
                className="w-full pe-10 font-mono"
              />
              {isConfigured && (
                <div className="absolute right-3 top-1/2 -translate-y-1/2 text-emerald-500">
                  <CheckCircle2 className="w-5 h-5 fill-background" />
                </div>
              )}
            </div>
            <p className="text-xs text-muted-foreground">
              {t("telegram_user_id_desc")}
            </p>
          </div>

          <div className="border-t border-border/40 my-6" />

          {/* Toggle Switches */}
          <div className="space-y-5">
            {/* Toggle 1: Enable Notifications */}
            <div className="flex items-start justify-between p-4 rounded-xl border border-border/30 hover:bg-muted/10 transition-colors">
              <div className="space-y-1 pe-6">
                <div className="flex items-center gap-2">
                  <Send className="w-4 h-4 text-muted-foreground" />
                  <label
                    className="text-sm font-semibold cursor-pointer select-none"
                    htmlFor="tg-notif-toggle"
                  >
                    {t("telegram_enabled")}
                  </label>
                </div>
                <p className="text-xs text-muted-foreground max-w-xl leading-relaxed">
                  {t("telegram_enabled_desc")}
                </p>
              </div>
              <label className="relative inline-flex items-center cursor-pointer mt-1">
                <input
                  id="tg-notif-toggle"
                  type="checkbox"
                  className="sr-only peer"
                  checked={telegramEnabled}
                  onChange={(e) => setTelegramEnabled(e.target.checked)}
                />
                <div className="w-10 h-6 bg-muted-foreground/20 rounded-full peer peer-checked:bg-primary after:content-[''] after:absolute after:top-[2px] after:start-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:after:translate-x-full rtl:peer-checked:after:-translate-x-full dark:after:border-zinc-600" />
              </label>
            </div>

            {/* Toggle 2: AI Summaries */}
            <div
              className={`flex items-start justify-between p-4 rounded-xl border border-border/30 transition-all duration-300 ${
                telegramEnabled
                  ? "opacity-100 hover:bg-muted/10"
                  : "opacity-40 pointer-events-none bg-muted/5 select-none"
              }`}
            >
              <div className="space-y-1 pe-6">
                <div className="flex items-center gap-2">
                  <Sparkles className="w-4 h-4 text-amber-400" />
                  <label
                    className="text-sm font-semibold cursor-pointer select-none"
                    htmlFor="tg-ai-toggle"
                  >
                    {t("telegram_ai_notifications")}
                  </label>
                  <span className="inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-bold bg-amber-500/10 text-amber-400">
                    {t("telegram_badge_ai")}
                  </span>
                </div>
                <p className="text-xs text-muted-foreground max-w-xl leading-relaxed">
                  {t("telegram_ai_notifications_desc")}
                </p>
              </div>
              <label className="relative inline-flex items-center cursor-pointer mt-1">
                <input
                  id="tg-ai-toggle"
                  type="checkbox"
                  className="sr-only peer"
                  disabled={!telegramEnabled}
                  checked={telegramAiNotif}
                  onChange={(e) => setTelegramAiNotif(e.target.checked)}
                />
                <div className="w-10 h-6 bg-muted-foreground/20 rounded-full peer peer-checked:bg-primary after:content-[''] after:absolute after:top-[2px] after:start-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:after:translate-x-full rtl:peer-checked:after:-translate-x-full dark:after:border-zinc-600" />
              </label>
            </div>

            {/* Toggle 3: Chat with AI */}
            <div className="flex items-start justify-between p-4 rounded-xl border border-border/30 hover:bg-muted/10 transition-colors">
              <div className="space-y-1 pe-6">
                <div className="flex items-center gap-2">
                  <MessageSquare className="w-4 h-4 text-indigo-400" />
                  <label
                    className="text-sm font-semibold cursor-pointer select-none"
                    htmlFor="tg-chat-toggle"
                  >
                    {t("telegram_ai_chat")}
                  </label>
                  <span className="inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-bold bg-indigo-500/10 text-indigo-400">
                    {t("telegram_badge_interactive")}
                  </span>
                </div>
                <p className="text-xs text-muted-foreground max-w-xl leading-relaxed">
                  {t("telegram_ai_chat_desc")}
                </p>
              </div>
              <label className="relative inline-flex items-center cursor-pointer mt-1">
                <input
                  id="tg-chat-toggle"
                  type="checkbox"
                  className="sr-only peer"
                  checked={telegramAiChat}
                  onChange={(e) => setTelegramAiChat(e.target.checked)}
                />
                <div className="w-10 h-6 bg-muted-foreground/20 rounded-full peer peer-checked:bg-primary after:content-[''] after:absolute after:top-[2px] after:start-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:after:translate-x-full rtl:peer-checked:after:-translate-x-full dark:after:border-zinc-600" />
              </label>
            </div>
          </div>

          <div className="border-t border-border/40 my-6" />

          {/* Action Buttons */}
          <div className="flex justify-end gap-3">
            <Button
              disabled={saving || envOnly}
              onClick={handleSave}
              className="px-6 py-2 bg-primary hover:bg-primary/90 text-primary-foreground font-semibold rounded-lg shadow-md transition-all duration-200 flex items-center gap-2"
            >
              {saving ? (
                <>
                  <Loader2 className="w-4 h-4 animate-spin" />
                  <span>{t("telegram_saving")}</span>
                </>
              ) : (
                <>
                  <CheckCircle2 className="w-4 h-4" />
                  <span>{t("save")}</span>
                </>
              )}
            </Button>
          </div>
        </CardContent>
      </Card>

      {/* Information Tip Banner */}
      <div className="flex gap-3 p-4 rounded-xl border border-blue-500/10 bg-blue-500/5 text-blue-400/90 max-w-3xl">
        <Info className="w-5 h-5 shrink-0 mt-0.5 text-blue-400" />
        <div className="space-y-1">
          <h5 className="font-semibold text-xs text-blue-200">
            {t("telegram_security_note_title")}
          </h5>
          <p className="text-[11px] leading-relaxed">
            {t("telegram_security_note_desc")}
          </p>
        </div>
      </div>
    </div>
  );
}
