import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { QueryClientProvider } from "@tanstack/react-query";
import { RouterProvider } from "react-router/dom";
import { query_client } from "./api/query";
import { FeedbackProvider } from "./ui/feedback";
import { router } from "./app/router";
import "./global.css";
createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <FeedbackProvider>
      <QueryClientProvider client={query_client}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    </FeedbackProvider>
  </StrictMode>,
);
