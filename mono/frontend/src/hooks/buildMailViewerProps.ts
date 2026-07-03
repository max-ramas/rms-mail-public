import type React from "react";
import type { EmailViewerProps } from "@/components/email-viewer/types";

type BuilderExtras = {
  setMobileView: (v: "sidebar" | "list" | "viewer") => void;
  setTranslation: (v: string | null) => void;
  setIsReplying: (v: boolean) => void;
  setIsForwarding: (v: boolean) => void;
  setReplyTo: (v: EmailViewerProps["replyTo"]) => void;
  setDraftId: (v: string | null) => void;
  setComposeTo: (v: string[]) => void;
  setComposeCc: (v: string[]) => void;
  setComposeSubject: (v: string) => void;
  setComposeBody: (v: string) => void;
  setFromIdentity: (v: string) => void;
  setAttachments: React.Dispatch<
    React.SetStateAction<EmailViewerProps["composeAttachments"]>
  >;
  moveEmailMutation: {
    mutate: (args: { emailId: string; folderId: string }) => void;
  };
  pinEmailMutation: { mutate: (id: string) => void };
  flagEmailMutation: { mutate: (id: string) => void };
  muteEmailMutation: { mutate: (id: string) => void };
  clearDraftMutation: { mutateAsync: (id: string) => Promise<unknown> };
  assignEmail: {
    mutate: (args: { email_id: string; user_id: string }) => void;
  };
  unassignEmail: { mutate: (id: string) => void };
  createComment: {
    mutate: (args: {
      email_id: string;
      author_id: string;
      body: string;
      internal: boolean;
    }) => void;
  };
  deleteComment: { mutate: (id: string) => void };
  bulkAction: {
    mutate: (args: {
      action: string;
      ids: string[];
      filter?: {
        account_id: string;
        filter_folder_id?: string;
        unified?: boolean;
      };
    }) => void;
  };
  usersQuery: { data?: EmailViewerProps["usersData"] };
  displayedEmails: EmailViewerProps["selectedEmail"][];
  fileInputRef: React.RefObject<HTMLInputElement | null>;
  handleSnooze: () => void;
};

export type BuildMailViewerPropsInput = Omit<
  EmailViewerProps,
  | "onBackClick"
  | "onDismissTranslation"
  | "onMoveToFolder"
  | "onTogglePin"
  | "onToggleFlag"
  | "onToggleMute"
  | "onSnooze"
  | "onClearDraft"
  | "onCompose"
  | "onRemoveAttachment"
  | "onFileInputClick"
  | "onAssign"
  | "onUnassign"
  | "onAddComment"
  | "onDeleteComment"
  | "onToggleRead"
> &
  BuilderExtras;

/** Builds EmailViewer props from inbox page state and handlers. */
export function buildMailViewerProps(
  input: BuildMailViewerPropsInput,
): EmailViewerProps {
  const {
    setMobileView,
    setTranslation,
    setIsReplying,
    setIsForwarding,
    setReplyTo,
    setDraftId,
    setComposeTo,
    setComposeCc,
    setComposeSubject,
    setComposeBody,
    setFromIdentity,
    setAttachments,
    activeAccount,
    selectedEmail,
    moveEmailMutation,
    pinEmailMutation,
    flagEmailMutation,
    muteEmailMutation,
    clearDraftMutation,
    assignEmail,
    unassignEmail,
    createComment,
    deleteComment,
    bulkAction,
    usersQuery,
    displayedEmails,
    selectedEmailId,
    fileInputRef,
    getSignatureHtml,
    handleSnooze,
    ...rest
  } = input;

  return {
    ...rest,
    selectedEmailId,
    selectedEmail,
    activeAccount,
    getSignatureHtml,
    onBackClick: () => setMobileView("list"),
    onDismissTranslation: () => setTranslation(null),
    onMoveToFolder: (folderId) =>
      moveEmailMutation.mutate({
        emailId: selectedEmail?.id ?? "",
        folderId,
      }),
    onTogglePin: () => pinEmailMutation.mutate(selectedEmail?.id ?? ""),
    onToggleFlag: () => flagEmailMutation.mutate(selectedEmail?.id ?? ""),
    onToggleMute: () => muteEmailMutation.mutate(selectedEmail?.id ?? ""),
    onSnooze: () => handleSnooze(),
    onClearDraft: (emailId) => {
      if (emailId) void clearDraftMutation.mutateAsync(emailId);
    },
    onCompose: () => {
        setIsReplying(true);
        setIsForwarding(false);
        setReplyTo(null);
        setDraftId(null);
        setComposeTo([]);
        setComposeCc([]);
        setComposeSubject("");
        setComposeBody(getSignatureHtml());
        setFromIdentity(
          activeAccount !== "unified" ? `account:${activeAccount}` : "",
        );
      },
    onRemoveAttachment: (i) =>
      setAttachments((prev) => prev.filter((_, idx) => idx !== i)),
    onFileInputClick: () => fileInputRef.current?.click(),
    onAssign: (emailId, userId) =>
      assignEmail.mutate({ email_id: emailId, user_id: userId }),
    onUnassign: (emailId) => unassignEmail.mutate(emailId),
    onAddComment: (body, internal) => {
      const authorId =
        usersQuery.data?.[0]?.id ?? "00000000-0000-0000-0000-000000000000";
      createComment.mutate({
        email_id: selectedEmail?.id ?? "",
        author_id: authorId,
        body,
        internal,
      });
    },
    onDeleteComment: (id) => deleteComment.mutate(id),
    onToggleRead: () => {
      if (selectedEmailId) {
        const currentEmail = displayedEmails.find(
          (e) => e?.id === selectedEmailId,
        );
        bulkAction.mutate({
          action: currentEmail?.is_read ? "unread" : "read",
          ids: [selectedEmailId],
        });
      }
    },
    chatOpen: rest.aiEnabled ? rest.chatOpen : false,
    onToggleChat: rest.aiEnabled ? rest.onToggleChat : undefined,
    chatMessages: rest.aiEnabled ? rest.chatMessages : [],
    onChatSend: rest.aiEnabled ? rest.onChatSend : undefined,
    chatPending: rest.aiEnabled ? rest.chatPending : false,
    onSummarize: rest.aiEnabled ? rest.onSummarize : undefined,
    onCategorize: rest.aiEnabled ? rest.onCategorize : undefined,
    onTranslate: rest.aiEnabled ? rest.onTranslate : undefined,
  };
}
