"use client";

import React from "react";
import { useTranslations } from "next-intl";

interface State {
  hasError: boolean;
  error: Error | null;
}

class ErrorBoundaryInner extends React.Component<
  {
    children: React.ReactNode;
    fallback?: React.ReactNode;
    t: ReturnType<typeof useTranslations<"common">>;
  },
  State
> {
  constructor(props: {
    children: React.ReactNode;
    fallback?: React.ReactNode;
    t: ReturnType<typeof useTranslations<"common">>;
  }) {
    super(props);
    this.state = { hasError: false, error: null };
  }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error };
  }

  render() {
    if (this.state.hasError) {
      return (
        this.props.fallback || (
          <div className="flex h-screen items-center justify-center bg-background text-foreground">
            <div className="text-center space-y-4">
              <h1 className="text-2xl font-bold">
                {this.props.t("error_title")}
              </h1>
              <p className="text-muted-foreground text-sm">
                {this.state.error?.message || this.props.t("unexpected_error")}
              </p>
              <button
                onClick={() => {
                  this.setState({ hasError: false, error: null });
                  window.location.reload();
                }}
                className="rounded-lg bg-primary text-primary-foreground px-4 py-2 text-sm font-medium hover:bg-primary/90"
              >
                {this.props.t("reload_page")}
              </button>
            </div>
          </div>
        )
      );
    }
    return this.props.children;
  }
}

export function ErrorBoundary({
  children,
  fallback,
}: {
  children: React.ReactNode;
  fallback?: React.ReactNode;
}) {
  const t = useTranslations();
  return (
    <ErrorBoundaryInner t={t} fallback={fallback}>
      {children}
    </ErrorBoundaryInner>
  );
}
