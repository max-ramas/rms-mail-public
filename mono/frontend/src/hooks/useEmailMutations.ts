// Barrel file — re-exports for backward compatibility.
// Each hook lives in its own file in this directory.

export { useMarkEmailRead } from "./useMarkRead";
export { useFlagEmail } from "./useFlagEmail";
export { usePinEmail } from "./usePinEmail";
export { useMuteEmail } from "./useMuteEmail";
export { useSnoozeEmail } from "./useSnoozeEmail";
export { useMoveEmailToFolder } from "./useMoveEmail";
export { useDeleteEmail } from "./useDeleteEmail";
export { useSendEmail } from "./useSendEmail";
export { useSaveDraft } from "./useSaveDraft";
export { useBulkEmailAction } from "./useBulkAction";
export {
  useSummarizeEmail,
  useCategorizeEmail,
  useClearDraftReply,
  useSetEmailLabels,
} from "./useEmailAI";
export {
  useAssignEmail,
  useUnassignEmail,
  useCreateComment,
  useDeleteComment,
} from "./useAssignEmail";
export {
  useCreateFolder,
  useRenameFolder,
  useDeleteFolder,
} from "./useFolderMutations";

export type { AICustomParams } from "./types";
