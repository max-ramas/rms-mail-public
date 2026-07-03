/** Semi-transparent drag ghost so drop targets stay visible underneath. */
export function setSemiTransparentDragImage(
  e: React.DragEvent<HTMLElement>,
  opacity = 0.42,
) {
  const el = e.currentTarget;
  const rect = el.getBoundingClientRect();
  const ghost = el.cloneNode(true) as HTMLElement;
  ghost.style.position = "fixed";
  ghost.style.top = "-10000px";
  ghost.style.left = "-10000px";
  ghost.style.width = `${rect.width}px`;
  ghost.style.opacity = String(opacity);
  ghost.style.pointerEvents = "none";
  ghost.style.boxShadow = "0 6px 20px rgba(0,0,0,0.25)";
  document.body.appendChild(ghost);
  const offsetX = Math.max(0, e.clientX - rect.left);
  const offsetY = Math.max(0, e.clientY - rect.top);
  e.dataTransfer.setDragImage(ghost, offsetX, offsetY);
  requestAnimationFrame(() => {
    ghost.remove();
  });
}
