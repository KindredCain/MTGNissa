import { MutationCache, QueryCache, QueryClient } from "@tanstack/react-query";
import { notify_error } from "../ui/feedback";
export const query_client = new QueryClient({
  queryCache: new QueryCache({ onError: notify_error }),
  mutationCache: new MutationCache({ onError: notify_error }),
  defaultOptions: {
    queries: { staleTime: 30_000, retry: false, refetchOnWindowFocus: false },
    mutations: { retry: false },
  },
});
query_client.setQueryDefaults(["deck_context"], { gcTime: Infinity });
