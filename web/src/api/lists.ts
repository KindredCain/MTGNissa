import { infiniteQueryOptions } from "@tanstack/react-query";
import { api, type DTO, type QueryParams } from "./client";

// A confirmed cancellation stays visible until the next list request.
export type WishlistItemView = Omit<DTO<"WishlistItemResponse">, "included"> &
  Pick<DTO<"WishlistResponse">, "included">;
export type WishlistPageView = Omit<DTO<"WishlistPageResponse">, "items"> & {
  items: WishlistItemView[];
};
export function set_params(params: URLSearchParams) {
  return {
    q: params.get("q") ?? undefined,
    language: params.getAll("language"),
    set_type: params.getAll("set_type"),
    sort: (params.get("sort") ?? "released_desc") as
      | "released_desc"
      | "released_asc"
      | "name_asc"
      | "name_desc"
      | "code_asc"
      | "code_desc"
      | "card_count_desc"
      | "card_count_asc",
  };
}
export function personal_params(params: URLSearchParams) {
  return {
    q: params.get("q") ?? undefined,
    set_code: params.getAll("set_code"),
    language: params.getAll("language"),
    color: params.getAll("color") as NonNullable<
      QueryParams<"/api/v1/collection">
    >["color"],
    mana_value: params.getAll("mana_value") as NonNullable<
      QueryParams<"/api/v1/collection">
    >["mana_value"],
    rarity: params.getAll("rarity") as NonNullable<
      QueryParams<"/api/v1/collection">
    >["rarity"],
    sort: (params.get("sort") ?? "set") as
      "set" | "color" | "mana_value" | "name",
  };
}
export function collection_params(params: URLSearchParams) {
  const query = personal_params(params);
  return {
    ...query,
    merge_sets:
      query.sort === "set" ? false : params.get("merge_sets") === "true",
    merge_languages: params.get("merge_languages") !== "false",
  };
}
export function deck_params(params: URLSearchParams) {
  return {
    q: params.get("q") ?? undefined,
    sort: (params.get("sort") ?? "updated_desc") as
      "updated_desc" | "created_desc" | "name_asc" | "card_count_desc",
  };
}
export function sets_options(params: URLSearchParams) {
  const query = set_params(params);
  return infiniteQueryOptions({
    queryKey: ["sets", "list", query],
    initialPageParam: null as string | null,
    queryFn: ({ pageParam, signal }) =>
      api.get(
        "/api/v1/sets",
        { ...query, cursor: pageParam ?? undefined },
        { signal },
      ),
    getNextPageParam: (last) => last.next_cursor ?? undefined,
  });
}
export function collection_options(params: URLSearchParams) {
  const query = collection_params(params);
  return infiniteQueryOptions({
    queryKey: ["collection", "list", query],
    initialPageParam: null as string | null,
    queryFn: ({ pageParam, signal }) =>
      api.get(
        "/api/v1/collection",
        { ...query, cursor: pageParam ?? undefined },
        { signal },
      ),
    getNextPageParam: (last) => last.next_cursor ?? undefined,
  });
}
export function wishlist_options(params: URLSearchParams) {
  const query = personal_params(params);
  return infiniteQueryOptions({
    queryKey: ["wishlist", "list", query],
    initialPageParam: null as string | null,
    queryFn: ({ pageParam, signal }): Promise<WishlistPageView> =>
      api.get(
        "/api/v1/wishlist",
        { ...query, cursor: pageParam ?? undefined },
        { signal },
      ),
    getNextPageParam: (last) => last.next_cursor ?? undefined,
  });
}
export function decks_options(params: URLSearchParams) {
  const query = deck_params(params);
  return infiniteQueryOptions({
    queryKey: ["decks", "list", query],
    initialPageParam: null as string | null,
    queryFn: ({ pageParam, signal }) =>
      api.get(
        "/api/v1/decks",
        { ...query, cursor: pageParam ?? undefined },
        { signal },
      ),
    getNextPageParam: (last) => last.next_cursor ?? undefined,
  });
}
export function search_options(q: string) {
  return infiniteQueryOptions({
    queryKey: ["search", q],
    initialPageParam: null as string | null,
    queryFn: ({ pageParam, signal }) =>
      api.get(
        "/api/v1/search",
        { q, cursor: pageParam ?? undefined },
        { signal },
      ),
    getNextPageParam: (last) => last.next_cursor ?? undefined,
  });
}
