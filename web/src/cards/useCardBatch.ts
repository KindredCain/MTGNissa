import { useEffect, useRef, useState } from "react";

const minimum_batch = 30;

export function useCardBatch() {
  const [grid, setGrid] = useState<HTMLDivElement | null>(null);
  const [visible, setVisible] = useState(minimum_batch);
  const columns = useRef(1);
  useEffect(() => {
    if (!grid) return;
    const observer = new ResizeObserver(() => {
      if (!grid.clientWidth) return;
      const count =
        getComputedStyle(grid).gridTemplateColumns.split(" ").length;
      columns.current = count;
      setVisible((current) => Math.ceil(current / count) * count);
    });
    observer.observe(grid);
    return () => observer.disconnect();
  }, [grid]);
  function show_more() {
    const batch = Math.ceil(minimum_batch / columns.current) * columns.current;
    setVisible((current) => current + batch);
  }
  return { visible, grid_ref: setGrid, show_more };
}
