import { afterEach, expect, it, vi } from "vitest";
import { random_uuid } from "./uuid";

afterEach(() => vi.restoreAllMocks());

it.each([
  [0x00, "00000000-0000-4000-8000-000000000000"],
  [0xff, "ffffffff-ffff-4fff-bfff-ffffffffffff"],
])(
  "sets the UUID version and variant with random bytes %i",
  (value, expected) => {
    vi.spyOn(crypto, "getRandomValues").mockImplementation((bytes) => {
      (bytes as Uint8Array).fill(value as number);
      return bytes;
    });
    expect(random_uuid()).toBe(expected);
  },
);

it("uses fresh random values for each identifier without calling randomUUID", () => {
  const native = vi.spyOn(crypto, "randomUUID").mockImplementation(() => {
    throw new Error("Unavailable on HTTP");
  });
  const random = vi.spyOn(crypto, "getRandomValues");
  const first = random_uuid();
  const second = random_uuid();
  expect(first).toMatch(
    /^[\da-f]{8}-[\da-f]{4}-4[\da-f]{3}-[89ab][\da-f]{3}-[\da-f]{12}$/,
  );
  expect(second).not.toBe(first);
  expect(random).toHaveBeenCalledTimes(2);
  expect(native).not.toHaveBeenCalled();
});

it("propagates random source failures instead of producing a weak identifier", () => {
  vi.spyOn(crypto, "getRandomValues").mockImplementation(() => {
    throw new Error("Random source unavailable");
  });
  expect(random_uuid).toThrow("Random source unavailable");
});
