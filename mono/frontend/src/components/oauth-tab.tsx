"use client";

import React, { useState, useEffect, useCallback } from "react";
import axios from "axios";
import { useTranslations } from "next-intl";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Copy, Check, ShieldAlert } from "lucide-react";
import { API_BASE } from "@/hooks/useEmailTypes";

export function OAuthTab() {
  const t = useTranslations("settings");
  const [googleClientId, setGoogleClientId] = useState("");
  const [googleClientSecret, setGoogleClientSecret] = useState("");
  const [microsoftClientId, setMicrosoftClientId] = useState("");
  const [microsoftClientSecret, setMicrosoftClientSecret] = useState("");
  const [loading, setLoading] = useState(false);
  const [copied, setCopied] = useState(false);
  const [redirectUri] = useState(
    () =>
      (typeof window !== "undefined" ? window.location.origin : "") +
      "/api/oauth/callback",
  );

  const loadSettings = useCallback(async () => {
    try {
      setLoading(true);
      const { data } = await axios.get(`${API_BASE}/api/system/oauth`);
      setGoogleClientId(data.google_client_id || "");
      setGoogleClientSecret(data.google_client_secret || "");
      setMicrosoftClientId(data.microsoft_client_id || "");
      setMicrosoftClientSecret(data.microsoft_client_secret || "");
    } catch (e) {
      if (process.env.NODE_ENV === "development") console.error(e);
      alert(
        t("oauth_load_failed", {
          defaultMessage: "Failed to load OAuth settings",
        }),
      );
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    loadSettings();
  }, [loadSettings]);

  const handleSave = async () => {
    try {
      setLoading(true);
      await axios.post(`${API_BASE}/api/system/oauth`, {
        google_client_id: googleClientId,
        google_client_secret: googleClientSecret,
        microsoft_client_id: microsoftClientId,
        microsoft_client_secret: microsoftClientSecret,
      });
      alert(
        t("oauth_saved", {
          defaultMessage: "OAuth settings saved successfully",
        }),
      );
      loadSettings();
    } catch (e) {
      if (process.env.NODE_ENV === "development") console.error(e);
      alert(
        t("oauth_save_failed", {
          defaultMessage: "Failed to save OAuth settings",
        }),
      );
    } finally {
      setLoading(false);
    }
  };

  const copyRedirectUri = () => {
    navigator.clipboard.writeText(redirectUri);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 xl:gap-8 w-full">
      {/* Left Column: Forms */}
      <div className="flex flex-col gap-6">
        <div className="bg-yellow-500/10 border border-yellow-500/30 p-4 rounded-xl flex gap-3 text-sm shadow-sm">
          <ShieldAlert className="w-5 h-5 text-yellow-500 shrink-0 mt-0.5" />
          <div className="space-y-1">
            <p className="font-semibold text-yellow-600 dark:text-yellow-500">
              {t("oauth_byoa_title", {
                defaultMessage: "Bring Your Own App (BYOA)",
              })}
            </p>
            <p className="text-yellow-700/80 dark:text-yellow-500/80 leading-relaxed">
              {t("oauth_byoa_desc", {
                defaultMessage:
                  "Configure your own Google and Microsoft OAuth applications to isolate API quotas and enhance security. Client secrets are securely masked after saving.",
              })}
            </p>
          </div>
        </div>

        <Card className="border-border/50 shadow-sm">
          <CardHeader className="pb-3">
            <CardTitle className="text-lg">
              {t("oauth_redirect_uri", { defaultMessage: "Redirect URI" })}
            </CardTitle>
            <p className="text-sm text-muted-foreground">
              {t("oauth_redirect_desc", {
                defaultMessage:
                  "Copy this URL and paste it as the Authorized Redirect URI in your OAuth provider console.",
              })}
            </p>
          </CardHeader>
          <CardContent>
            <div className="flex items-center gap-2">
              <Input
                readOnly
                value={redirectUri}
                className="bg-muted/50 font-mono text-sm"
              />
              <Button
                variant="secondary"
                size="icon"
                className="shrink-0"
                onClick={copyRedirectUri}
                title="Copy to clipboard"
              >
                {copied ? (
                  <Check className="w-4 h-4 text-green-500" />
                ) : (
                  <Copy className="w-4 h-4 text-muted-foreground" />
                )}
              </Button>
            </div>
          </CardContent>
        </Card>

        <Card className="border-border/50 shadow-sm">
          <CardHeader className="pb-4">
            <CardTitle className="text-lg">Google OAuth</CardTitle>
          </CardHeader>
          <CardContent className="space-y-5">
            <div className="space-y-1.5">
              <label className="text-sm font-semibold">
                {t("oauth_client_id", { defaultMessage: "Client ID" })}
              </label>
              <Input
                value={googleClientId}
                onChange={(e) => setGoogleClientId(e.target.value)}
                placeholder={t("oauth_client_id_placeholder_google", {
                  defaultMessage:
                    "e.g. 123456789-abc.apps.googleusercontent.com",
                })}
                className="bg-background"
              />
            </div>
            <div className="space-y-1.5">
              <label className="text-sm font-semibold">
                {t("oauth_client_secret", { defaultMessage: "Client Secret" })}
              </label>
              <Input
                type="password"
                value={googleClientSecret}
                onChange={(e) => setGoogleClientSecret(e.target.value)}
                placeholder="********"
                className="bg-background"
              />
              <p className="text-xs text-muted-foreground pt-1">
                {t("oauth_client_secret_desc", {
                  defaultMessage:
                    "Leave blank or as ******** to keep the current secret unchanged.",
                })}
              </p>
            </div>
          </CardContent>
        </Card>

        <Card className="border-border/50 shadow-sm">
          <CardHeader className="pb-4">
            <CardTitle className="text-lg">Microsoft OAuth</CardTitle>
          </CardHeader>
          <CardContent className="space-y-5">
            <div className="space-y-1.5">
              <label className="text-sm font-semibold">
                {t("oauth_client_id", { defaultMessage: "Client ID" })}
              </label>
              <Input
                value={microsoftClientId}
                onChange={(e) => setMicrosoftClientId(e.target.value)}
                placeholder={t("oauth_client_id_placeholder_ms", {
                  defaultMessage: "e.g. 11111111-2222-3333-4444-555555555555",
                })}
                className="bg-background"
              />
            </div>
            <div className="space-y-1.5">
              <label className="text-sm font-semibold">
                {t("oauth_client_secret", { defaultMessage: "Client Secret" })}
              </label>
              <Input
                type="password"
                value={microsoftClientSecret}
                onChange={(e) => setMicrosoftClientSecret(e.target.value)}
                placeholder="********"
                className="bg-background"
              />
              <p className="text-xs text-muted-foreground pt-1">
                {t("oauth_client_secret_desc", {
                  defaultMessage:
                    "Leave blank or as ******** to keep the current secret unchanged.",
                })}
              </p>
            </div>
          </CardContent>
        </Card>

        <div className="flex justify-end pt-2 pb-6 lg:pb-0">
          <Button
            onClick={handleSave}
            disabled={loading}
            className="w-full sm:w-auto"
          >
            {loading
              ? t("saving", { defaultMessage: "Saving..." })
              : t("save", { defaultMessage: "Save Settings" })}
          </Button>
        </div>
      </div>

      {/* Right Column: Instructions */}
      <div className="flex flex-col">
        <Card className="border-border/50 shadow-sm bg-card/40 flex-1 overflow-hidden">
          <Tabs defaultValue="google" className="w-full flex flex-col h-full">
            <CardHeader className="pb-4 border-b bg-muted/20">
              <TabsList className="grid w-full grid-cols-2">
                <TabsTrigger value="google">Google</TabsTrigger>
                <TabsTrigger value="microsoft">Microsoft</TabsTrigger>
              </TabsList>
            </CardHeader>
            <CardContent className="pt-6">
              <div className="mb-6 p-4 bg-muted/40 rounded-xl border border-border/40 text-sm">
                <h3 className="font-semibold mb-2">
                  {t("oauth_google_inst_prep_title", {
                    defaultMessage: "Preparation",
                  })}
                </h3>
                <p className="text-muted-foreground mb-3">
                  {t("oauth_google_inst_prep_desc", {
                    defaultMessage:
                      "In your OAuth tab, copy the Redirect URI. Usually it looks like:",
                  })}
                </p>
                <div className="flex items-center gap-2">
                  <code className="bg-background px-2.5 py-1.5 rounded-md border text-xs flex-1 truncate select-all">
                    {redirectUri}
                  </code>
                  <Button
                    variant="secondary"
                    size="icon"
                    className="h-8 w-8 shrink-0"
                    onClick={copyRedirectUri}
                    title="Copy to clipboard"
                  >
                    {copied ? (
                      <Check className="w-3.5 h-3.5 text-green-500" />
                    ) : (
                      <Copy className="w-3.5 h-3.5 text-muted-foreground" />
                    )}
                  </Button>
                </div>
              </div>

              <TabsContent
                value="google"
                className="m-0 focus-visible:outline-none"
              >
                <div className="space-y-5 text-sm">
                  <div className="flex gap-3.5">
                    <div className="flex mt-0.5 h-6 w-6 shrink-0 items-center justify-center rounded-full bg-primary/10 text-primary text-xs font-bold">
                      1
                    </div>
                    <div>
                      <h4 className="font-semibold text-foreground mb-1">
                        {t("oauth_google_inst_1_title", {
                          defaultMessage: "1. Google Cloud Console",
                        })}
                      </h4>
                      <p className="text-muted-foreground leading-relaxed">
                        {t("oauth_google_inst_1_desc", {
                          defaultMessage: "Go to Google Cloud Console.",
                        })}
                      </p>
                    </div>
                  </div>

                  <div className="flex gap-3.5">
                    <div className="flex mt-0.5 h-6 w-6 shrink-0 items-center justify-center rounded-full bg-primary/10 text-primary text-xs font-bold">
                      2
                    </div>
                    <div>
                      <h4 className="font-semibold text-foreground mb-1">
                        {t("oauth_google_inst_2_title", {
                          defaultMessage: "2. Create Project",
                        })}
                      </h4>
                      <p className="text-muted-foreground leading-relaxed">
                        {t("oauth_google_inst_2_desc", {
                          defaultMessage:
                            "In the top left corner, click the projects dropdown and select New Project. Enter a name (e.g. RMS Mail) and click Create.",
                        })}
                      </p>
                    </div>
                  </div>

                  <div className="flex gap-3.5">
                    <div className="flex mt-0.5 h-6 w-6 shrink-0 items-center justify-center rounded-full bg-primary/10 text-primary text-xs font-bold">
                      3
                    </div>
                    <div>
                      <h4 className="font-semibold text-foreground mb-1">
                        {t("oauth_google_inst_3_title", {
                          defaultMessage: "3. Configure Consent Screen",
                        })}
                      </h4>
                      <p className="text-muted-foreground leading-relaxed">
                        {t("oauth_google_inst_3_desc", {
                          defaultMessage:
                            "Go to APIs & Services → OAuth consent screen. Select External and click Create. Fill in App name, User support email, and Developer contact information. Click Save and Continue. Under Scopes, add mail scopes (e.g. https://mail.google.com/, email, profile). Important: Return to OAuth consent screen and click Publish App, otherwise authorization expires in 7 days.",
                        })}
                      </p>
                    </div>
                  </div>

                  <div className="flex gap-3.5">
                    <div className="flex mt-0.5 h-6 w-6 shrink-0 items-center justify-center rounded-full bg-primary/10 text-primary text-xs font-bold">
                      4
                    </div>
                    <div>
                      <h4 className="font-semibold text-foreground mb-1">
                        {t("oauth_google_inst_4_title", {
                          defaultMessage: "4. Create Credentials",
                        })}
                      </h4>
                      <p className="text-muted-foreground leading-relaxed">
                        {t("oauth_google_inst_4_desc", {
                          defaultMessage:
                            "Go to Credentials. Click + Create Credentials and select OAuth client ID. Select Web application as type. Under Authorized redirect URIs, click + Add URI and paste your Redirect URI. Click Create.",
                        })}
                      </p>
                    </div>
                  </div>

                  <div className="flex gap-3.5">
                    <div className="flex mt-0.5 h-6 w-6 shrink-0 items-center justify-center rounded-full bg-primary/10 text-primary text-xs font-bold">
                      5
                    </div>
                    <div>
                      <h4 className="font-semibold text-foreground mb-1">
                        {t("oauth_google_inst_5_title", {
                          defaultMessage: "5. Copy Keys",
                        })}
                      </h4>
                      <p className="text-muted-foreground leading-relaxed">
                        {t("oauth_google_inst_5_desc", {
                          defaultMessage:
                            "A window will appear with your Client ID and Client Secret. Copy and paste them into the settings here.",
                        })}
                      </p>
                    </div>
                  </div>
                </div>
              </TabsContent>

              <TabsContent
                value="microsoft"
                className="m-0 focus-visible:outline-none"
              >
                <div className="space-y-5 text-sm">
                  <div className="flex gap-3.5">
                    <div className="flex mt-0.5 h-6 w-6 shrink-0 items-center justify-center rounded-full bg-primary/10 text-primary text-xs font-bold">
                      1
                    </div>
                    <div>
                      <h4 className="font-semibold text-foreground mb-1">
                        {t("oauth_ms_inst_1_title", {
                          defaultMessage: "1. Azure Portal",
                        })}
                      </h4>
                      <p className="text-muted-foreground leading-relaxed">
                        {t("oauth_ms_inst_1_desc", {
                          defaultMessage:
                            "Go to Azure Portal and log in with your Microsoft account.",
                        })}
                      </p>
                    </div>
                  </div>

                  <div className="flex gap-3.5">
                    <div className="flex mt-0.5 h-6 w-6 shrink-0 items-center justify-center rounded-full bg-primary/10 text-primary text-xs font-bold">
                      2
                    </div>
                    <div>
                      <h4 className="font-semibold text-foreground mb-1">
                        {t("oauth_ms_inst_2_title", {
                          defaultMessage: "2. Microsoft Entra ID",
                        })}
                      </h4>
                      <p className="text-muted-foreground leading-relaxed">
                        {t("oauth_ms_inst_2_desc", {
                          defaultMessage:
                            "Search for and select Microsoft Entra ID (formerly Azure Active Directory).",
                        })}
                      </p>
                    </div>
                  </div>

                  <div className="flex gap-3.5">
                    <div className="flex mt-0.5 h-6 w-6 shrink-0 items-center justify-center rounded-full bg-primary/10 text-primary text-xs font-bold">
                      3
                    </div>
                    <div>
                      <h4 className="font-semibold text-foreground mb-1">
                        {t("oauth_ms_inst_3_title", {
                          defaultMessage: "3. App Registration",
                        })}
                      </h4>
                      <p className="text-muted-foreground leading-relaxed">
                        {t("oauth_ms_inst_3_desc", {
                          defaultMessage:
                            "Go to App registrations and click + New registration. Enter a name. Under Supported account types, select the third option (Accounts in any organizational directory and personal Microsoft accounts). Under Redirect URI, select Web and paste your Redirect URI. Click Register.",
                        })}
                      </p>
                    </div>
                  </div>

                  <div className="flex gap-3.5">
                    <div className="flex mt-0.5 h-6 w-6 shrink-0 items-center justify-center rounded-full bg-primary/10 text-primary text-xs font-bold">
                      4
                    </div>
                    <div>
                      <h4 className="font-semibold text-foreground mb-1">
                        {t("oauth_ms_inst_4_title", {
                          defaultMessage: "4. Get Client ID",
                        })}
                      </h4>
                      <p className="text-muted-foreground leading-relaxed">
                        {t("oauth_ms_inst_4_desc", {
                          defaultMessage:
                            "After creation, copy the Application (client) ID value — this is your Client ID.",
                        })}
                      </p>
                    </div>
                  </div>

                  <div className="flex gap-3.5">
                    <div className="flex mt-0.5 h-6 w-6 shrink-0 items-center justify-center rounded-full bg-primary/10 text-primary text-xs font-bold">
                      5
                    </div>
                    <div>
                      <h4 className="font-semibold text-foreground mb-1">
                        {t("oauth_ms_inst_5_title", {
                          defaultMessage: "5. Create Client Secret",
                        })}
                      </h4>
                      <p className="text-muted-foreground leading-relaxed">
                        {t("oauth_ms_inst_5_desc", {
                          defaultMessage:
                            "Go to Certificates & secrets. Under Client secrets, click + New client secret. Add a description and expiration. Important: Copy the Value immediately (not the Secret ID!). It is shown only once.",
                        })}
                      </p>
                    </div>
                  </div>

                  <div className="flex gap-3.5">
                    <div className="flex mt-0.5 h-6 w-6 shrink-0 items-center justify-center rounded-full bg-primary/10 text-primary text-xs font-bold">
                      6
                    </div>
                    <div>
                      <h4 className="font-semibold text-foreground mb-1">
                        {t("oauth_ms_inst_6_title", {
                          defaultMessage: "6. Configure API Permissions",
                        })}
                      </h4>
                      <p className="text-muted-foreground leading-relaxed">
                        {t("oauth_ms_inst_6_desc", {
                          defaultMessage:
                            "Go to API permissions. Click + Add a permission → Microsoft Graph → Delegated permissions. Add mail permissions (e.g. IMAP.AccessAsUser.All, SMTP.Send, offline_access, User.Read). Click Add permissions.",
                        })}
                      </p>
                    </div>
                  </div>
                </div>
              </TabsContent>
            </CardContent>
          </Tabs>
        </Card>
      </div>
    </div>
  );
}
