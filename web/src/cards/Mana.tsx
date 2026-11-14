import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import styles from "./detail.module.css";
type SymbolMap = Record<string, string>;
export function Mana({ cost }: { cost: string | null }) {
  const symbols = cost?.match(/\{[^}]+\}/g) ?? [];
  const complex = symbols.some(
    (symbol) => !/^\{([WUBRGCXYZS]|\d+)\}$/.test(symbol),
  );
  const mapping = useQuery({
    queryKey: ["symbology"],
    enabled: complex,
    staleTime: Infinity,
    retry: false,
    meta: { silent: true },
    queryFn: async ({ signal }) => {
      try {
        const response = await fetch("https://api.scryfall.com/symbology", {
          signal,
        });
        if (!response.ok) return {} as SymbolMap;
        const data = (await response.json()) as {
          data: { symbol: string; svg_uri: string }[];
        };
        return Object.fromEntries(
          data.data.map((item) => [item.symbol, item.svg_uri]),
        );
      } catch {
        return {} as SymbolMap;
      }
    },
  });
  return (
    <span className={styles.mana} aria-label={cost ?? ""}>
      {symbols.map((symbol, index) => {
        const simple = /^\{([WUBRGCXYZS]|\d+)\}$/.test(symbol);
        const url = simple
          ? `https://svgs.scryfall.io/card-symbols/${symbol.slice(1, -1)}.svg`
          : mapping.data?.[symbol];
        return (
          <ManaSymbol key={`${index}:${symbol}`} symbol={symbol} url={url} />
        );
      })}
    </span>
  );
}
function ManaSymbol({ symbol, url }: { symbol: string; url?: string }) {
  const [failed, setFailed] = useState(false);
  return url && !failed ? (
    <img
      src={url}
      alt={symbol}
      loading="lazy"
      onError={() => setFailed(true)}
    />
  ) : (
    <span>{symbol}</span>
  );
}
