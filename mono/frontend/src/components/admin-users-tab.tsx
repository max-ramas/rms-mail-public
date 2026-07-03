"use client";

import { useTranslations } from "next-intl";
import { Card, CardHeader, CardTitle, CardContent } from "./ui/card";
import { useAdminUsers } from "@/hooks/useAdminQueries";

export function AdminUsersTab() {
  const t = useTranslations("settings");
  const {
    data: users,
    isLoading: isLoadingUsers,
    error: usersError,
  } = useAdminUsers();

  return (
    <Card className="border">
      <CardHeader>
        <CardTitle>{t("admin_users", { defaultMessage: "Users" })}</CardTitle>
        <p className="text-sm text-muted-foreground">
          {t("admin_users_desc", {
            defaultMessage: "Manage registered user accounts.",
          })}
        </p>
      </CardHeader>
      <CardContent>
        {isLoadingUsers ? (
          <div className="text-sm text-muted-foreground">
            {t("loading_users", { defaultMessage: "Loading users..." })}
          </div>
        ) : usersError ? (
          <div className="text-sm text-red-500">
            {t("error_loading_users", {
              defaultMessage: "Failed to load users.",
            })}
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm text-left">
              <thead className="text-xs text-muted-foreground uppercase bg-muted/50">
                <tr>
                  <th className="px-4 py-2 font-medium">Email</th>
                  <th className="px-4 py-2 font-medium">Name</th>
                  <th className="px-4 py-2 font-medium">Role</th>
                  <th className="px-4 py-2 font-medium">Last Seen At</th>
                </tr>
              </thead>
              <tbody>
                {users?.map(
                  (u: {
                    id: string;
                    email: string;
                    name: string;
                    role: string;
                    last_seen_at: string;
                  }) => (
                    <tr key={u.id} className="border-b last:border-0">
                      <td className="px-4 py-2">{u.email}</td>
                      <td className="px-4 py-2">{u.name || "-"}</td>
                      <td className="px-4 py-2">
                        <span
                          className={`px-2 py-0.5 rounded-full text-xs font-semibold ${u.role === "admin" ? "bg-amber-500/10 text-amber-600" : "bg-muted text-muted-foreground"}`}
                        >
                          {u.role || "user"}
                        </span>
                      </td>
                      <td className="px-4 py-2 text-muted-foreground">
                        {u.last_seen_at
                          ? new Date(u.last_seen_at).toLocaleString()
                          : "Never"}
                      </td>
                    </tr>
                  ),
                )}
                {users?.length === 0 && (
                  <tr>
                    <td
                      colSpan={4}
                      className="px-4 py-4 text-center text-muted-foreground"
                    >
                      No users found.
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
