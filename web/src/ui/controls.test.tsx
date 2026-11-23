import {
  cleanup,
  act,
  fireEvent,
  render,
  renderHook,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { createMemoryRouter, RouterProvider } from "react-router";
import { MultiSelect, useURLState } from "./controls";
import { FeedbackProvider, validation_error } from "./feedback";
import { useAnchorRect, useHoverPreview } from "./preview";
afterEach(cleanup);
describe("shared interactions", () => {
  it("stops tracking a preview when its anchor is removed", () => {
    const anchor = document.createElement("button");
    document.body.append(anchor);
    const disconnect = vi.fn(),
      remove = vi.spyOn(document, "removeEventListener");
    vi.stubGlobal(
      "ResizeObserver",
      class {
        observe = vi.fn();
        disconnect = disconnect;
      },
    );
    try {
      const { result, rerender } = renderHook(() => useAnchorRect(anchor));
      expect(result.current).not.toBeNull();
      anchor.remove();
      rerender();
      expect(result.current).toBeNull();
      expect(disconnect).toHaveBeenCalledOnce();
      expect(remove).toHaveBeenCalledWith("scroll", expect.any(Function), true);
    } finally {
      cleanup();
      anchor.remove();
      remove.mockRestore();
      vi.unstubAllGlobals();
    }
  });
  it("delays leaving the trigger but hides immediately when leaving the preview", () => {
    vi.useFakeTimers();
    try {
      const { result, unmount } = renderHook(() =>
        useHoverPreview<string>(() => "pinned"),
      );
      act(() => result.current.show("first"));
      act(() => result.current.leave());
      act(() => vi.advanceTimersByTime(100));
      expect(result.current.value).toBe("first");
      act(() => result.current.keep());
      act(() => vi.advanceTimersByTime(200));
      expect(result.current.value).toBe("first");
      act(() => result.current.hide());
      expect(result.current.value).toBeNull();
      act(() => vi.advanceTimersByTime(200));
      expect(result.current.value).toBeNull();
      act(() => result.current.show("second"));
      act(() => result.current.leave());
      act(() => vi.advanceTimersByTime(150));
      expect(result.current.value).toBe("pinned");
      act(() => result.current.leave());
      unmount();
      expect(vi.getTimerCount()).toBe(0);
    } finally {
      vi.useRealTimers();
    }
  });

  for (const navigation of ["leave", "history", "late_submit"] as const) {
    it(`ignores pending URL updates after ${navigation}`, async () => {
      let release!: () => void;
      const pending = new Promise<void>((resolve) => {
        release = resolve;
      });
      function Filters() {
        const { update } = useURLState(() => pending);
        return (
          <button
            onClick={() => {
              if (navigation === "late_submit")
                void pending.then(() => update({ q: "slow" }));
              else update({ q: "slow" });
            }}
          >
            查询
          </button>
        );
      }
      const router = createMemoryRouter(
        [
          { path: "/sets", element: <Filters /> },
          { path: "/decks", element: <div>卡组列表</div> },
        ],
        { initialEntries: ["/sets", "/sets?q=current"], initialIndex: 1 },
      );
      render(<RouterProvider router={router} />);
      fireEvent.click(screen.getByRole("button", { name: "查询" }));
      await act(async () => {
        if (navigation === "history") await router.navigate(-1);
        else await router.navigate("/decks");
      });
      await act(async () => {
        release();
        await pending;
      });
      expect(router.state.location.pathname).toBe(
        navigation === "history" ? "/sets" : "/decks",
      );
      expect(router.state.location.search).toBe("");
      router.dispose();
    });
  }
  it("commits a successful filter when navigation has not changed", async () => {
    let release!: () => void;
    const pending = new Promise<void>((resolve) => {
      release = resolve;
    });
    function Filters() {
      const { update } = useURLState(() => pending);
      return <button onClick={() => update({ q: "accepted" })}>查询</button>;
    }
    const router = createMemoryRouter(
      [{ path: "/sets", element: <Filters /> }],
      { initialEntries: ["/sets"] },
    );
    render(<RouterProvider router={router} />);
    fireEvent.click(screen.getByRole("button", { name: "查询" }));
    expect(router.state.location.search).toBe("");
    await act(async () => {
      release();
      await pending;
    });
    expect(router.state.location.search).toBe("?q=accepted");
    router.dispose();
  });
  it("applies multi-select changes once when the popover closes", async () => {
    const change = vi.fn();
    render(
      <MultiSelect
        label="颜色"
        options={[
          { value: "W", label: "白" },
          { value: "U", label: "蓝" },
          { value: "B", label: "黑" },
        ]}
        value={[]}
        onChange={change}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "颜色" }));
    fireEvent.click(screen.getByRole("checkbox", { name: "白" }));
    fireEvent.click(screen.getByRole("checkbox", { name: "蓝" }));
    expect(change).not.toHaveBeenCalled();
    fireEvent.keyDown(screen.getByRole("checkbox", { name: "蓝" }), {
      key: "Escape",
    });
    await waitFor(() =>
      expect(change).toHaveBeenCalledExactlyOnceWith(["W", "U"]),
    );
  });
  for (const initial of [[], ["W"], ["W", "U"]]) {
    it(`normalizes a complete multi-select to no filter from ${JSON.stringify(initial)}`, async () => {
      const change = vi.fn();
      render(
        <MultiSelect
          label="颜色"
          options={[
            { value: "W", label: "白" },
            { value: "U", label: "蓝" },
          ]}
          value={initial}
          onChange={change}
        />,
      );
      fireEvent.click(screen.getByRole("button", { name: "颜色" }));
      for (const [value, label] of [
        ["W", "白"],
        ["U", "蓝"],
      ])
        if (!initial.includes(value))
          fireEvent.click(screen.getByRole("checkbox", { name: label }));
      expect(
        screen.getByRole("checkbox", { name: "颜色：全部" }),
      ).toBeChecked();
      expect(change).not.toHaveBeenCalled();
      fireEvent.keyDown(screen.getByRole("checkbox", { name: "蓝" }), {
        key: "Escape",
      });
      await waitFor(() => expect(screen.queryByRole("checkbox")).toBeNull());
      if (initial.length) expect(change).toHaveBeenCalledExactlyOnceWith([]);
      else expect(change).not.toHaveBeenCalled();
    });
  }
  it("does not submit a filter when selecting the only available option", async () => {
    const change = vi.fn();
    render(
      <MultiSelect
        label="语言"
        value={[]}
        options={[{ value: "en", label: "英文" }]}
        onChange={change}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "语言" }));
    fireEvent.click(screen.getByRole("checkbox", { name: "英文" }));
    fireEvent.keyDown(screen.getByRole("checkbox", { name: "英文" }), {
      key: "Escape",
    });
    await waitFor(() => expect(screen.queryByRole("checkbox")).toBeNull());
    expect(change).not.toHaveBeenCalled();
  });
  it("keeps an error open on Escape and closes only through its button", async () => {
    render(
      <FeedbackProvider>
        <div>原页面</div>
      </FeedbackProvider>,
    );
    validation_error("测试错误");
    const dialog = await screen.findByRole("dialog");
    expect(dialog).toHaveTextContent("测试错误");
    fireEvent.keyDown(dialog, { key: "Escape" });
    expect(screen.getByRole("dialog")).toBeVisible();
    fireEvent.click(screen.getByRole("button", { name: "关闭" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  });
});
