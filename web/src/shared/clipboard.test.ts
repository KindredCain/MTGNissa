import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { write_clipboard } from "./clipboard";

const original_command = Object.getOwnPropertyDescriptor(
  document,
  "execCommand",
);
const command = vi.fn();

beforeEach(() => {
  vi.stubGlobal("navigator", { clipboard: undefined });
  Object.defineProperty(document, "execCommand", {
    configurable: true,
    value: command,
  });
});

afterEach(() => {
  vi.unstubAllGlobals();
  command.mockReset();
  if (original_command)
    Object.defineProperty(document, "execCommand", original_command);
  else Reflect.deleteProperty(document, "execCommand");
  document.body.replaceChildren();
});

it("uses the native clipboard when available", async () => {
  const writeText = vi.fn().mockResolvedValue(undefined);
  vi.stubGlobal("navigator", { clipboard: { writeText } });
  await write_clipboard("Deck\n2 Test Card");
  expect(writeText).toHaveBeenCalledWith("Deck\n2 Test Card");
  expect(command).not.toHaveBeenCalled();
});

it("keeps a rejected native clipboard write as a failure", async () => {
  vi.stubGlobal("navigator", {
    clipboard: { writeText: vi.fn().mockRejectedValue(new Error("Denied")) },
  });
  await expect(write_clipboard("Deck")).rejects.toThrow("Denied");
  expect(command).not.toHaveBeenCalled();
});

it.each([true, false, "throw"])(
  "cleans up the HTTP copy field and restores focus and selection after %s",
  async (result) => {
    const button = document.createElement("button");
    const paragraph = document.createElement("p");
    paragraph.textContent = "Selected text";
    document.body.append(button, paragraph);
    button.focus();
    const range = document.createRange();
    range.selectNodeContents(paragraph);
    document.getSelection()!.removeAllRanges();
    document.getSelection()!.addRange(range);
    expect(document.getSelection()!.toString()).toBe("Selected text");
    command.mockImplementation((name: string) => {
      expect(name).toBe("copy");
      const field = document.activeElement as HTMLTextAreaElement;
      expect(field.value).toBe("Deck\n2 Test Card");
      expect(field.selectionStart).toBe(0);
      expect(field.selectionEnd).toBe(field.value.length);
      if (result === "throw") throw new Error("Copy unavailable");
      return result;
    });
    const copy = write_clipboard("Deck\n2 Test Card");
    if (result === true) await copy;
    else await expect(copy).rejects.toThrow();
    expect(document.querySelector("textarea")).toBeNull();
    expect(document.activeElement).toBe(button);
    expect(document.getSelection()!.toString()).toBe("Selected text");
  },
);
