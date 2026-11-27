import {
  Fragment,
  useEffect,
  useId,
  useRef,
  useState,
  type CSSProperties,
} from "react";
import { createPortal } from "react-dom";
import {
  DndContext,
  DragOverlay,
  PointerSensor,
  pointerWithin,
  useDraggable,
  useDroppable,
  useSensor,
  useSensors,
  type DragStartEvent,
} from "@dnd-kit/core";
import { useNavigate } from "react-router";
import { CardImage } from "../cards/CardImage";
import {
  exception_names,
  section_names,
  type_names,
} from "../shared/display-names";
import {
  compare_cards,
  type_order,
  type Deck,
  type DeckCard,
  type SectionType,
} from "./model";
import type { EditorStore } from "./store";
import { validation_error } from "../ui/feedback";
import styles from "./decks.module.css";
import { Icon } from "../ui/Icon";
import { useVisualViewport } from "../ui/viewport";
import { useAnchorRect, useHoverPreview } from "../ui/preview";
/* dnd-kit returns refs and render state in the same hook result. Its public
 * isDragging/listeners/setNodeRef fields are intentionally read during render. */
/* eslint-disable react-hooks/refs */
type Preview = {
  card: DeckCard;
  anchor: HTMLElement;
  exception?: boolean;
  section?: SectionType;
};
type Selection = { id: string; fixed: boolean };
export function Board({
  deck,
  editor,
  cover_mode = false,
  cover_oracle_id,
  on_cover,
}: {
  deck: Deck;
  editor?: EditorStore;
  cover_mode?: boolean;
  cover_oracle_id?: string;
  on_cover?: () => void;
}) {
  const [selection, setSelection] = useState<Selection | null>(null),
    [back_faces, setBackFaces] = useState<Record<string, boolean>>({}),
    [drag, setDrag] = useState<{
      card: DeckCard;
      section: SectionType;
      quantity: number;
    } | null>(null);
  const navigate = useNavigate(),
    pressed = useRef(0),
    selection_ref = useRef<Selection | null>(null),
    in_bubble = useRef(false),
    dragging = useRef(false),
    suppress_click = useRef(false);
  const {
    value: exception_preview,
    show: show_exception,
    keep: keep_exception,
    leave: leave_exception,
    hide: hide_exception,
  } = useHoverPreview<Preview>();
  const {
    value: preview,
    show: setPreview,
    keep: keep_preview,
    leave: hide_preview,
    hide: hide_bubble,
  } = useHoverPreview<Preview>(() => {
    const current = selection_ref.current;
    if (!current?.fixed) {
      selection_ref.current = null;
      setSelection(null);
      editor?.getState().remove_zero();
    }
    return null;
  });
  function select(next: Selection | null) {
    selection_ref.current = next;
    setSelection(next);
  }
  function clear_display() {
    in_bubble.current = false;
    select(null);
    setPreview(null);
    hide_exception();
    editor?.getState().remove_zero();
  }
  function enter(card: DeckCard, section: SectionType, anchor: HTMLElement) {
    if (dragging.current || suppress_click.current || in_bubble.current) return;
    const image = anchor.querySelector<HTMLButtonElement>(
      "[data-deck-card-image]",
    );
    if (!image) return;
    const id = `${section}:${card.oracle_id}`,
      previous = selection_ref.current,
      next = { card, section, anchor: image };
    select({ id, fixed: previous?.id === id && previous.fixed });
    hide_exception();
    setPreview(next);
    if (previous?.id !== id) editor?.getState().remove_zero();
  }
  useEffect(() => {
    if (!editor) return;
    function on_blank(event: PointerEvent) {
      if (
        !selection_ref.current ||
        !(event.target instanceof Element) ||
        document.querySelector('[role="dialog"]') ||
        event.target.closest(
          'button, a, input, textarea, select, label, [role="button"], [role="menu"], [data-deck-card], [data-deck-preview]',
        )
      )
        return;
      in_bubble.current = false;
      selection_ref.current = null;
      setSelection(null);
      setPreview(null);
      editor!.getState().remove_zero();
    }
    document.addEventListener("pointerdown", on_blank);
    return () => document.removeEventListener("pointerdown", on_blank);
  }, [editor, setPreview]);
  useEffect(() => {
    const current = selection_ref.current;
    if (
      current &&
      !deck.sections.some((section) =>
        section.cards.some(
          (card) => `${section.type}:${card.oracle_id}` === current.id,
        ),
      )
    ) {
      in_bubble.current = false;
      selection_ref.current = null;
      setSelection(null);
      setPreview(null);
    }
  }, [deck, setPreview]);
  useEffect(() => {
    if (
      exception_preview &&
      !deck.sections
        .find((section) => section.type === exception_preview.section)
        ?.cards.find(
          (card) => card.oracle_id === exception_preview.card.oracle_id,
        )?.exception
    )
      hide_exception();
  }, [deck, exception_preview, hide_exception]);
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 4 } }),
  );
  const others = deck.sections.some((section) =>
    section.cards.some((card) => card.type_group === "other"),
  );
  const columns = [
    "commander",
    ...type_order.filter((type) => type !== "other" || others),
  ];
  const cover_oracle =
    cover_oracle_id ??
    deck.sections
      .flatMap((section) => section.cards)
      .find((card) => card.printing_uuid === deck.cover_printing_uuid)
      ?.oracle_id;
  const onDragStart = (event: DragStartEvent) => {
    const data = event.active.data.current as
      { card: DeckCard; section: SectionType } | undefined;
    if (data) {
      dragging.current = true;
      setDrag({
        ...data,
        quantity:
          performance.now() - pressed.current >= 350 ? data.card.quantity : 1,
      });
      setPreview(null);
      hide_exception();
      suppress_click.current = true;
    }
  };
  const finish = (moved: boolean) => {
    dragging.current = false;
    setDrag(null);
    const current = selection_ref.current,
      remaining =
        moved && drag
          ? editor
              ?.getState()
              .deck.sections.find((section) => section.type === drag.section)
              ?.cards.find((card) => card.oracle_id === drag.card.oracle_id)
          : undefined;
    const keep_fixed =
      moved &&
      current?.fixed &&
      drag &&
      current.id === `${drag.section}:${drag.card.oracle_id}` &&
      remaining &&
      remaining.quantity > 0;
    if (moved && !keep_fixed) clear_display();
    else if (selection_ref.current?.fixed) setPreview(null);
    else hide_preview();
    setTimeout(() => {
      suppress_click.current = false;
    }, 0);
  };
  const preview_card = preview
    ? deck.sections
        .find((section) => section.type === preview.section)
        ?.cards.find((card) => card.oracle_id === preview.card.oracle_id)
    : undefined;
  const exception_card = exception_preview
    ? deck.sections
        .find((section) => section.type === exception_preview.section)
        ?.cards.find(
          (card) => card.oracle_id === exception_preview.card.oracle_id,
        )
    : undefined;
  return (
    <DndContext
      sensors={sensors}
      collisionDetection={(args) => {
        const hits = pointerWithin(args);
        return hits.some((hit) => hit.id === "commander")
          ? hits.filter((hit) => hit.id === "commander")
          : hits;
      }}
      onDragStart={onDragStart}
      onDragCancel={() => finish(false)}
      onDragEnd={(event) => {
        let moved = false;
        if (drag && event.over && editor) {
          const before = editor.getState().deck;
          const error = editor
            .getState()
            .move(
              drag.section,
              event.over.id as SectionType,
              drag.card.oracle_id,
              drag.quantity,
            );
          if (error) validation_error(error);
          moved = editor.getState().deck !== before;
        }
        finish(moved);
      }}
    >
      {(["mainboard", "sideboard", "maybeboard"] as const).map((kind) => {
        const sections = deck.sections.filter(
          (section) =>
            section.type === kind ||
            (kind === "mainboard" && section.type === "commander"),
        );
        const cards = sections.flatMap((section) => section.cards),
          totals = cards.reduce((sum, card) => sum + card.quantity, 0);
        const exceptions = (type: string) => {
          const map = new Map(
            cards
              .filter(
                (card) => card.exception?.type === type && card.quantity > 0,
              )
              .map((card) => [
                card.oracle_id,
                card.exception!.missing_quantity,
              ]),
          );
          return {
            count: map.size,
            missing: [...map.values()].reduce((a, b) => a + b, 0),
          };
        };
        const shortage = exceptions("shortage"),
          usage = exceptions("deck_usage");
        return (
          <DropRegion key={kind} kind={kind} editing={!!editor && !cover_mode}>
            <div className={styles.regionTitle}>
              <h2>{section_names[kind]}</h2>
              {shortage.count > 0 && (
                <span className={`${styles.badge} ${styles.shortage}`}>
                  <Icon name="triangle-alert" size={11} color="#ffaaa5" />
                  {exception_names.shortage} {shortage.count}种·缺
                  {shortage.missing}张
                </span>
              )}
              {usage.count > 0 && (
                <span className={`${styles.badge} ${styles.usage}`}>
                  <Icon name="layers" size={11} color="#ffd18a" />
                  {exception_names.deck_usage} {usage.count}种·缺
                  {usage.missing}张
                </span>
              )}
              <small>共 {totals} 张牌</small>
            </div>
            <div className={styles.regionScroll}>
              <div
                className={styles.columns}
                style={{
                  gridTemplateColumns: `repeat(${columns.length},158px)`,
                }}
              >
                {columns.map((type) => {
                  const section: SectionType =
                    type === "commander" ? "commander" : kind;
                  const items = [
                    ...(type === "commander"
                      ? kind === "mainboard"
                        ? (deck.sections.find(
                            (part) => part.type === "commander",
                          )?.cards ?? [])
                        : []
                      : (deck.sections
                          .find((part) => part.type === kind)
                          ?.cards.filter((card) => card.type_group === type) ??
                        [])),
                  ].sort(compare_cards);
                  return (
                    <DropColumn
                      key={type}
                      commander={type === "commander" && kind === "mainboard"}
                      editing={!!editor && !cover_mode}
                    >
                      <div className={styles.columnTitle}>
                        {items.length > 0 && (
                          <>
                            <span>
                              {type === "commander"
                                ? section_names.commander
                                : type_names[type as keyof typeof type_names]}
                            </span>
                            <span>
                              {items.reduce(
                                (sum, card) => sum + card.quantity,
                                0,
                              )}
                            </span>
                          </>
                        )}
                      </div>
                      <div className={styles.stack}>
                        {items.map((card) => {
                          const id = `${section}:${card.oracle_id}`,
                            expanded = selection?.id === id;
                          return (
                            <DraggableCard
                              key={card.oracle_id}
                              card={card}
                              section={section}
                              expanded={expanded}
                              back={!!back_faces[id]}
                              onFlip={() =>
                                setBackFaces((current) => ({
                                  ...current,
                                  [id]: !current[id],
                                }))
                              }
                              disabled={
                                !editor || cover_mode || card.quantity === 0
                              }
                              selected={
                                !!editor && cover_oracle === card.oracle_id
                              }
                              onPress={() => {
                                pressed.current = performance.now();
                              }}
                              onEnter={(anchor) => enter(card, section, anchor)}
                              onLeave={() => {
                                if (
                                  dragging.current ||
                                  in_bubble.current ||
                                  selection_ref.current?.id !== id
                                )
                                  return;
                                hide_preview();
                              }}
                              onClick={() => {
                                if (suppress_click.current) return;
                                if (cover_mode && editor) {
                                  editor.getState().cover(card.printing_uuid);
                                  on_cover?.();
                                  return;
                                }
                                if (!editor) {
                                  navigate(
                                    `/cards/${encodeURIComponent(card.printing_uuid)}`,
                                  );
                                  return;
                                }
                                if (
                                  selection_ref.current?.id === id &&
                                  selection_ref.current.fixed
                                ) {
                                  select({ id, fixed: false });
                                  return;
                                }
                                select({ id, fixed: true });
                              }}
                              onException={(anchor) => {
                                if (
                                  dragging.current ||
                                  suppress_click.current ||
                                  in_bubble.current
                                )
                                  return;
                                show_exception({
                                  card,
                                  anchor,
                                  section,
                                  exception: true,
                                });
                              }}
                              onExceptionLeave={leave_exception}
                              editor={editor}
                              cover_mode={cover_mode}
                            />
                          );
                        })}
                      </div>
                    </DropColumn>
                  );
                })}
              </div>
            </div>
          </DropRegion>
        );
      })}
      {preview &&
        preview_card &&
        !exception_preview &&
        !drag &&
        createPortal(
          <PreviewBubble
            preview={{ ...preview, card: preview_card }}
            deck={deck}
            onPointerEnter={() => {
              in_bubble.current = true;
              keep_preview();
            }}
            onPointerLeave={() => {
              in_bubble.current = false;
              hide_bubble();
              if (!selection_ref.current?.fixed) {
                select(null);
                editor?.getState().remove_zero();
              }
            }}
            back={!!back_faces[`${preview.section}:${preview.card.oracle_id}`]}
          />,
          document.body,
        )}
      {exception_preview &&
        exception_card?.exception &&
        !drag &&
        createPortal(
          <PreviewBubble
            preview={{ ...exception_preview, card: exception_card }}
            deck={deck}
            onPointerEnter={keep_exception}
            onPointerLeave={hide_exception}
          />,
          document.body,
        )}
      <DragOverlay dropAnimation={null}>
        {drag && (
          <div className={styles.dragGhost}>
            <CardImage
              images={drag.card.card.images}
              name={drag.card.card.display_name}
            />
            <span>{drag.quantity}</span>
          </div>
        )}
      </DragOverlay>
    </DndContext>
  );
}
function DropRegion({
  kind,
  editing,
  children,
}: {
  kind: SectionType;
  editing: boolean;
  children: React.ReactNode;
}) {
  const drop = useDroppable({ id: kind, disabled: !editing });
  return (
    <section
      ref={drop.setNodeRef}
      className={`${styles.region} ${drop.isOver ? styles.dropActive : ""}`}
    >
      {children}
    </section>
  );
}
function DropColumn({
  commander,
  editing,
  children,
}: {
  commander: boolean;
  editing: boolean;
  children: React.ReactNode;
}) {
  const unique = useId();
  const drop = useDroppable({
    id: commander ? "commander" : unique,
    disabled: !commander || !editing,
  });
  return (
    <div
      ref={commander ? drop.setNodeRef : undefined}
      className={`${styles.column} ${commander && drop.isOver ? styles.dropActive : ""}`}
    >
      {children}
    </div>
  );
}
function DraggableCard({
  card,
  section,
  expanded,
  back,
  onFlip,
  disabled,
  selected,
  onPress,
  onEnter,
  onLeave,
  onClick,
  onException,
  onExceptionLeave,
  editor,
  cover_mode,
}: {
  card: DeckCard;
  section: SectionType;
  expanded: boolean;
  back: boolean;
  onFlip: () => void;
  disabled: boolean;
  selected: boolean;
  onPress: () => void;
  onEnter: (anchor: HTMLElement) => void;
  onLeave: () => void;
  onClick: () => void;
  onException: (anchor: HTMLElement) => void;
  onExceptionLeave: () => void;
  editor?: EditorStore;
  cover_mode: boolean;
}) {
  const draggable = useDraggable({
    id: `${section}:${card.oracle_id}`,
    disabled,
    data: { card, section },
  });
  const ref = useRef<HTMLDivElement>(null),
    pointer_inside = useRef(false);
  const quick = (delta: number) => {
    const error = editor?.getState().quantity(section, card.oracle_id, delta);
    if (error) validation_error(error);
  };
  return (
    <div
      ref={ref}
      data-deck-card
      className={`${styles.slot} ${expanded ? styles.expanded : ""}`}
      style={{ zIndex: expanded ? 4 : undefined }}
      onPointerEnter={(event) => {
        if ((event.target as Element).closest("[data-deck-exception]")) return;
        pointer_inside.current = true;
        onEnter(event.currentTarget);
      }}
      onPointerLeave={() => {
        if (!pointer_inside.current) return;
        pointer_inside.current = false;
        onLeave();
      }}
      onFocus={(event) => {
        if (!event.target.closest("[data-deck-exception]"))
          onEnter(event.currentTarget);
      }}
      onBlur={(event) => {
        if (
          !event.target.closest("[data-deck-exception]") &&
          !pointer_inside.current &&
          !event.currentTarget.contains(event.relatedTarget)
        )
          onLeave();
      }}
    >
      <button
        ref={draggable.setNodeRef}
        data-deck-card-image
        {...draggable.attributes}
        aria-disabled={undefined}
        {...draggable.listeners}
        className={`${styles.cardButton} ${selected ? styles.selected : ""}`}
        style={
          {
            opacity: draggable.isDragging ? 0.3 : 1,
          } as CSSProperties
        }
        onPointerDownCapture={onPress}
        onContextMenu={(event) => {
          if (editor) event.preventDefault();
        }}
        onClick={onClick}
        aria-label={`${card.card.display_name}，${card.quantity} 张`}
      >
        <CardImage
          className={styles.deckImage}
          images={card.card.images}
          name={card.card.display_name}
          back={back}
        />
        <span
          className={styles.quantity}
          style={
            {
              "--quantity-size": `${Math.max(32, String(card.quantity).length * 7 + 22)}px`,
            } as CSSProperties
          }
        >
          {card.quantity}
        </span>
      </button>
      {expanded && card.card.images.has_distinct_back && (
        <button
          style={{
            position: "absolute",
            right: 20,
            top: 42,
            width: 24,
            height: 24,
            padding: 0,
          }}
          aria-label={`翻面：${card.card.display_name}`}
          onPointerDown={(event) => event.stopPropagation()}
          onClick={(event) => {
            event.stopPropagation();
            onFlip();
          }}
        >
          <Icon name="refresh-cw" size={15} />
        </button>
      )}
      {expanded && editor && !cover_mode && (
        <div
          className={styles.quick}
          onPointerDown={(event) => event.stopPropagation()}
          onClick={(event) => event.stopPropagation()}
        >
          <button
            aria-label={`减少${card.card.display_name}`}
            disabled={card.quantity === 0}
            onClick={() => quick(-1)}
          >
            <Icon name="minus" />
          </button>
          <i />
          <button
            aria-label={`增加${card.card.display_name}`}
            onClick={() => quick(1)}
          >
            <Icon name="plus" />
          </button>
        </div>
      )}
      {card.exception && (
        <button
          data-deck-exception
          className={`${styles.exception} ${card.exception.type === "deck_usage" ? styles.usage : ""}`}
          aria-label={exception_names[card.exception.type]}
          onPointerDown={(event) => event.stopPropagation()}
          onPointerEnter={(event) => {
            event.stopPropagation();
            pointer_inside.current = false;
            onLeave();
            onException(event.currentTarget);
          }}
          onPointerLeave={(event) => {
            onExceptionLeave();
            if (
              event.relatedTarget instanceof Node &&
              ref.current?.contains(event.relatedTarget)
            ) {
              pointer_inside.current = true;
              onEnter(ref.current);
            }
          }}
          onFocus={(event) => {
            event.stopPropagation();
            if (!pointer_inside.current) onLeave();
            onException(event.currentTarget);
          }}
          onBlur={onExceptionLeave}
          onClick={(event) => {
            event.stopPropagation();
            onException(event.currentTarget);
          }}
        >
          <Icon
            name={
              card.exception.type === "shortage" ? "triangle-alert" : "layers"
            }
            size={10}
            color={card.exception.type === "shortage" ? "#ffb0ab" : "#ffd18a"}
          />
        </button>
      )}
    </div>
  );
}
export function PreviewBubble({
  preview,
  deck,
  variant = "text",
  back = false,
  in_modal = false,
  onPointerEnter,
  onPointerLeave,
}: {
  preview: Preview;
  deck?: Deck;
  variant?: "text" | "image";
  back?: boolean;
  in_modal?: boolean;
  onPointerEnter?: () => void;
  onPointerLeave?: () => void;
}) {
  const viewport = useVisualViewport(),
    element = useRef<HTMLDivElement>(null),
    [height, setHeight] = useState(340);
  const { card, exception } = preview;
  const rect = useAnchorRect(preview.anchor);
  useEffect(() => {
    const node = element.current;
    if (!node) return;
    const observer = new ResizeObserver(() => setHeight(node.offsetHeight));
    observer.observe(node);
    return () => observer.disconnect();
  }, [variant, exception]);
  if (!rect) return null;
  const max_height = Math.min(480, viewport.height - 16);
  const width = exception ? 232 : 440,
    left =
      rect.right + 12 + width < viewport.left + viewport.width
        ? rect.right + 12
        : Math.max(viewport.left + 8, rect.left - width - 12);
  const top = Math.max(
    viewport.top + 8,
    Math.min(
      rect.top,
      viewport.top + viewport.height - (exception ? 100 : height) - 8,
    ),
  );
  const current =
    deck?.sections
      .filter((section) => section.type !== "maybeboard")
      .flatMap((section) => section.cards)
      .filter((item) => item.oracle_id === card.oracle_id)
      .reduce((sum, item) => sum + item.quantity, 0) ?? card.quantity;
  if (exception) {
    const opens_right =
      rect.right + 4 + 232 <= viewport.left + viewport.width - 8;
    const hint_left = opens_right ? rect.right + 4 : rect.left - 4 - 232;
    const hint_top = Math.max(
      viewport.top + 8,
      Math.min(rect.top - 5, viewport.top + viewport.height - 92 - 8),
    );
    const arrow_top = Math.max(
      6,
      Math.min(74, rect.top + rect.height / 2 - hint_top - 6),
    );
    const usage = card.exception?.type === "deck_usage";
    return (
      <div
        className={`${styles.exceptionInfo} ${usage ? styles.exceptionUsage : ""}`}
        data-deck-preview
        data-exception-side={opens_right ? "right" : "left"}
        style={
          {
            left: Math.max(viewport.left + 8, hint_left),
            top: hint_top,
            maxHeight: viewport.height - 16,
            "--exception-arrow-top": `${arrow_top}px`,
          } as CSSProperties
        }
        tabIndex={0}
        onPointerEnter={onPointerEnter}
        onPointerLeave={onPointerLeave}
        onFocus={onPointerEnter}
        onBlur={onPointerLeave}
      >
        <svg
          className={styles.exceptionArrow}
          width="7"
          height="12"
          viewBox="0 0 7 12"
          aria-hidden="true"
        >
          <path d={opens_right ? "M7 0 1 6 7 12" : "M0 0 6 6 0 12"} />
        </svg>
        <div className={styles.exceptionContent}>
          <div className={styles.exceptionTitle}>
            <svg
              width="14"
              height="14"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="1.7"
              strokeLinejoin="round"
              aria-hidden="true"
            >
              {usage ? (
                <>
                  <path d="m12 3 10 5-10 5L2 8Z" />
                  <path d="m2 12 10 5 10-5M2 16l10 5 10-5" />
                </>
              ) : (
                <>
                  <path d="M10.3 3.9 2 18.2A2 2 0 0 0 3.7 21h16.6a2 2 0 0 0 1.7-2.8L13.7 3.9a2 2 0 0 0-3.4 0Z" />
                  <path d="M12 9v4m0 4h.01" />
                </>
              )}
            </svg>
            <span>
              {exception_names[usage ? "deck_usage" : "shortage"]}{" "}
              {card.exception?.missing_quantity}
            </span>
          </div>
          <div className={styles.exceptionMetrics}>
            {[
              ["当前卡组", current],
              ["实际拥有", card.owned_quantity],
              [
                "所有卡组合计",
                deck
                  ? current + card.other_decks_quantity
                  : card.all_decks_quantity,
              ],
              [
                "使用卡组",
                deck
                  ? card.other_decks_count + (current > 0 ? 1 : 0)
                  : card.using_decks,
              ],
            ].map(([name, value]) => (
              <div key={name}>
                {name}
                <strong>
                  {value} {name === "使用卡组" ? "个" : "张"}
                </strong>
              </div>
            ))}
          </div>
        </div>
      </div>
    );
  }
  if (variant === "text")
    return (
      <TextPreview
        card={card}
        rect={rect}
        back={back}
        onPointerEnter={onPointerEnter}
        onPointerLeave={onPointerLeave}
      />
    );
  return (
    <div
      ref={element}
      role="tooltip"
      data-deck-preview
      className={styles.bubble}
      style={
        {
          // A centered modal's transform establishes the portal's containing block.
          left: in_modal
            ? `calc(50% + ${left - viewport.left - viewport.width / 2}px)`
            : left,
          top: in_modal
            ? `calc(50% + ${top - viewport.top - viewport.height / 2}px)`
            : top,
          "--bubble-max-height": `${max_height}px`,
        } as CSSProperties
      }
      onPointerEnter={onPointerEnter}
      onPointerLeave={onPointerLeave}
      onFocusCapture={onPointerEnter}
      onBlurCapture={(event) => {
        if (!event.currentTarget.contains(event.relatedTarget))
          onPointerLeave?.();
      }}
    >
      <CardImage
        className={styles.deckImage}
        images={card.card.images}
        name={card.card.display_name}
        back={back}
      />
      <div className={styles.bubbleText} tabIndex={0} aria-label="卡牌说明">
        <PreviewText card={card} back={back} />
      </div>
    </div>
  );
}

function PreviewText({ card, back }: { card: DeckCard; back: boolean }) {
  const active = card.faces[back ? 1 : 0] ?? card.faces[0],
    faces = card.card.images.has_distinct_back
      ? active
        ? [active]
        : []
      : card.faces;
  return faces.map((face) => (
    <Fragment key={face.face_index}>
      <h3>{face.display_name || face.name || card.card.display_name}</h3>
      <div>
        <small>类别</small>
        <p>{face.display_type_line ?? face.type_line}</p>
      </div>
      <div>
        <small>规则文字</small>
        <p>{face.display_oracle_text ?? face.oracle_text}</p>
      </div>
    </Fragment>
  ));
}

function TextPreview({
  card,
  rect,
  back,
  onPointerEnter,
  onPointerLeave,
}: {
  card: DeckCard;
  rect: DOMRect;
  back: boolean;
  onPointerEnter?: () => void;
  onPointerLeave?: () => void;
}) {
  const viewport = useVisualViewport(),
    element = useRef<HTMLDivElement>(null),
    [height, setHeight] = useState(164);
  useEffect(() => {
    const node = element.current;
    if (!node) return;
    const observer = new ResizeObserver(() => setHeight(node.offsetHeight));
    observer.observe(node);
    return () => observer.disconnect();
  }, []);
  const on_left = rect.right + 6 + 300 > viewport.left + viewport.width - 8,
    left = on_left
      ? Math.max(viewport.left + 8, rect.left - 300 - 6)
      : rect.right + 6,
    top = Math.max(
      viewport.top + 8,
      Math.min(rect.top, viewport.top + viewport.height - height - 8),
    );
  return (
    <div
      ref={element}
      role="tooltip"
      data-deck-preview
      className={styles.textPreview}
      onPointerEnter={onPointerEnter}
      onPointerLeave={onPointerLeave}
      onFocusCapture={onPointerEnter}
      onBlurCapture={(event) => {
        if (!event.currentTarget.contains(event.relatedTarget))
          onPointerLeave?.();
      }}
      data-side={on_left ? "left" : "right"}
      style={
        {
          left,
          top,
          "--preview-max-height": `${Math.min(360, viewport.height - 16)}px`,
          "--arrow-top": `${Math.max(16, Math.min(height - 16, rect.top + 48 - top))}px`,
        } as CSSProperties
      }
    >
      <div className={styles.previewText} tabIndex={0} aria-label="卡牌说明">
        <PreviewText card={card} back={back} />
      </div>
    </div>
  );
}
