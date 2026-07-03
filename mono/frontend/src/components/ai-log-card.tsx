"use client";

import { Activity, RefreshCw, Trash2 } from "lucide-react";
import { useTranslations } from "next-intl";
import { useAIStats, useAILog, useResetAIStats } from "@/hooks/useAIApi";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { useToast } from "@/hooks/useToast";

export function AILogCard() {
  const t = useTranslations("settings");
  const {
    data: stats,
    isLoading: statsLoading,
    isError: statsError,
    error: statsErrorObj,
  } = useAIStats();
  const {
    data: log,
    isLoading: logLoading,
    isError: logError,
    error: logErrorObj,
  } = useAILog();
  const resetStats = useResetAIStats();
  const toast = useToast();

  const handleReset = async () => {
    if (confirm(t("confirm_reset_ai_stats"))) {
      try {
        await resetStats.mutateAsync();
        toast.addToast(t("reset_success"), "success");
      } catch {
        toast.addToast(t("reset_error"), "error");
      }
    }
  };

  return (
    <Card className="border">
      <CardHeader>
        <div className="flex items-center justify-between w-full">
          <CardTitle className="text-base flex items-center gap-2">
            <Activity className="w-5 h-5" /> {t("ai_usage")}
          </CardTitle>
          <Button
            variant="ghost"
            size="sm"
            onClick={handleReset}
            disabled={resetStats.isPending}
            className="h-8 text-muted-foreground hover:text-red-500"
          >
            <Trash2 className="w-4 h-4 me-1" /> {t("reset")}
          </Button>
        </div>
      </CardHeader>
      <CardContent>
        {statsLoading || logLoading ? (
          <div className="text-muted-foreground text-sm flex items-center gap-2">
            <RefreshCw className="w-3 h-3 animate-spin" /> {t("loading")}
          </div>
        ) : statsError || logError ? (
          <div className="text-red-500 text-sm">
            {(statsErrorObj || logErrorObj)?.message ||
              String(statsErrorObj || logErrorObj || t("loading"))}
          </div>
        ) : !stats ? (
          <div className="text-muted-foreground text-sm">
            {t("no_ai_usage")}
          </div>
        ) : (
          <div className="space-y-4 text-sm">
            <div className="grid grid-cols-3 gap-2 sm:gap-3">
              <div className="bg-muted rounded p-2 sm:p-3 text-center overflow-hidden">
                <div
                  className="text-lg sm:text-xl lg:text-2xl font-bold text-primary truncate"
                  title={stats.total_actions.toString()}
                >
                  {stats.total_actions}
                </div>
                <div className="text-[10px] sm:text-xs text-muted-foreground truncate">
                  {t("total_calls")}
                </div>
              </div>
              <div className="bg-muted rounded p-2 sm:p-3 text-center overflow-hidden">
                <div
                  className="text-lg sm:text-xl lg:text-2xl font-bold text-green-400 truncate"
                  title={`${(stats.total_tokens / 1000).toFixed(1)}K`}
                >
                  {(stats.total_tokens / 1000).toFixed(1)}K
                </div>
                <div className="text-[10px] sm:text-xs text-muted-foreground truncate">
                  {t("tokens_used")}
                </div>
              </div>
              <div className="bg-muted rounded p-2 sm:p-3 text-center overflow-hidden">
                <div
                  className="text-lg sm:text-xl lg:text-2xl font-bold text-yellow-400 truncate"
                  title={`$${stats.total_cost_usd.toFixed(4)}`}
                >
                  ${stats.total_cost_usd.toFixed(4)}
                </div>
                <div className="text-[10px] sm:text-xs text-muted-foreground truncate">
                  {t("est_cost")}
                </div>
              </div>
            </div>

            {Object.keys(stats.by_action).length > 0 && (
              <div>
                <div className="text-xs text-muted-foreground mb-1">
                  {t("by_action")}
                </div>
                <div className="flex gap-2 flex-wrap">
                  {Object.entries(stats.by_action).map(([action, count]) => (
                    <span
                      key={action}
                      className="bg-muted rounded px-2 py-0.5 text-xs"
                    >
                      {action}: {count}
                    </span>
                  ))}
                </div>
              </div>
            )}

            {log && log.length > 0 && (
              <div>
                <div className="text-xs text-muted-foreground mb-1">
                  {t("recent_activity")}
                </div>
                <div className="max-h-32 overflow-y-auto space-y-1">
                  {log.slice(0, 20).map((entry) => (
                    <div
                      key={entry.id}
                      className="text-[11px] text-muted-foreground flex gap-2"
                    >
                      <span
                        className={`w-1.5 h-1.5 rounded-full mt-1 shrink-0 ${entry.status === "success" ? "bg-green-500" : "bg-red-500"}`}
                      />
                      <span className="w-16 shrink-0">{entry.action}</span>
                      <span className="w-20 shrink-0">{entry.provider}</span>
                      <span>{entry.total_tokens}t</span>
                      <span className="text-muted-foreground/50">
                        {new Date(entry.created_at).toLocaleTimeString()}
                      </span>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
