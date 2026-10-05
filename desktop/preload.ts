import { contextBridge } from "electron";

contextBridge.exposeInMainWorld("kanbanDesktop", {
	platform: process.platform,
	version: process.versions.electron,
});
