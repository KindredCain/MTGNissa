import { useId, useRef, useState, type CSSProperties } from "react";
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
import { Mana } from "../cards/Mana";
import {
  compare_cards,
  section_names,
  type_names,
  type_order,
  type Deck,
  type DeckCard,
  type SectionType,
} from "./model";
import type { EditorStore } from "./store";
import styles from "./decks.module.css";
import { useVisualViewport } from "../ui/viewport";
/* dnd-kit returns refs and render state in the same hook result. Its public
 * isDragging/listeners/setNodeRef fields are intentionally read during render. */
/* eslint-disable react-hooks/refs */
type Preview = { card: DeckCard; rect: DOMRect; exception?: boolean };
export function Board({
  deck,
  editor,
  cover_mode = false,
  on_cover,
}: {
  deck: Deck;
  editor?: EditorStore;
  cover_mode?: boolean;
  on_cover?: () => void;
}) {
  const [pinned, setPinned] = useState<string | null>(null),
    [hover, setHover] = useState<string | null>(null),
    [preview, setPreview] = useState<Preview | null>(null),
    [drag, setDrag] = useState<{
      card: DeckCard;
      section: SectionType;
      quantity: number;
    } | null>(null);
  const navigate = useNavigate(),
    pressed = useRef(0),
    pinned_preview = useRef<Preview | null>(null),
    suppress_click = useRef(false);
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
  const cover_oracle = deck.sections
    .flatMap((section) => section.cards)
    .find((card) => card.printing_uuid === deck.cover_printing_uuid)?.oracle_id;
  const onDragStart = (event: DragStartEvent) => {
    const data = event.active.data.current as
      { card: DeckCard; section: SectionType } | undefined;
    if (data) {
      setDrag({
        ...data,
        quantity:
          performance.now() - pressed.current >= 350 ? data.card.quantity : 1,
      });
      setPreview(null);
      suppress_click.current = true;
    }
  };
  const finish = () => {
    setDrag(null);
    setTimeout(() => {
      suppress_click.current = false;
    }, 0);
  };
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
      onDragCancel={finish}
      onDragEnd={(event) => {
        if (drag && event.over && editor)
          editor
            .getState()
            .move(
              drag.section,
              event.over.id as SectionType,
              drag.card.oracle_id,
              drag.quantity,
            );
        finish();
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
                  ⚠ 数量不足 {shortage.count}种·缺{shortage.missing}张
                </span>
              )}
              {usage.count > 0 && (
                <span className={`${styles.badge} ${styles.usage}`}>
                  ▱ 卡组占用 {usage.count}种·缺{usage.missing}张
                </span>
              )}
              <small>共 {totals} 张牌</small>
            </div>
            <div className={styles.regionScroll}>
              <div
                className={styles.columns}
                style={{
                  gridTemplateColumns: `repeat(${columns.length},150px)`,
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
                                ? "指挥官"
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
                            expanded = hover === id || pinned === id;
                          return (
                            <DraggableCard
                              key={card.oracle_id}
                              card={card}
                              section={section}
                              expanded={expanded}
                              disabled={
                                !editor || cover_mode || card.quantity === 0
                              }
                              selected={
                                !!editor && cover_oracle === card.oracle_id
                              }
                              onPress={() => {
                                pressed.current = performance.now();
                              }}
                              onEnter={(rect) => {
                                if (drag) return;
                                setHover(id);
                                setPreview({ card, rect });
                              }}
                              onLeave={() => {
                                setHover(null);
                                if (card.quantity === 0 && pinned === id) {
                                  pinned_preview.current = null;
                                  setPinned(null);
                                }
                                setPreview(pinned_preview.current);
                                editor?.getState().remove_zero();
                              }}
                              onClick={(rect) => {
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
                                setPinned(pinned === id ? null : id);
                                pinned_preview.current =
                                  pinned === id ? null : { card, rect };
                                setPreview(
                                  pinned === id ? null : { card, rect },
                                );
                              }}
                              onException={(rect) =>
                                setPreview({ card, rect, exception: true })
                              }
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
        !drag &&
        createPortal(
          <PreviewBubble preview={preview} deck={deck} />,
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
  disabled,
  selected,
  onPress,
  onEnter,
  onLeave,
  onClick,
  onException,
  editor,
  cover_mode,
}: {
  card: DeckCard;
  section: SectionType;
  expanded: boolean;
  disabled: boolean;
  selected: boolean;
  onPress: () => void;
  onEnter: (rect: DOMRect) => void;
  onLeave: () => void;
  onClick: (rect: DOMRect) => void;
  onException: (rect: DOMRect) => void;
  editor?: EditorStore;
  cover_mode: boolean;
}) {
  const draggable = useDraggable({
      id: `${section}:${card.oracle_id}`,
      disabled,
      data: { card, section },
    }),
    [back, setBack] = useState(false);
  const ref = useRef<HTMLDivElement>(null),
    rect = () => ref.current!.getBoundingClientRect();
  const quick = (delta: number) =>
    editor?.getState().quantity(section, card.oracle_id, delta);
  return (
    <div
      ref={ref}
      className={`${styles.slot} ${expanded ? styles.expanded : ""}`}
      style={{ zIndex: expanded ? 4 : undefined }}
      onPointerEnter={() => onEnter(rect())}
      onPointerLeave={onLeave}
      onFocus={() => onEnter(rect())}
      onBlur={(event) => {
        if (!event.currentTarget.contains(event.relatedTarget)) onLeave();
      }}
    >
      <button
        ref={draggable.setNodeRef}
        {...draggable.attributes}
        {...draggable.listeners}
        className={`${styles.cardButton} ${selected ? styles.selected : ""}`}
        style={
          {
            opacity: draggable.isDragging ? 0.3 : 1,
            touchAction: disabled ? "auto" : "none",
          } as CSSProperties
        }
        onPointerDownCapture={onPress}
        onContextMenu={(event) => {
          if (editor) event.preventDefault();
        }}
        onClick={() => onClick(rect())}
        aria-label={`${card.card.display_name}，${card.quantity} 张`}
      >
        <CardImage
          className={styles.deckImage}
          images={card.card.images}
          name={card.card.display_name}
          back={back}
        />
        <span className={styles.quantity}>{card.quantity}</span>
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
            setBack((value) => !value);
          }}
        >
          ↻
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
            −
          </button>
          <i />
          <button
            aria-label={`增加${card.card.display_name}`}
            onClick={() => quick(1)}
          >
            ＋
          </button>
        </div>
      )}
      {card.exception && (
        <button
          className={`${styles.exception} ${card.exception.type === "deck_usage" ? styles.usage : ""}`}
          aria-label={
            card.exception.type === "shortage" ? "数量不足" : "卡组占用"
          }
          onPointerDown={(event) => event.stopPropagation()}
          onPointerEnter={(event) => {
            event.stopPropagation();
            onException(rect());
          }}
          onFocus={() => onException(rect())}
          onClick={(event) => {
            event.stopPropagation();
            onException(rect());
          }}
        >
          {card.exception.type === "shortage" ? "!" : "▱"}
        </button>
      )}
    </div>
  );
}
export function PreviewBubble({
  preview,
  deck,
}: {
  preview: Preview;
  deck?: Deck;
}) {
  const viewport = useVisualViewport();
  const { card, rect, exception } = preview;
  const width = exception ? 232 : 440,
    left =
      rect.right + 12 + width < viewport.left + viewport.width
        ? rect.right + 12
        : Math.max(viewport.left + 8, rect.left - width - 12);
  const top = Math.max(
    8,
    Math.min(
      rect.top,
      viewport.top + viewport.height - (exception ? 100 : 340),
    ),
  );
  const current =
    deck?.sections
      .filter((section) => section.type !== "maybeboard")
      .flatMap((section) => section.cards)
      .filter((item) => item.oracle_id === card.oracle_id)
      .reduce((sum, item) => sum + item.quantity, 0) ?? card.quantity;
  if (exception)
    return (
      <div className={styles.exceptionInfo} style={{ left, top }}>
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
            "使用卡组数",
            deck
              ? card.other_decks_count + (current > 0 ? 1 : 0)
              : card.using_decks,
          ],
        ].map(([name, value]) => (
          <div key={name}>
            {name}
            <strong>{value}</strong>
          </div>
        ))}
      </div>
    );
  return (
    <div className={styles.bubble} style={{ left, top }}>
      <CardImage
        className={styles.deckImage}
        images={card.card.images}
        name={card.card.display_name}
      />
      <div>
        <h3>{card.card.display_name}</h3>
        <Mana cost={card.statistics.mana_cost} />
        <p>{card.translated_type_line}</p>
        <p>{card.translated_oracle_text}</p>
      </div>
    </div>
  );
}
