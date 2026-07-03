"use client";

import React from "react";
import { useTranslations } from "next-intl";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { editionLetter } from "@/hooks/useEmails";
import {
  getSavedUndoDelay,
  getSavedDateFormat,
  type DateFormat,
} from "@/lib/date-format";
import axios from "axios";
import "@/lib/api-client";

function getMarkReadDelay(): number {
  if (typeof window === "undefined") return 3000;
  const saved = localStorage.getItem("rms-mail_mark_read_delay_ms");
  return saved ? parseInt(saved, 10) : 3000;
}

export function getSavedMarkReadDelay(): number {
  return getMarkReadDelay();
}

function getBrowserNotifications(): boolean {
  if (typeof window === "undefined") return false;
  return localStorage.getItem("rms-mail_notifications") === "true";
}

export function getBrowserNotificationsEnabled(): boolean {
  return getBrowserNotifications();
}

function getUseEmailThreads(): boolean {
  if (typeof window === "undefined") return true;
  return localStorage.getItem("rms-mail_use_threads") !== "false";
}

export function getUseEmailThreadsEnabled(): boolean {
  return getUseEmailThreads();
}

function getShowAccountName(): boolean {
  if (typeof window === "undefined") return false;
  return localStorage.getItem("rms-mail_show_account_name") === "true";
}

export function getShowAccountNameEnabled(): boolean {
  return getShowAccountName();
}

import { API_BASE } from "@/hooks/useEmailTypes";

export function GeneralTab() {
  const t = useTranslations("settings");

  const [delay, setDelay] = React.useState(() => getMarkReadDelay());
  const [notifications, setNotifications] = React.useState(() =>
    getBrowserNotifications(),
  );
  const [useThreads, setUseThreads] = React.useState(() =>
    getUseEmailThreads(),
  );
  const [showAccountName, setShowAccountName] = React.useState(() =>
    getShowAccountName(),
  );
  const [currentPw, setCurrentPw] = React.useState("");
  const [newPw, setNewPw] = React.useState("");
  const [confirmPw, setConfirmPw] = React.useState("");
  const [pwStatus, setPwStatus] = React.useState<string | null>(null);

  // Undo send delay (0 = disabled, 5000–30000ms)
  const [undoDelay, setUndoDelay] = React.useState(() => getSavedUndoDelay());
  const handleUndoDelayChange = (value: number) => {
    setUndoDelay(value);
    localStorage.setItem("rms-mail_undo_delay_ms", String(value));
  };

  // Date format
  const [dateFormat, setDateFormat] = React.useState<DateFormat>(() =>
    getSavedDateFormat(),
  );
  const handleDateFormatChange = (value: DateFormat) => {
    setDateFormat(value);
    localStorage.setItem("rms-mail_date_format", value);
  };

  const handleChange = (value: number) => {
    setDelay(value);
    localStorage.setItem("rms-mail_mark_read_delay_ms", String(value));
  };

  const handleNotificationsChange = (checked: boolean) => {
    if (!checked) {
      setNotifications(false);
      localStorage.setItem("rms-mail_notifications", "false");
      return;
    }

    if (typeof Notification !== "undefined") {
      if (Notification.permission === "default") {
        // Must be called synchronously for Safari to recognize the user gesture
        try {
          const promise = Notification.requestPermission();
          if (promise && promise.then) {
            promise.then((permission) => {
              const granted = permission === "granted";
              setNotifications(granted);
              localStorage.setItem("rms-mail_notifications", String(granted));
            });
          }
        } catch {
          // Fallback for older browsers
          Notification.requestPermission((permission) => {
            const granted = permission === "granted";
            setNotifications(granted);
            localStorage.setItem("rms-mail_notifications", String(granted));
          });
        }
      } else if (Notification.permission === "denied") {
        setNotifications(false);
        localStorage.setItem("rms-mail_notifications", "false");
        alert(t("notifications_blocked"));
      } else {
        setNotifications(true);
        localStorage.setItem("rms-mail_notifications", "true");
      }
    }
  };

  const handleUseThreadsChange = (checked: boolean) => {
    setUseThreads(checked);
    localStorage.setItem("rms-mail_use_threads", String(checked));
  };

  const handleShowAccountNameChange = (checked: boolean) => {
    setShowAccountName(checked);
    localStorage.setItem("rms-mail_show_account_name", String(checked));
    window.dispatchEvent(new Event("rms-mail_settings_changed"));
  };

  const handleChangePassword = async () => {
    setPwStatus(null);
    if (!currentPw || !newPw || !confirmPw) {
      setPwStatus(t("password_all_fields"));
      return;
    }
    if (newPw !== confirmPw) {
      setPwStatus(t("password_mismatch"));
      return;
    }
    if (newPw.length < 6) {
      setPwStatus(t("password_too_short"));
      return;
    }
    try {
      await axios.post(`${API_BASE}/api/auth/change-password`, {
        current_password: currentPw,
        new_password: newPw,
      });
      setPwStatus("success");
      setCurrentPw("");
      setNewPw("");
      setConfirmPw("");
      setTimeout(() => setPwStatus(null), 3000);
    } catch (err: unknown) {
      let message = t("password_change_failed");
      if (err && typeof err === "object" && "response" in err) {
        const response = (err as { response?: { data?: string } }).response;
        if (response?.data) {
          message = response.data;
        }
      }
      setPwStatus(message);
    }
  };

  const isMono = editionLetter().startsWith("M");

  return (
    <>
      <Card className="border">
        <CardHeader className="pb-3">
          <CardTitle className="text-base">{t("tab_general")}</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-4">
            <div>
              <label className="text-sm font-medium">
                {t("mark_read_delay")}: {delay / 1000}s
              </label>
              <p className="text-xs text-muted-foreground mb-2">
                {t("mark_read_delay_desc")}
              </p>
              <div className="flex gap-2 items-center">
                <span className="text-xs text-muted-foreground">1s</span>
                <input
                  type="range"
                  min="1000"
                  max="30000"
                  step="1000"
                  value={delay}
                  onChange={(e) => handleChange(Number(e.target.value))}
                  className="flex-1"
                />
                <span className="text-xs text-muted-foreground">30s</span>
              </div>
            </div>
            <div className="flex items-center gap-3 pt-2 border-t border-border">
              <label className="text-sm font-medium cursor-pointer">
                🔔 {t("notifications")}
              </label>
              <label className="relative inline-flex items-center cursor-pointer">
                <input
                  type="checkbox"
                  className="sr-only peer"
                  checked={notifications}
                  onChange={(e) => handleNotificationsChange(e.target.checked)}
                />
                <div className="w-9 h-5 bg-muted-foreground/30 rounded-full peer peer-checked:bg-primary after:content-[''] after:absolute after:top-[2px] after:start-[2px] after:bg-white after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:after:translate-x-full rtl:peer-checked:after:-translate-x-full" />
              </label>
            </div>
            <div className="flex items-center gap-3 pt-2 border-t border-border">
              <label className="text-sm font-medium cursor-pointer">
                🧵 {t("use_threads")}
              </label>
              <label className="relative inline-flex items-center cursor-pointer">
                <input
                  type="checkbox"
                  className="sr-only peer"
                  checked={useThreads}
                  onChange={(e) => handleUseThreadsChange(e.target.checked)}
                />
                <div className="w-9 h-5 bg-muted-foreground/30 rounded-full peer peer-checked:bg-primary after:content-[''] after:absolute after:top-[2px] after:start-[2px] after:bg-white after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:after:translate-x-full rtl:peer-checked:after:-translate-x-full" />
              </label>
            </div>
            <div className="flex items-center gap-3 pt-2 border-t border-border">
              <label className="text-sm font-medium cursor-pointer">
                👤 {t("show_account_name")}
              </label>
              <label className="relative inline-flex items-center cursor-pointer">
                <input
                  type="checkbox"
                  className="sr-only peer"
                  checked={showAccountName}
                  onChange={(e) =>
                    handleShowAccountNameChange(e.target.checked)
                  }
                />
                <div className="w-9 h-5 bg-muted-foreground/30 rounded-full peer peer-checked:bg-primary after:content-[''] after:absolute after:top-[2px] after:start-[2px] after:bg-white after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:after:translate-x-full rtl:peer-checked:after:-translate-x-full" />
              </label>
            </div>
            <div className="flex items-center gap-3 pt-2 border-t border-border">
              <label className="text-sm font-medium">
                ⏱ {t("undo_send_delay")}:{" "}
                {undoDelay === 0 ? t("disabled") : `${undoDelay / 1000}s`}
              </label>
              <select
                value={undoDelay}
                onChange={(e) => handleUndoDelayChange(Number(e.target.value))}
                className="h-8 rounded-md border bg-background px-2 text-xs"
              >
                <option value={0}>{t("disabled")}</option>
                <option value={5000}>5s</option>
                <option value={10000}>10s</option>
                <option value={15000}>15s</option>
                <option value={20000}>20s</option>
                <option value={30000}>30s</option>
              </select>
            </div>
            <div className="flex items-center gap-3 pt-2 border-t border-border">
              <label className="text-sm font-medium">
                🗓 {t("date_format")}
              </label>
              <select
                value={dateFormat}
                onChange={(e) =>
                  handleDateFormatChange(e.target.value as DateFormat)
                }
                className="h-8 rounded-md border bg-background px-2 text-xs"
              >
                <option value="auto">
                  {t("date_format_auto")} (12:30 / 5 Apr)
                </option>
                <option value="eu">
                  {t("date_format_eu")} (05.04.2026 12:30)
                </option>
                <option value="us">
                  {t("date_format_us")} (04/05/2026 12:30 PM)
                </option>
                <option value="iso">
                  {t("date_format_iso")} (2026-04-05 12:30)
                </option>
                <option value="uk">
                  {t("date_format_uk")} (05/04/2026 12:30)
                </option>
              </select>
            </div>
          </div>
        </CardContent>
      </Card>

      {!isMono && (
        <Card className="border mt-4">
          <CardHeader className="pb-3">
            <CardTitle className="text-base">{t("change_password")}</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-3 max-w-sm">
              <Input
                type="password"
                placeholder={t("current_password")}
                value={currentPw}
                onChange={(e) => setCurrentPw(e.target.value)}
              />
              <Input
                type="password"
                placeholder={t("new_password")}
                value={newPw}
                onChange={(e) => setNewPw(e.target.value)}
              />
              <Input
                type="password"
                placeholder={t("confirm_password")}
                value={confirmPw}
                onChange={(e) => setConfirmPw(e.target.value)}
              />
              {pwStatus && (
                <p
                  className={`text-xs ${pwStatus === "success" ? "text-green-500" : "text-red-500"}`}
                >
                  {pwStatus === "success" ? t("password_changed") : pwStatus}
                </p>
              )}
              <Button size="sm" onClick={handleChangePassword}>
                {t("change_password")}
              </Button>
            </div>
          </CardContent>
        </Card>
      )}
    </>
  );
}
