let bulkSelectionActive = false;

export function setBulkSelectionActive(active: boolean) {
  bulkSelectionActive = active;
}

export function isBulkSelectionActive(): boolean {
  return bulkSelectionActive;
}
