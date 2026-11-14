import fs from "node:fs";
import path from "node:path";
import { pathToFileURL } from "node:url";

const methods = new Set([
  "get",
  "post",
  "put",
  "patch",
  "delete",
  "head",
  "options",
  "trace",
]);
const maintenance = new Set([
  "/api/v1/card_data/load",
  "/api/v1/card_data/load/{task_id}",
]);

// The approved OpenAPI is the source, never an output of this check.
export function checkContract() {
  const spec = JSON.parse(
    fs.readFileSync(
      new URL("../../docs/openapi.json", import.meta.url),
      "utf8",
    ),
  );
  if (spec.openapi !== "3.1.0")
    throw Error("Expected an OpenAPI 3.1.0 contract.");
  const router = fs.readFileSync(
    new URL("../../internal/httpapi/router.go", import.meta.url),
    "utf8",
  );
  const registered = new Set(
    [
      ...router.matchAll(
        /api\.(GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS|TRACE)\("([^"]+)"/g,
      ),
    ]
      .map(([, method, route]) => [
        method.toLowerCase(),
        "/api/v1" + route.replace(/:(\w+)/g, "{$1}"),
      ])
      .filter(([, route]) => !maintenance.has(route))
      .map(([method, route]) => method + " " + route),
  );
  const documented = new Set();
  function resolve(ref) {
    if (!ref.startsWith("#/"))
      throw Error("Expected a local reference: " + ref);
    const value = ref
      .slice(2)
      .split("/")
      .reduce(
        (node, key) => node?.[key.replaceAll("~1", "/").replaceAll("~0", "~")],
        spec,
      );
    if (value === undefined) throw Error("Invalid reference " + ref);
    return value;
  }
  function visit(value) {
    if (!value || typeof value !== "object") return;
    if (value.$ref) resolve(value.$ref);
    Object.values(value).forEach(visit);
  }
  visit(spec);
  for (const [route, item] of Object.entries(spec.paths)) {
    for (const method of Object.keys(item).filter((key) => methods.has(key))) {
      const key = method + " " + route;
      if (!registered.has(key))
        throw Error("Unknown frontend operation " + key);
      documented.add(key);
    }
  }
  for (const key of registered) {
    if (!documented.has(key))
      throw Error("Undocumented frontend operation " + key);
  }
  return spec;
}

if (
  process.argv[1] &&
  pathToFileURL(path.resolve(process.argv[1])).href === import.meta.url
) {
  const spec = checkContract();
  const count = Object.values(spec.paths).reduce(
    (total, item) =>
      total + Object.keys(item).filter((method) => methods.has(method)).length,
    0,
  );
  console.log(
    "Verified " +
      count +
      " frontend operations and all local references; maintenance endpoints are excluded.",
  );
}
