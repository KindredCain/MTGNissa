import { MutationCache, QueryCache, QueryClient } from "@tanstack/react-query";
import { notify_error } from "../ui/feedback";
const query_errors = new WeakSet<Error>();
export const query_client = new QueryClient({
  queryCache: new QueryCache({
    onError: (error) => {
      query_errors.add(error);
      notify_error(error);
    },
  }),
  mutationCache: new MutationCache({
    onError: (error) => {
      // A query failure propagated through a mutation has already been reported.
      if (!query_errors.has(error)) notify_error(error);
    },
  }),
  defaultOptions: {
    queries: { staleTime: 30_000, retry: false, refetchOnWindowFocus: false },
    mutations: { retry: false },
  },
});
query_client.setQueryDefaults(["deck_context"], { gcTime: Infinity });
