import { execFileSync, spawnSync } from "node:child_process";
import { cpSync, mkdirSync, rmSync } from "node:fs";
import { dirname, join, resolve, sep } from "node:path";
import { fileURLToPath } from "node:url";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const web = join(root, "web");
const source = join(web, "dist");
const destination = join(root, "internal", "ui", "dist");

function run(command, args, cwd = root) {
  const options = { cwd, stdio: "inherit" };
  // Windows Corepack uses a .cmd shim; its arguments here are fixed script names.
  const result =
    process.platform === "win32" && command === "corepack"
      ? spawnSync(`corepack ${args.join(" ")}`, { ...options, shell: true })
      : spawnSync(command, args, options);
  if (result.error) throw result.error;
  if (result.status !== 0) {
    throw new Error(
      `${command} ${args.join(" ")} failed (${result.status ?? result.signal})`,
    );
  }
}

try {
  const goos = execFileSync("go", ["env", "GOOS"], {
    cwd: root,
    encoding: "utf8",
  }).trim();

  run("corepack", ["pnpm", "typecheck"], web);
  run("corepack", ["pnpm", "check:contract"], web);
  run("corepack", ["pnpm", "build"], web);

  // Only replace the generated directory owned by this build, never source files.
  if (!destination.startsWith(root + sep)) {
    throw new Error(
      "Frontend output directory must stay inside the repository",
    );
  }
  rmSync(destination, { recursive: true, force: true });
  cpSync(source, destination, { recursive: true });

  const output = join(
    root,
    "bin",
    goos === "windows" ? "mtgnissa.exe" : "mtgnissa",
  );
  mkdirSync(dirname(output), { recursive: true });
  run("go", [
    "build",
    "-tags",
    "embed_ui",
    "-trimpath",
    "-o",
    output,
    "./cmd/server",
  ]);
  console.log(`Built ${output}`);
} catch (error) {
  console.error(error.message);
  process.exitCode = 1;
}
