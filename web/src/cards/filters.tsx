import { MultiSelect, Select, SortDivider } from "../ui/controls";
import {
  color_options,
  mana_options,
  rarity_options,
  local_sort,
} from "./model";
export function CardFilters({
  params,
  update,
  sort = true,
}: {
  params: URLSearchParams;
  update: (values: Record<string, string | string[] | undefined>) => void;
  sort?: boolean;
}) {
  return (
    <>
      {[
        { key: "color", label: "颜色", options: color_options },
        { key: "mana_value", label: "法术力", options: mana_options },
        { key: "rarity", label: "稀有度", options: rarity_options },
      ].map((filter) => (
        <MultiSelect
          key={filter.key}
          label={filter.label}
          options={filter.options}
          value={params.getAll(filter.key)}
          onChange={(value) => update({ [filter.key]: value })}
        />
      ))}
      {sort && (
        <>
          <SortDivider />
          <Select
            label="排序"
            prefix="排序："
            value={params.get("sort") ?? "collector_number"}
            options={local_sort}
            onChange={(value) => update({ sort: value })}
          />
        </>
      )}
    </>
  );
}
