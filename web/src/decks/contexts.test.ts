import { expect, it, vi } from "vitest";
import { EditingSession } from "./contexts";
import { query_client } from "../api/query";

function deferred() {
  let resolve!: () => void;
  const promise = new Promise<void>((done) => {
    resolve = done;
  });
  return { promise, resolve };
}

it("retains idle editing contexts until explicit session cleanup without retaining ordinary queries", async () => {
  vi.useFakeTimers();
  try {
    const session = new EditingSession();
    session.activate();
    const printing = session.key("1", "printing", "card"),
      oracle = session.key("1", "oracle", "oracle"),
      ordinary = ["context_gc_control"];
    query_client.setQueryData(printing, { card: "saved" });
    query_client.setQueryData(oracle, { card: "saved" });
    query_client.setQueryData(ordinary, { card: "ordinary" });
    await vi.advanceTimersByTimeAsync(6 * 60_000);
    expect(query_client.getQueryData(printing)).toEqual({ card: "saved" });
    expect(query_client.getQueryData(oracle)).toEqual({ card: "saved" });
    expect(query_client.getQueryData(ordinary)).toBeUndefined();
    session.close();
    query_client.removeQueries({ queryKey: session.prefix("1") });
    expect(query_client.getQueryData(printing)).toBeUndefined();
    expect(query_client.getQueryData(oracle)).toBeUndefined();
  } finally {
    query_client.clear();
    vi.useRealTimers();
  }
});

it("allows concurrent additions but excludes duplicate additions, saves and imports", async () => {
  const session = new EditingSession();
  session.activate();
  const first = deferred(),
    second = deferred(),
    blocked = vi.fn();
  const a = session.run_task("add", "first", () => first.promise);
  const b = session.run_task("add", "second", () => second.promise);
  await session.run_task("add", "first", blocked);
  await session.run_task("save", "save", blocked);
  await session.run_task("parse", "parse", blocked);
  expect(blocked).not.toHaveBeenCalled();
  first.resolve();
  await a;
  expect(session.has_tasks).toBe(true);
  second.resolve();
  await b;
  const saved = deferred();
  const saving = session.run_task("save", "save", () => saved.promise);
  await session.run_task("add", "third", blocked);
  await session.run_task("parse", "parse", blocked);
  expect(blocked).not.toHaveBeenCalled();
  saved.resolve();
  await saving;
  expect(session.has_tasks).toBe(false);
});

it("releases a failed addition so it can be retried and then saved", async () => {
  const session = new EditingSession();
  session.activate();
  await expect(
    session.run_task("add", "card", async () => {
      throw new Error("failed");
    }),
  ).rejects.toThrow("failed");
  expect(session.has_tasks).toBe(false);
  expect(await session.run_task("add", "card", async () => "added")).toBe(
    "added",
  );
  expect(await session.run_task("save", "save", async () => "saved")).toBe(
    "saved",
  );
});
