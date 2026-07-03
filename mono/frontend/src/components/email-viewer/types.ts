import type {
  Email,
  EmailComment,
  Folder,
  Account,
  Identity,
  Label,
  User,
  AIMessage,
} from "@/hooks/useEmailTypes";

export interface EmailViewerProps {
  selectedEmailId: string | null;
  selectedEmail: Email | null;
  locale: string;
  summary: string | null;
  translation: string | null;
  onReply: () => void;
  onForward: () => void;
  onReplyAll: () => void;
  onReplyWithQuote?: (selectedText?: string) => void;
  onArchive: () => void;
  onDelete: () => void;
  onMoveToFolder: (folderId: string) => void;
  onToggleRead: () => void;
  onSummarize?: () => void;
  onCategorize?: () => void;
  onTranslate?: () => void;
  onDismissTranslation?: () => void;
  onTogglePin: () => void;
  onToggleFlag: () => void;
  onToggleMute: () => void;
  onSnooze: (minutes: number) => void;
  onClearDraft: (emailId: string) => void;
  onCompose: () => void;
  summarizePending: boolean;
  categorizePending: boolean;
  translatePending: boolean;
  isReplying: boolean;
  isForwarding: boolean;
  replyTo: Email | null;
  composeTo: string[];
  composeCc: string[];
  composeSubject: string;
  composeBody: string;
  forwardOriginalHtml?: string;
  forwardMeta?: {
    from: string;
    subject: string;
    date: string;
    to: string;
  } | null;
  fromIdentity: string;
  composeAttachments: Array<{
    id: string;
    filename: string;
    size: number;
  }>;
  accounts: Account[];
  activeAccount: string;
  identities: Identity[] | undefined;
  onChangeComposeTo: (to: string[]) => void;
  onChangeComposeCc: (cc: string[]) => void;
  onChangeComposeSubject: (subject: string) => void;
  onChangeComposeBody: (body: string) => void;
  onChangeFromIdentity: (id: string) => void;
  onRemoveAttachment: (index: number) => void;
  onCancelReply: () => void;
  onSendEmail: (options: {
    to: string[];
    cc: string[];
    subject: string;
    body: string;
    html: string;
    identity: string;
  }) => void;
  sendPending: boolean;
  onSaveDraft?: (
    syncRemote: boolean,
    data?: {
      to: string[];
      cc: string[];
      subject: string;
      body: string;
      identity: string;
    },
  ) => void;
  saveDraftPending?: boolean;
  onFileInputClick: () => void;
  getSignatureHtml: () => string;
  emailHtml: string | undefined;
  emailBody: string | undefined;
  emailSnippet: string;
  emailLoading: boolean;
  emailAttachments:
    | Array<{
        hash: string;
        filename: string;
        size: number;
        content_id?: string;
      }>
    | undefined;
  draftReply: string | undefined;
  emailTagsData: string[] | undefined;
  folders: Folder[];
  labelsData: Label[] | undefined;
  selectedLabelIds: Set<string>;
  onToggleLabel: (labelId: string) => void;
  onSetLabels: (emailId: string, accountId: string, labelIds: string[]) => void;
  usersData: User[] | undefined;
  onAssign: (emailId: string, userId: string) => void;
  onUnassign: (emailId: string) => void;
  onSetStatus?: (emailId: string, status: string) => void;
  comments: EmailComment[];
  onAddComment: (body: string, internal: boolean) => void;
  onDeleteComment: (id: string) => void;
  aiEnabled?: boolean;
  chatOpen?: boolean;
  onToggleChat?: () => void;
  chatMessages?: AIMessage[];
  onChatSend?: (input: string) => void;
  chatPending?: boolean;
  tags: string[];
  formatFileSize: (bytes: number) => string;
  threadEmails?: Email[];
  onRestoreFromTrash?: (emailId: string) => void;
  onReplyToEmail?: (email: Email) => void;
  onDeleteEmail?: (emailId: string) => void;
  onEmailAction?: (emailId: string, action: string) => void;
  onBackClick?: () => void;
}
