import { useState, useCallback, useEffect, useRef, CSSProperties } from 'react';

interface UseDraggableOptions {
  /** Whether dragging is enabled (false on mobile, when maximized, etc.) */
  enabled: boolean;
}

interface DragOffset {
  x: number;
  y: number;
}

interface UseDraggableReturn {
  /** Current drag offset from initial position, null when not dragged yet */
  offset: DragOffset | null;
  /** Whether a drag is actively in progress */
  isDragging: boolean;
  /** Attach to the drag handle element (e.g., panel header) */
  handleProps: {
    onMouseDown: (e: React.MouseEvent) => void;
    // Must be CSSProperties (not a loose record) — consumers pass this to
    // MUI's Box.style which demands the narrow literal unions for properties
    // like `userSelect`.
    style: CSSProperties;
  };
  /** Reset position back to default (clears drag offset) */
  resetPosition: () => void;
}

/**
 * Hook that provides drag-to-reposition behavior.
 *
 * Returns an offset delta (x, y) from the element's original position.
 * The consumer applies this offset via CSS transform.
 *
 * Mouse listeners are attached to `document` so dragging continues even
 * when the cursor moves faster than the element can follow.
 */
export function useDraggable({ enabled }: UseDraggableOptions): UseDraggableReturn {
  const [offset, setOffset] = useState<DragOffset | null>(null);
  const [isDragging, setIsDragging] = useState(false);

  // Refs to avoid stale closures in document listeners
  const dragStartRef = useRef<{
    mouseX: number;
    mouseY: number;
    offsetX: number;
    offsetY: number;
  } | null>(null);
  const offsetRef = useRef<DragOffset | null>(null);

  // Keep offsetRef in sync
  useEffect(() => {
    offsetRef.current = offset;
  }, [offset]);

  const handleMouseDown = useCallback(
    (e: React.MouseEvent) => {
      if (!enabled) return;
      e.preventDefault();
      const currentOffset = offsetRef.current ?? { x: 0, y: 0 };
      dragStartRef.current = {
        mouseX: e.clientX,
        mouseY: e.clientY,
        offsetX: currentOffset.x,
        offsetY: currentOffset.y,
      };
      setIsDragging(true);
    },
    [enabled],
  );

  useEffect(() => {
    if (!isDragging) return;

    const handleMouseMove = (e: MouseEvent) => {
      const start = dragStartRef.current;
      if (!start) return;
      const dx = e.clientX - start.mouseX;
      const dy = e.clientY - start.mouseY;
      setOffset({ x: start.offsetX + dx, y: start.offsetY + dy });
    };

    const handleMouseUp = () => {
      setIsDragging(false);
    };

    document.addEventListener('mousemove', handleMouseMove);
    document.addEventListener('mouseup', handleMouseUp);
    return () => {
      document.removeEventListener('mousemove', handleMouseMove);
      document.removeEventListener('mouseup', handleMouseUp);
    };
  }, [isDragging]);

  const resetPosition = useCallback(() => {
    setOffset(null);
    setIsDragging(false);
  }, []);

  const cursor = !enabled ? 'default' : isDragging ? 'grabbing' : 'grab';

  return {
    offset,
    isDragging,
    handleProps: {
      onMouseDown: handleMouseDown,
      style: { cursor, userSelect: 'none' },
    },
    resetPosition,
  };
}
