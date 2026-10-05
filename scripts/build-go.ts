// Builds the Go binary with the version from VERSION injected via -ldflags.
// Usage: bun scripts/build-go.ts [GOOS/GOARCH ...]  (default: host platform)
import { readFileSync } from "node:fs";

const version = readFileSync("VERSION", "utf8").trim();
const targets = process.argv.slice(2);

export function buildGo(goos?: string, goarch?: string, out?: string): string {
	const os = goos ?? process.platform.replace("win32", "windows");
	const exe = os === "windows" ? ".exe" : "";
	const output = out ?? (goos ? `dist/go/kanban-${version}-${goos}-${goarch}${exe}` : `bin/kanban${exe}`);
	const env: Record<string, string> = { ...process.env, CGO_ENABLED: "0" } as Record<string, string>;
	if (goos && goarch) {
		env.GOOS = goos;
		env.GOARCH = goarch;
	}
	const proc = Bun.spawnSync(
		["go", "build", "-trimpath", "-ldflags", `-s -w -X main.version=${version}`, "-o", output, "./cmd/kanban"],
		{ env, stdout: "inherit", stderr: "inherit" },
	);
	if (proc.exitCode !== 0) throw new Error(`go build failed for ${goos ?? "host"}/${goarch ?? ""}`);
	console.log(`built ${output}`);
	return output;
}

if (import.meta.main) {
	if (targets.length === 0) buildGo();
	for (const t of targets) {
		const [goos, goarch] = t.split("/");
		buildGo(goos, goarch);
	}
}
