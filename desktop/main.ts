import { type ChildProcess, spawn } from "node:child_process";
import { createWriteStream, existsSync, mkdirSync, readFileSync, renameSync, statSync, writeFileSync } from "node:fs";
import path from "node:path";
import { createInterface } from "node:readline";
import {
	app,
	BrowserWindow,
	dialog,
	Menu,
	type MenuItemConstructorOptions,
	type Rectangle,
	screen,
	shell,
} from "electron";

const REPO_URL = "https://github.com/AlexGalhardo/trello-golang-htmx";
const STARTUP_TIMEOUT_MS = 15_000;
const LOG_MAX_BYTES = 5 * 1024 * 1024;

interface WindowState {
	bounds: Rectangle;
	maximized: boolean;
}

let sidecar: ChildProcess | undefined;
let mainWindow: BrowserWindow | undefined;
let serverURL = "";
let quitting = false;

// Must run before "ready" and before the single-instance lock, which is keyed
// by the userData directory (tests rely on this for isolation).
const dataDirOverride = process.env.KANBAN_DATA_DIR;
if (dataDirOverride) app.setPath("userData", path.resolve(dataDirOverride));

globalThis.__kanbanSecondInstances = 0;

export function sidecarPath(): string {
	const exe = process.platform === "win32" ? "kanban.exe" : "kanban";
	if (app.isPackaged) return path.join(process.resourcesPath, "bin", exe);
	return path.join(app.getAppPath(), "bin", exe);
}

function openLog(): NodeJS.WritableStream {
	const dir = path.join(app.getPath("userData"), "logs");
	mkdirSync(dir, { recursive: true });
	const file = path.join(dir, "kanban.log");
	if (existsSync(file) && statSync(file).size > LOG_MAX_BYTES) renameSync(file, `${file}.1`);
	return createWriteStream(file, { flags: "a" });
}

function fatal(message: string): void {
	dialog.showErrorBox("Kanban", message);
	quitting = true;
	stopSidecar();
	app.exit(1);
}

function startSidecar(): Promise<string> {
	const userData = app.getPath("userData");
	mkdirSync(userData, { recursive: true });
	const log = openLog();
	const child = spawn(
		sidecarPath(),
		["--addr", "127.0.0.1:0", "--db", path.join(userData, "kanban.db"), "--log-level", "info"],
		{ stdio: ["ignore", "pipe", "pipe"], windowsHide: true },
	);
	sidecar = child;
	globalThis.__kanbanSidecarPid = child.pid;
	child.stderr?.pipe(log);

	return new Promise((resolve, reject) => {
		let ready = false;
		const timer = setTimeout(() => {
			if (!ready) reject(new Error(`o servidor não respondeu em ${STARTUP_TIMEOUT_MS / 1000}s`));
		}, STARTUP_TIMEOUT_MS);

		if (!child.stdout) {
			reject(new Error("stdout do servidor indisponível"));
			return;
		}
		const lines = createInterface({ input: child.stdout });
		lines.on("line", (line) => {
			const match = /^LISTENING (\S+)$/.exec(line.trim());
			if (!ready && match) {
				ready = true;
				clearTimeout(timer);
				resolve(`http://${match[1]}`);
				return;
			}
			log.write(`${line}\n`);
		});

		child.once("error", (err) => {
			clearTimeout(timer);
			reject(err);
		});
		child.once("exit", (code, signal) => {
			clearTimeout(timer);
			sidecar = undefined;
			if (quitting) return;
			const reason = `o servidor encerrou inesperadamente (código ${code ?? signal ?? "?"})`;
			if (!ready) reject(new Error(reason));
			else fatal(`O ${reason.slice(2)}. Veja os logs em ${path.join(userData, "logs")}.`);
		});
	});
}

function stopSidecar(): void {
	if (sidecar && sidecar.exitCode === null) sidecar.kill();
	sidecar = undefined;
}

function stateFile(): string {
	return path.join(app.getPath("userData"), "window-state.json");
}

function loadWindowState(): WindowState | undefined {
	try {
		const raw: unknown = JSON.parse(readFileSync(stateFile(), "utf8"));
		if (!isWindowState(raw)) return undefined;
		// Discard positions on monitors that are no longer connected.
		const b = raw.bounds;
		const visible = screen.getAllDisplays().some(({ workArea: w }) =>
			b.x < w.x + w.width && b.x + b.width > w.x && b.y < w.y + w.height && b.y + b.height > w.y,
		);
		return visible ? raw : undefined;
	} catch {
		return undefined;
	}
}

function isWindowState(v: unknown): v is WindowState {
	if (typeof v !== "object" || v === null) return false;
	const { bounds, maximized } = v as Record<string, unknown>;
	if (typeof maximized !== "boolean" || typeof bounds !== "object" || bounds === null) return false;
	const b = bounds as Record<string, unknown>;
	return ["x", "y", "width", "height"].every((k) => typeof b[k] === "number" && Number.isFinite(b[k]));
}

function saveWindowState(win: BrowserWindow): void {
	const state: WindowState = { bounds: win.getNormalBounds(), maximized: win.isMaximized() };
	try {
		writeFileSync(stateFile(), JSON.stringify(state));
	} catch {
		// Losing the window layout is not worth interrupting shutdown.
	}
}

function isSameOrigin(url: string): boolean {
	try {
		return new URL(url).origin === new URL(serverURL).origin;
	} catch {
		return false;
	}
}

function openExternalSafe(url: string): void {
	try {
		const { protocol } = new URL(url);
		if (protocol === "https:" || protocol === "http:" || protocol === "mailto:") void shell.openExternal(url);
	} catch {
		// Malformed URL: ignore.
	}
}

function createWindow(): BrowserWindow {
	const state = loadWindowState();
	const win = new BrowserWindow({
		width: state?.bounds.width ?? 1280,
		height: state?.bounds.height ?? 800,
		x: state?.bounds.x,
		y: state?.bounds.y,
		minWidth: 800,
		minHeight: 500,
		title: "Kanban",
		show: false,
		backgroundColor: "#0f0f10",
		webPreferences: {
			// Bun inlines __dirname as the source dir, so resolve from the app root.
			preload: path.join(app.getAppPath(), "desktop", "dist", "preload.js"),
			contextIsolation: true,
			nodeIntegration: false,
			sandbox: true,
		},
	});
	if (state?.maximized) win.maximize();
	win.once("ready-to-show", () => win.show());
	win.on("close", () => saveWindowState(win));
	win.on("closed", () => {
		mainWindow = undefined;
	});

	win.webContents.on("will-navigate", (event, url) => {
		if (isSameOrigin(url)) return;
		event.preventDefault();
		openExternalSafe(url);
	});
	win.webContents.setWindowOpenHandler(({ url }) => {
		openExternalSafe(url);
		return { action: "deny" };
	});

	void win.loadURL(serverURL);
	return win;
}

async function backup(): Promise<void> {
	try {
		// Node's fetch sends no Origin/Sec-Fetch-Site, so the server's CSRF
		// guard treats it as a same-origin, non-browser request.
		const res = await fetch(`${serverURL}/backup`, { method: "POST" });
		const body = (await res.text()).trim();
		if (!res.ok) throw new Error(body || `HTTP ${res.status}`);
		const opts = {
			type: "info" as const,
			title: "Backup",
			message: "Backup concluído",
			detail: body,
			buttons: ["OK", "Abrir pasta"],
			defaultId: 0,
			cancelId: 0,
		};
		const { response } = mainWindow ? await dialog.showMessageBox(mainWindow, opts) : await dialog.showMessageBox(opts);
		if (response === 1) shell.showItemInFolder(body);
	} catch (err) {
		dialog.showErrorBox("Backup", `Falha ao fazer backup: ${err instanceof Error ? err.message : String(err)}`);
	}
}

async function about(): Promise<void> {
	const opts = {
		type: "info" as const,
		title: "Sobre",
		message: `Kanban ${app.getVersion()}`,
		detail: `Electron ${process.versions.electron}\n${REPO_URL}`,
		buttons: ["OK", "Abrir no GitHub"],
		defaultId: 0,
		cancelId: 0,
	};
	const { response } = mainWindow ? await dialog.showMessageBox(mainWindow, opts) : await dialog.showMessageBox(opts);
	if (response === 1) void shell.openExternal(REPO_URL);
}

function buildMenu(): Menu {
	const view: MenuItemConstructorOptions[] = [{ role: "reload", label: "Recarregar" }];
	if (!app.isPackaged) view.push({ role: "toggleDevTools", label: "Ferramentas do desenvolvedor" });
	view.push(
		{ type: "separator" },
		{ role: "resetZoom", label: "Tamanho real" },
		{ role: "zoomIn", label: "Aumentar zoom" },
		{ role: "zoomOut", label: "Diminuir zoom" },
		{ type: "separator" },
		{ role: "togglefullscreen", label: "Tela cheia" },
	);
	return Menu.buildFromTemplate([
		{
			label: "Arquivo",
			submenu: [
				{ label: "Fazer backup", click: () => void backup() },
				{ label: "Abrir pasta de dados", click: () => void shell.openPath(app.getPath("userData")) },
				{ type: "separator" },
				{ role: "quit", label: "Sair" },
			],
		},
		{
			label: "Editar",
			submenu: [
				{ role: "undo", label: "Desfazer" },
				{ role: "redo", label: "Refazer" },
				{ type: "separator" },
				{ role: "cut", label: "Recortar" },
				{ role: "copy", label: "Copiar" },
				{ role: "paste", label: "Colar" },
				{ role: "selectAll", label: "Selecionar tudo" },
			],
		},
		{ label: "Exibir", submenu: view },
		{ label: "Ajuda", submenu: [{ label: "Sobre", click: () => void about() }] },
	]);
}

if (!app.requestSingleInstanceLock()) {
	app.quit();
} else {
	app.on("second-instance", () => {
		globalThis.__kanbanSecondInstances = (globalThis.__kanbanSecondInstances ?? 0) + 1;
		if (!mainWindow) return;
		if (mainWindow.isMinimized()) mainWindow.restore();
		mainWindow.show();
		mainWindow.focus();
	});

	app.on("before-quit", () => {
		quitting = true;
		stopSidecar();
	});
	app.on("will-quit", stopSidecar);
	process.on("exit", stopSidecar);

	app.on("window-all-closed", () => app.quit());

	app.whenReady().then(async () => {
		Menu.setApplicationMenu(buildMenu());
		try {
			serverURL = await startSidecar();
		} catch (err) {
			fatal(`Não foi possível iniciar o servidor: ${err instanceof Error ? err.message : String(err)}`);
			return;
		}
		mainWindow = createWindow();
		app.on("activate", () => {
			if (!mainWindow) mainWindow = createWindow();
		});
	});
}
