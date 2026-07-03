"use client";

import React, { useState, useEffect } from "react";
import { useAIModels } from "@/hooks/useAIApi";
import { useToast } from "@/hooks/useToast";
import { useTranslations } from "next-intl";
import { RefreshCw } from "lucide-react";

interface ModelSelectorProps {
  provider: string;
  apiKey?: string;
  value: string;
  onChange: (val: string) => void;
  disabled?: boolean;
}

export function ModelSelector({
  provider,
  apiKey,
  value,
  onChange,
  disabled,
}: ModelSelectorProps) {
  const t = useTranslations("settings");
  const toast = useToast();
  const [forceRefresh, setForceRefresh] = useState(true);

  const { data, isLoading, isFetching, isError, error } = useAIModels(
    provider,
    apiKey,
    forceRefresh,
  );

  useEffect(() => {
    if (isError && error) {
      // Assuming the API returns a useful error message in error.response.data.error or similar
      const errMsg =
        // @ts-expect-error Types don't know about axios response.data
        error.response?.data?.error ||
        error.message ||
        t("fetch_models_error_fallback", { provider });
      toast.addToast(
        t("fetch_models_error", { provider, error: errMsg }),
        "error",
      );
    }
  }, [isError, error, provider, toast, t]);

  const handleRefresh = () => {
    setForceRefresh(true);
  };

  const models = React.useMemo(() => data?.models || [], [data?.models]);

  // Auto-correct stale model + reset forceRefresh after load
  useEffect(() => {
    if (provider === "ollama") return;
    if (models.length === 0) return;
    // Auto-correct if current value is not in the list
    if (value && !models.includes(value)) {
      onChange(models[0]);
    }
    // Reset forceRefresh after successful fetch
    if (forceRefresh) {
      React.startTransition(() => setForceRefresh(false));
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [models, value, provider]);

  if (provider === "ollama") {
    // Ollama allows any model name (can be typed)
    return (
      <div className="flex-1 flex gap-2">
        <input
          value={value}
          onChange={(e) => onChange(e.target.value)}
          disabled={disabled}
          className="flex-1 h-9 rounded-md border bg-background px-2 py-1 text-sm text-foreground shadow-sm disabled:opacity-50"
          placeholder={t("model_name_placeholder")}
        />
      </div>
    );
  }

  return (
    <div className="flex-1 flex gap-2 items-center">
      <select
        value={value}
        onChange={(e) => onChange(e.target.value)}
        disabled={disabled || isLoading}
        className="flex-1 h-9 rounded-md border bg-background px-2 py-1 text-sm text-foreground shadow-sm disabled:opacity-50"
      >
        {models.length === 0 && !isLoading && (
          <option value={value || t("default_model")}>
            {value || t("default_model")}
          </option>
        )}
        {models.map((m) => (
          <option key={m} value={m}>
            {m}
          </option>
        ))}
      </select>
      <button
        type="button"
        onClick={handleRefresh}
        disabled={disabled || isLoading || isFetching}
        className="h-9 w-9 flex items-center justify-center border rounded-md hover:bg-muted text-muted-foreground transition-colors disabled:opacity-50"
        title={t("refresh_models")}
      >
        <RefreshCw
          className={`w-4 h-4 ${isLoading || isFetching ? "animate-spin" : ""}`}
        />
      </button>
    </div>
  );
}
