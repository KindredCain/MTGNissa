import {
  ArrowUpRight,
  CalendarDays,
  ChartNoAxesColumnIncreasing,
  Check,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  ChevronUp,
  CircleAlert,
  CircleOff,
  CircleQuestionMark,
  ClipboardCopy,
  Clock3,
  Copy,
  EllipsisVertical,
  FileInput,
  FileText,
  FolderMinus,
  FolderPlus,
  Images,
  Languages,
  Layers,
  LoaderCircle,
  Minus,
  Pencil,
  PencilLine,
  Plus,
  RefreshCw,
  Save,
  Search,
  Sparkles,
  Star,
  StarOff,
  Trash2,
  TriangleAlert,
  X,
} from "lucide-react";
import styles from "./ui.module.css";

const icons = {
  "arrow-up-right": ArrowUpRight,
  "calendar-days": CalendarDays,
  "chart-no-axes-column-increasing": ChartNoAxesColumnIncreasing,
  check: Check,
  "chevron-down": ChevronDown,
  "chevron-left": ChevronLeft,
  "chevron-right": ChevronRight,
  "chevron-up": ChevronUp,
  "circle-alert": CircleAlert,
  "circle-off": CircleOff,
  "circle-question-mark": CircleQuestionMark,
  "clipboard-copy": ClipboardCopy,
  "clock-3": Clock3,
  copy: Copy,
  "ellipsis-vertical": EllipsisVertical,
  "file-input": FileInput,
  "file-text": FileText,
  "folder-minus": FolderMinus,
  "folder-plus": FolderPlus,
  images: Images,
  languages: Languages,
  layers: Layers,
  "loader-circle": LoaderCircle,
  minus: Minus,
  pencil: Pencil,
  "pencil-line": PencilLine,
  plus: Plus,
  "refresh-cw": RefreshCw,
  save: Save,
  search: Search,
  sparkles: Sparkles,
  star: Star,
  "star-off": StarOff,
  "trash-2": Trash2,
  "triangle-alert": TriangleAlert,
  x: X,
};
export type IconName = keyof typeof icons;

export function Icon({
  name,
  size = 14,
  filled = false,
  className = "",
  color,
}: {
  name: IconName;
  size?: number;
  filled?: boolean;
  className?: string;
  color?: string;
}) {
  const Glyph = icons[name];
  return (
    <Glyph
      size={size}
      color={color}
      fill={filled ? "currentColor" : "none"}
      className={`${styles.icon} ${className}`}
      aria-hidden="true"
      focusable="false"
    />
  );
}
