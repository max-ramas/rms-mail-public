import type { Folder } from "@/hooks/useEmailTypes";

const TRASH_FOLDER_NAMES = new Set([
  "Trash",
  "TRASH",
  "trash",
  "Корзина",
  "Deleted Items",
  "Papierkorb",
  "ゴミ箱",
]);

export function isTrashFolder(
  folder: Pick<Folder, "name" | "special_use">,
): boolean {
  if (folder.special_use === "trash") return true;
  return TRASH_FOLDER_NAMES.has(folder.name);
}
