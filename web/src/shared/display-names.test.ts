import { describe, expect, it } from "vitest";
import { language_name } from "./display-names";

describe("language_name", () => {
  it("resolves ordinary and special language names through the same lookup", () => {
    expect(language_name("en")).toBe("英文");
    expect(language_name("zhs")).toBe("简中");
    expect(language_name("sa")).toBe("梵文");
    expect(language_name("ph")).toBe("非瑞");
  });

  it("keeps unrecognized API and historical language codes visible", () => {
    expect(language_name("unknown-language")).toBe("unknown-language");
    expect(language_name("constructor")).toBe("constructor");
    expect(language_name("__proto__")).toBe("__proto__");
  });
});
