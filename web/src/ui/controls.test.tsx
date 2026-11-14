import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { MultiSelect } from "./controls";
import { FeedbackProvider, validation_error } from "./feedback";
afterEach(cleanup);
describe("shared interactions", () => {
  it("applies multi-select changes once when the popover closes", async () => {
    const change = vi.fn();
    render(
      <MultiSelect
        label="颜色"
        options={[
          { value: "W", label: "白" },
          { value: "U", label: "蓝" },
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
