import {
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type RefObject,
} from "react";
import * as Dialog from "@radix-ui/react-dialog";
import * as echarts from "echarts/core";
import { BarChart } from "echarts/charts";
import { GridComponent, TooltipComponent } from "echarts/components";
import { CanvasRenderer } from "echarts/renderers";
import { Select } from "../ui/controls";
import { Mana } from "../cards/Mana";
import { color_names } from "../shared/display-names";
import { colors, palette, type Deck } from "./model";
import {
  curve,
  group_cards,
  overview,
  probability,
  type Grouping,
} from "./calculations";
import styles from "./statistics.module.css";
import { useVisualViewport } from "../ui/viewport";
import { Icon } from "../ui/Icon";
echarts.use([BarChart, GridComponent, TooltipComponent, CanvasRenderer]);
const group_options = [
  { value: "type", label: "类型" },
  { value: "color", label: "颜色" },
  { value: "mana", label: "法术力值" },
  { value: "tag", label: "功能标签" },
  { value: "produced", label: "产费颜色" },
];
export default function Statistics({
  deck,
  close,
  bookmark_top,
  trigger_ref,
}: {
  deck: Deck;
  close: () => void;
  bookmark_top: number;
  trigger_ref: RefObject<HTMLButtonElement | null>;
}) {
  const viewport = useVisualViewport();
  const drawer = useRef<HTMLDivElement>(null),
    [motion, setMotion] = useState<"entering" | "open" | "closing">("entering");
  useLayoutEffect(() => {
    // Establish the offscreen position before starting the CSS transition.
    drawer.current?.getBoundingClientRect();
    const frame = requestAnimationFrame(() =>
      setMotion((current) => (current === "entering" ? "open" : current)),
    );
    return () => cancelAnimationFrame(frame);
  }, []);
  const [curves, setCurves] = useState<Grouping>("type"),
    [prob, setProb] = useState<Grouping>("type"),
    [distribution, setDistribution] = useState<Grouping>("type"),
    [all_curves, setAllCurves] = useState(false),
    [all_prob, setAllProb] = useState(false),
    [requested_at_least, setAtLeast] = useState(1),
    [requested_draws, setDraws] = useState(7);
  const summary = overview(deck),
    probability_cards = deck.sections
      .filter((section) => section.type === "mainboard")
      .flatMap((section) => section.cards),
    probability_total = probability_cards.reduce(
      (sum, card) => sum + card.quantity,
      0,
    ),
    nonlands = summary.cards.filter((card) => !card.statistics.is_land),
    curve_groups = group_cards(nonlands, curves),
    prob_groups = group_cards(probability_cards, prob).filter(
      (group) => prob !== "mana" || group.quantity > 0,
    ),
    distribution_groups = group_cards(summary.cards, distribution).slice(0, 24);
  const cost_total = summary.cost.reduce((a, b) => a + b, 0),
    produced_total = summary.produced.reduce((a, b) => a + b, 0),
    draws = probability_total
      ? Math.max(1, Math.min(requested_draws, probability_total))
      : 0,
    at_least = draws ? Math.max(1, Math.min(requested_at_least, draws)) : 0;
  return (
    <Dialog.Root
      open
      onOpenChange={(value) => {
        if (!value) setMotion("closing");
      }}
    >
      <Dialog.Portal>
        <Dialog.Overlay
          className={styles.overlay}
          style={{
            left: viewport.left,
            top: viewport.top,
            width: viewport.width,
            height: viewport.height,
            right: "auto",
            bottom: "auto",
          }}
          onClick={(event) => {
            if (event.target === event.currentTarget) setMotion("closing");
          }}
        >
          <Dialog.Content
            ref={drawer}
            className={styles.drawer}
            data-motion={motion}
            aria-describedby={undefined}
            onPointerDownOutside={() => setMotion("closing")}
            onCloseAutoFocus={(event) => {
              event.preventDefault();
              trigger_ref.current?.focus({ preventScroll: true });
            }}
            onTransitionEnd={(event) => {
              if (
                event.target === event.currentTarget &&
                event.propertyName === "transform" &&
                motion === "closing"
              )
                close();
            }}
          >
            <button
              className={styles.bookmark}
              style={{ top: bookmark_top - 150 }}
              onClick={() => setMotion("closing")}
            >
              <Icon name="chart-no-axes-column-increasing" size={15} />
              <span>卡组统计</span>
            </button>
            <Dialog.Title className={styles.title}>卡组统计</Dialog.Title>
            <p className={styles.subtitle}>统计主牌 · 备牌、备选不计入</p>
            <div className={styles.overview}>
              <div className={styles.keyMetrics}>
                <div>
                  <span>平均法术力值</span>
                  <strong>{summary.average.toFixed(2)}</strong>
                </div>
                <div>
                  <span>地牌</span>
                  <strong>{summary.lands}</strong>
                </div>
              </div>
              <div className={styles.manaOverview}>
                <ColorDistribution label="费用颜色" values={summary.cost} />
                <ColorDistribution label="产费颜色" values={summary.produced} />
              </div>
            </div>
            <div className={styles.colors}>
              {colors.map((color, index) => (
                <div
                  key={color}
                  style={{ opacity: summary.cost[index] === 0 ? 0.4 : 1 }}
                >
                  <Mana cost={`{${color}}`} />
                  <span>费用</span>
                  <div className={styles.bar}>
                    <i
                      style={{
                        width: `${cost_total ? (summary.cost[index] / cost_total) * 100 : 0}%`,
                        background: palette[index],
                      }}
                    />
                  </div>
                  <small>
                    {summary.cost[index]}点·{summary.cost_cards[index]}张
                  </small>
                  <span>产费</span>
                  <div className={styles.bar}>
                    <i
                      style={{
                        width: `${produced_total ? (summary.produced[index] / produced_total) * 100 : 0}%`,
                        background: palette[index],
                      }}
                    />
                  </div>
                  <small>
                    {summary.produced[index]}点·{summary.produced_cards[index]}
                    张
                  </small>
                </div>
              ))}
            </div>
            <section className={styles.module}>
              <h3>法术力曲线</h3>
              <Chart
                labels={["0", "1", "2", "3", "4", "5", "6", "7", "8+"]}
                values={curve(summary.cards)}
              />
            </section>
            <section className={styles.module}>
              <div className={styles.moduleTitle}>
                <h3>法术力曲线分组</h3>
                <Select
                  label="法术力曲线分组"
                  value={curves}
                  options={group_options.filter((item) =>
                    ["type", "color", "tag"].includes(item.value),
                  )}
                  onChange={(value) => {
                    setCurves(value as Grouping);
                    setAllCurves(false);
                  }}
                />
              </div>
              {curves === "tag" && (
                <div className={styles.groupNotes}>
                  <span>按牌张数量排序 · 默认显示前 12 个</span>
                  <span>同一张牌可计入多个标签</span>
                </div>
              )}
              <div className={styles.curves}>
                {(curves === "tag" && !all_curves
                  ? curve_groups.slice(0, 12)
                  : curve_groups
                ).map((group) => (
                  <div key={group.key}>
                    <div className={styles.curveTitle}>
                      <h4 title={group.name}>{group.name}</h4>
                      <small>{group.quantity}张</small>
                    </div>
                    <Chart
                      labels={["0", "1", "2", "3", "4", "5", "6", "7", "8+"]}
                      values={curve(group.cards)}
                      color={group.color}
                      small
                    />
                  </div>
                ))}
              </div>
              {curves === "tag" && curve_groups.length > 12 && (
                <ExpandButton
                  expanded={all_curves}
                  total={curve_groups.length}
                  onClick={() => setAllCurves((value) => !value)}
                />
              )}
            </section>
            <section className={styles.module}>
              <div className={styles.moduleTitle}>
                <h3>抽到概率</h3>
                <div className={styles.condition}>
                  至少{" "}
                  <NumberStepper
                    label="至少张数"
                    value={at_least}
                    min={probability_total ? 1 : 0}
                    max={draws}
                    disabled={!probability_total}
                    change={setAtLeast}
                  />{" "}
                  张 抽{" "}
                  <NumberStepper
                    label="抽牌张数"
                    value={draws}
                    min={at_least}
                    max={probability_total}
                    disabled={!probability_total}
                    change={setDraws}
                  />{" "}
                  张
                </div>
                <Select
                  label="概率分组"
                  value={prob}
                  options={group_options.filter((item) =>
                    ["type", "mana", "tag", "produced"].includes(item.value),
                  )}
                  onChange={(value) => {
                    setProb(value as Grouping);
                    setAllProb(false);
                  }}
                />
              </div>
              <table className={styles.probability}>
                <thead>
                  <tr>
                    <th>
                      {group_options.find((item) => item.value === prob)?.label}
                    </th>
                    <th>牌张数</th>
                    <th>概率</th>
                  </tr>
                </thead>
                <tbody>
                  {(prob === "tag" && !all_prob
                    ? prob_groups.slice(0, 12)
                    : prob_groups
                  ).map((group) => (
                    <tr key={group.key}>
                      <td>
                        <span className={styles.probabilityName}>
                          {prob === "produced" &&
                            (group.key === "none" ? (
                              <Icon name="circle-off" />
                            ) : (
                              <Mana cost={`{${group.key}}`} />
                            ))}
                          {group.name}
                        </span>
                      </td>
                      <td>{group.quantity}</td>
                      <td>
                        {(
                          (probability_total
                            ? probability(
                                probability_total,
                                group.quantity,
                                draws,
                                at_least,
                              )
                            : 0) * 100
                        ).toFixed(2)}
                        %
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
              {prob === "tag" && prob_groups.length > 12 && (
                <ExpandButton
                  expanded={all_prob}
                  total={prob_groups.length}
                  onClick={() => setAllProb((value) => !value)}
                />
              )}
            </section>
            <section className={styles.module}>
              <div className={styles.moduleTitle}>
                <h3>数量分布</h3>
                <Select
                  label="数量分布分组"
                  value={distribution}
                  options={group_options.filter((item) =>
                    ["type", "color", "tag"].includes(item.value),
                  )}
                  onChange={(value) => setDistribution(value as Grouping)}
                />
              </div>
              <Chart
                labels={distribution_groups.map((group) => group.name)}
                values={distribution_groups.map((group) => group.quantity)}
                quantity_distribution
                color={
                  distribution === "color"
                    ? distribution_groups.map((group) => group.color)
                    : "#FA890D"
                }
              />
            </section>
          </Dialog.Content>
        </Dialog.Overlay>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
function ColorDistribution({
  label,
  values,
}: {
  label: string;
  values: number[];
}) {
  const total = values.reduce((sum, value) => sum + value, 0),
    describe = (value: number, index: number) =>
      `${color_names[colors[index]]} ${value} 点（${((value / total) * 100).toFixed(1)}%）`;
  return (
    <div className={styles.colorDistribution}>
      <span>{label}</span>
      <div
        className={styles.colorTrack}
        role="img"
        aria-label={`${label}：${total > 0 ? values.map(describe).join("，") : "共 0 点"}`}
      >
        {total > 0 &&
          values.map(
            (value, index) =>
              value > 0 && (
                <i
                  key={colors[index]}
                  title={describe(value, index)}
                  style={{ flexGrow: value, background: palette[index] }}
                />
              ),
          )}
      </div>
    </div>
  );
}
function ExpandButton({
  expanded,
  total,
  onClick,
}: {
  expanded: boolean;
  total: number;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      className={styles.expandButton}
      aria-expanded={expanded}
      onClick={onClick}
    >
      {expanded ? "收起" : `显示全部（共 ${total} 个）`}
      <Icon name={expanded ? "chevron-up" : "chevron-down"} size={12} />
    </button>
  );
}

function NumberStepper({
  label,
  value,
  min,
  max,
  disabled = false,
  change,
}: {
  label: string;
  value: number;
  min: number;
  max: number;
  disabled?: boolean;
  change: (value: number) => void;
}) {
  const [input, setInput] = useState<string | null>(null);
  function commit() {
    const number = input === null ? value : Number(input);
    change(
      Math.max(
        min,
        Math.min(max, Number.isFinite(number) ? Math.trunc(number) : min),
      ),
    );
    setInput(null);
  }
  return (
    <div className={styles.stepper}>
      <button
        aria-label={`减少${label}`}
        disabled={disabled || value <= min}
        onClick={() => change(Math.max(min, value - 1))}
      >
        <Icon name="minus" />
      </button>
      <input
        aria-label={label}
        type="number"
        min={min}
        max={max}
        step={1}
        disabled={disabled}
        value={disabled ? value : (input ?? value)}
        onFocus={() => setInput(String(value))}
        onBlur={commit}
        onKeyDown={(event) => {
          if (event.key === "Enter") event.currentTarget.blur();
        }}
        onChange={(event) => {
          const raw = event.target.value;
          setInput(raw);
          const number = Number(raw);
          if (
            raw !== "" &&
            Number.isInteger(number) &&
            number >= min &&
            number <= max
          )
            change(number);
        }}
      />
      <button
        aria-label={`增加${label}`}
        disabled={disabled || value >= max}
        onClick={() => change(Math.min(max, value + 1))}
      >
        <Icon name="plus" />
      </button>
    </div>
  );
}
function Chart({
  labels,
  values,
  color = "#FA890D",
  small = false,
  quantity_distribution = false,
}: {
  labels: string[];
  values: number[];
  color?: string | string[];
  small?: boolean;
  quantity_distribution?: boolean;
}) {
  const element = useRef<HTMLDivElement>(null),
    chart = useRef<echarts.EChartsType | null>(null);
  useEffect(() => {
    if (!element.current) return;
    const instance = echarts.init(element.current);
    chart.current = instance;
    const observer = new ResizeObserver(() => instance.resize());
    observer.observe(element.current);
    return () => {
      observer.disconnect();
      instance.dispose();
      chart.current = null;
    };
  }, []);
  useEffect(() => {
    const label_width = quantity_distribution
      ? Math.max(
          12,
          ((chart.current?.getWidth() ?? 1000) - 48) /
            Math.max(1, labels.length) -
            8,
        )
      : small
        ? 12
        : 50;
    chart.current?.setOption(
      {
        animation: false,
        grid: {
          top: small ? 20 : 28,
          bottom: small ? 20 : quantity_distribution ? 68 : 55,
          left: small ? 20 : 36,
          right: 12,
        },
        tooltip: { trigger: "axis", confine: true },
        xAxis: {
          type: "category",
          data: labels,
          axisLabel: {
            color: "#a8b8ad",
            fontSize: small ? 8 : 10,
            interval: 0,
            width: label_width,
            overflow: quantity_distribution ? "break" : "truncate",
            lineOverflow: "truncate",
            height: quantity_distribution ? 36 : undefined,
            lineHeight: quantity_distribution ? 12 : undefined,
            rotate: 0,
          },
          axisLine: { lineStyle: { color: "#3c5547" } },
        },
        yAxis: {
          type: "value",
          minInterval: 1,
          axisLabel: { color: "#a8b8ad", fontSize: 9 },
          splitLine: { lineStyle: { color: "#2c4437" } },
        },
        series: [
          {
            type: "bar",
            data: values.map((value, index) => ({
              value,
              itemStyle: { color: Array.isArray(color) ? color[index] : color },
            })),
            barMaxWidth: quantity_distribution ? undefined : small ? 12 : 52,
            barWidth: quantity_distribution ? "70%" : undefined,
            label: {
              show: true,
              position: "top",
              color: "#d7e5db",
              fontSize: small ? 8 : 10,
            },
          },
        ],
      },
      true,
    );
  }, [labels, values, color, small, quantity_distribution]);
  return (
    <div
      ref={element}
      role="img"
      aria-label={labels
        .map((label, index) => `${label}：${values[index]}`)
        .join("，")}
      style={{ height: small ? 105 : 240, width: "100%" }}
    />
  );
}
