// Client behaviour that HTMX attributes cannot express: dialogs, toasts,
// drag-and-drop, keyboard shortcuts and the command palette. No inline
// handlers anywhere, so the CSP can keep script-src 'self'.
(function () {
	"use strict";

	const $ = (sel, root = document) => root.querySelector(sel);
	const $$ = (sel, root = document) => Array.from(root.querySelectorAll(sel));

	// ---------- Toasts ----------
	function toast(message, level = "info") {
		const box = $("#toasts");
		if (!box || !message) return;
		const el = document.createElement("div");
		el.className = "toast";
		el.dataset.level = level;
		const dot = document.createElement("span");
		dot.className = "mt-1 size-2 shrink-0 rounded-full " +
			(level === "error" ? "bg-red-500" : level === "success" ? "bg-green-500" : "bg-accent");
		const text = document.createElement("span");
		text.className = "break-words";
		text.textContent = message;
		el.append(dot, text);
		box.append(el);
		setTimeout(() => el.remove(), level === "error" ? 6000 : 3500);
	}

	// Server messages arrive in the X-Toast header (ASCII-escaped JSON). It is
	// read on htmx:after:request, which fires before the swap, so the toast
	// shows even when the swap removes the element that made the request.
	document.addEventListener("htmx:after:request", (e) => {
		const res = e.detail.ctx && e.detail.ctx.response;
		if (!res) return;
		const header = res.headers.get("X-Toast");
		if (header) {
			try {
				const t = JSON.parse(header);
				toast(t.message, t.level);
			} catch (err) {
				toast("Resposta inesperada do servidor.", "error");
			}
		} else if (res.status >= 400 && res.status !== 422) {
			toast(`Erro ${res.status}`, "error");
		}
	});
	document.addEventListener("htmx:error", (e) => {
		if (!(e.detail.ctx && e.detail.ctx.response)) toast("Sem conexão com o servidor.", "error");
	});

	// ---------- Dialogs loaded into #modal ----------
	function openModalDialog() {
		const dialog = $("#modal dialog");
		if (dialog && !dialog.open) {
			dialog.addEventListener("close", () => {
				if (dialog.isConnected) $("#modal").replaceChildren();
			}, { once: true });
			dialog.showModal();
			const focus = $("[autofocus]", dialog);
			if (focus) focus.focus();
		}
	}

	document.body.addEventListener("htmx:after:settle", () => {
		openModalDialog();
		restoreCardFocus();
		closeMenus();
	});

	// Ctrl/Cmd+Enter submits the form of a focused field (Linear behaviour).
	document.addEventListener("keydown", (e) => {
		if (e.key === "Enter" && (e.ctrlKey || e.metaKey) && e.target.form) {
			e.preventDefault();
			e.target.form.requestSubmit();
		}
	});

	// Changing the status select inside the card dialog moves the card.
	document.addEventListener("change", (e) => {
		const select = e.target.closest("[data-move-card]");
		if (!select) return;
		moveCard(select.dataset.moveCard, select.value, 0);
	});

	// ---------- Forms ----------
	document.body.addEventListener("htmx:after:request", (e) => {
		const form = e.target;
		const res = e.detail.ctx && e.detail.ctx.response;
		if (res && res.status < 400 && form.matches && form.matches("form[data-reset-on-success]")) {
			form.reset();
			const input = $("input", form);
			if (input) input.focus();
		}
	});

	// Inline title editor: Enter saves, Esc cancels.
	document.addEventListener("keydown", (e) => {
		const field = e.target.closest && e.target.closest("[data-submit-on-enter]");
		if (!field) return;
		if (e.key === "Enter" && !e.shiftKey) {
			e.preventDefault();
			field.form.requestSubmit();
		} else if (e.key === "Escape") {
			e.preventDefault();
			const cancel = $("[data-cancel]", field.form);
			if (cancel) cancel.click();
		}
	});

	// Keep keyboard focus on a card after its title editor is swapped out.
	let focusCardId = null;
	document.body.addEventListener("htmx:before:request", (e) => {
		const card = e.target.closest && e.target.closest("li.card");
		if (card && card.contains(document.activeElement)) focusCardId = card.id.replace("card-", "");
	});
	function restoreCardFocus() {
		if (!focusCardId) return;
		const btn = $(`#card-${focusCardId} .card-open`);
		focusCardId = null;
		if (btn) btn.focus();
	}

	function closeMenus(except) {
		$$("details.col-menu[open]").forEach((d) => {
			if (d !== except) d.open = false;
		});
	}
	document.addEventListener("click", (e) => {
		const menu = e.target.closest("details.col-menu");
		closeMenus(menu);
	});

	// ---------- Theme, sidebar, misc actions ----------
	function toggleTheme() {
		const dark = !document.documentElement.classList.contains("dark");
		document.documentElement.classList.toggle("dark", dark);
		try {
			localStorage.setItem("theme", dark ? "dark" : "light");
		} catch (err) {
			// Ignore blocked storage; the toggle still applies to this page.
		}
	}

	// On narrow screens the sidebar is an overlay with a backdrop; Esc, a
	// backdrop click or navigating closes it.
	function toggleSidebar(force) {
		const nav = $("#sidebar");
		if (!nav) return;
		const show = force ?? nav.classList.contains("hidden");
		nav.classList.toggle("hidden", !show);
		nav.classList.toggle("flex", show);
		const backdrop = $("#sidebar-backdrop");
		if (backdrop) backdrop.classList.toggle("hidden", !show);
	}
	const sidebarOverlayOpen = () => {
		const backdrop = $("#sidebar-backdrop");
		return backdrop && !backdrop.classList.contains("hidden");
	};
	document.addEventListener("keydown", (e) => {
		if (e.key === "Escape" && sidebarOverlayOpen()) toggleSidebar(false);
	});
	document.addEventListener("htmx:before:request", (e) => {
		if (sidebarOverlayOpen() && e.target.closest && e.target.closest("#sidebar")) toggleSidebar(false);
	});

	function focusNewCard(columnId) {
		const input = columnId ? $(`#new-card-${columnId}`) : $("[data-new-card]");
		if (input) {
			input.scrollIntoView({ block: "nearest", inline: "nearest" });
			input.focus();
		}
	}

	document.addEventListener("click", (e) => {
		const el = e.target.closest("[data-action], [data-focus-new-card]");
		if (!el) return;
		if (el.dataset.focusNewCard) return focusNewCard(el.dataset.focusNewCard);
		switch (el.dataset.action) {
			case "theme": return toggleTheme();
			case "shortcuts": return $("#shortcuts-dialog").showModal();
			case "palette": return openPalette();
			case "sidebar": return toggleSidebar();
			case "sidebar-close": return toggleSidebar(false);
		}
	});

	// ---------- Drag and drop ----------
	let dragged = null;
	const marker = document.createElement("li");
	marker.className = "drop-marker";
	marker.setAttribute("aria-hidden", "true");

	document.addEventListener("dragstart", (e) => {
		const card = e.target.closest && e.target.closest("li.card[draggable]");
		if (!card) return;
		dragged = card;
		e.dataTransfer.effectAllowed = "move";
		e.dataTransfer.setData("text/plain", card.dataset.cardId);
		requestAnimationFrame(() => card.classList.add("dragging"));
	});

	document.addEventListener("dragend", () => {
		if (dragged) dragged.classList.remove("dragging");
		dragged = null;
		marker.remove();
		$$(".drop-target").forEach((z) => z.classList.remove("drop-target"));
	});

	// The drop is anchored on the visible card it lands above (not an index):
	// with a filter on, hidden cards would make an index wrong.
	function dropBefore(zone, y) {
		const cards = $$("li.card", zone).filter((c) => c !== dragged);
		return cards.find((c) => {
			const r = c.getBoundingClientRect();
			return y < r.top + r.height / 2;
		}) || null;
	}

	// moveCard asks the server to move a card; the response replaces only the
	// affected columns out of band. On any failure the board is re-fetched so
	// an optimistic DOM move never lies about what was saved.
	function moveCard(id, columnId, beforeId) {
		const action = `/cards/${id}/move`;
		let status = 0;
		// htmx.ajax resolves without the response, so read it from the event.
		const record = (e) => {
			const ctx = e.detail.ctx;
			if (ctx && ctx.request && ctx.request.action === action && ctx.response) status = ctx.response.status;
		};
		document.addEventListener("htmx:after:request", record);
		return htmx.ajax("POST", action, {
			swap: "none",
			values: { column_id: columnId, before_id: beforeId || 0 },
		}).catch(() => {}).finally(() => {
			document.removeEventListener("htmx:after:request", record);
			if (!status || status >= 400) {
				htmx.ajax("GET", location.href, { target: "#columns", select: "#columns", swap: "outerHTML" }).catch(() => {});
			}
		});
	}

	document.addEventListener("dragover", (e) => {
		const zone = e.target.closest && e.target.closest("[data-dropzone]");
		if (!zone || !dragged) return;
		e.preventDefault();
		e.dataTransfer.dropEffect = "move";
		$$(".drop-target").forEach((z) => z !== zone && z.classList.remove("drop-target"));
		zone.classList.add("drop-target");
		zone.insertBefore(marker, dropBefore(zone, e.clientY));
	});

	document.addEventListener("drop", (e) => {
		const zone = e.target.closest && e.target.closest("[data-dropzone]");
		if (!zone || !dragged) return;
		e.preventDefault();
		const before = dropBefore(zone, e.clientY);
		const id = dragged.dataset.cardId;
		// Optimistic move so the UI responds instantly; the server response
		// re-renders the two columns (counts, WIP limits) right after.
		zone.insertBefore(dragged, before);
		marker.remove();
		moveCard(id, zone.dataset.columnId, before ? before.dataset.cardId : 0);
	});

	// ---------- Command palette ----------
	const palette = () => $("#command-palette");

	function commands() {
		const list = [];
		const add = (label, hint, run) => list.push({ label, hint, run });
		add("Criar novo projeto", "N", () => $("[data-action=new-board]").click());
		if ($("[data-new-card]")) add("Criar novo card", "C", () => focusNewCard());
		if ($("#search")) add("Filtrar cards", "/", () => $("#search").focus());
		if ($("[data-action=labels]")) add("Gerenciar labels", "", () => $("[data-action=labels]").click());
		if ($("[data-action=settings]")) add("Configurações do projeto", "", () => $("[data-action=settings]").click());
		add("Alternar tema claro/escuro", "T", toggleTheme);
		add("Exportar dados (JSON)", "", () => $("[data-action=export]").click());
		add("Importar dados", "", () => $("[data-action=import]").click());
		add("Fazer backup do banco", "", () => $("[data-action=backup]").click());
		add("Atalhos de teclado", "?", () => $("#shortcuts-dialog").showModal());
		$$("[data-board-link]").forEach((a) => {
			add(`Ir para ${a.dataset.boardLink}`, "Projeto", () => a.click());
		});
		return list;
	}

	let items = [];
	let active = 0;

	function renderPalette() {
		const q = $("#command-input").value.trim().toLowerCase();
		const terms = q.split(/\s+/).filter(Boolean);
		items = commands().filter((c) => terms.every((t) => c.label.toLowerCase().includes(t)));
		active = Math.min(active, Math.max(items.length - 1, 0));
		const ul = $("#command-list");
		ul.replaceChildren(...items.map((c, i) => {
			const li = document.createElement("li");
			li.id = `cmd-${i}`;
			li.setAttribute("role", "option");
			li.setAttribute("aria-selected", String(i === active));
			li.className = "flex h-9 cursor-pointer items-center gap-3 rounded-md px-3 text-[13px] " +
				(i === active ? "bg-surface-hover text-fg" : "text-muted");
			const label = document.createElement("span");
			label.className = "flex-1 truncate";
			label.textContent = c.label;
			li.append(label);
			if (c.hint) {
				const hint = document.createElement(c.hint.length === 1 ? "kbd" : "span");
				hint.className = c.hint.length === 1 ? "" : "text-[11px] text-faint";
				hint.textContent = c.hint;
				li.append(hint);
			}
			li.addEventListener("mousemove", () => { if (active !== i) { active = i; renderPalette(); } });
			li.addEventListener("click", () => runCommand(i));
			return li;
		}));
		if (!items.length) {
			const li = document.createElement("li");
			li.className = "px-3 py-6 text-center text-[13px] text-faint";
			li.textContent = "Nenhum comando encontrado.";
			ul.append(li);
		}
		$("#command-input").setAttribute("aria-activedescendant", items.length ? `cmd-${active}` : "");
		const current = $(`#cmd-${active}`);
		if (current) current.scrollIntoView({ block: "nearest" });
	}

	function openPalette() {
		const dlg = palette();
		if (!dlg || dlg.open) return;
		$("#command-input").value = "";
		active = 0;
		renderPalette();
		dlg.showModal();
		$("#command-input").focus();
	}

	function runCommand(i) {
		const cmd = items[i];
		palette().close();
		if (cmd) cmd.run();
	}

	document.addEventListener("input", (e) => {
		if (e.target.id === "command-input") { active = 0; renderPalette(); }
	});
	document.addEventListener("keydown", (e) => {
		if (e.target.id !== "command-input") return;
		if (e.key === "ArrowDown") { e.preventDefault(); active = (active + 1) % Math.max(items.length, 1); renderPalette(); }
		else if (e.key === "ArrowUp") { e.preventDefault(); active = (active - 1 + items.length) % Math.max(items.length, 1); renderPalette(); }
		else if (e.key === "Enter") { e.preventDefault(); runCommand(active); }
	});

	// ---------- Global keyboard shortcuts ----------
	function typing(el) {
		return el && (el.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test(el.tagName));
	}

	function jumpBoard(step) {
		const links = $$("[data-board-link]");
		if (!links.length) return;
		const cur = links.findIndex((a) => a.getAttribute("aria-current") === "page");
		links[(cur + step + links.length) % links.length].click();
	}

	let pendingG = false;
	document.addEventListener("keydown", (e) => {
		if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "k") {
			e.preventDefault();
			const dlg = palette();
			if (dlg && dlg.open) dlg.close(); else openPalette();
			return;
		}
		if (e.ctrlKey || e.metaKey || e.altKey || typing(e.target) || $("dialog[open]")) return;
		if (pendingG) {
			pendingG = false;
			if (e.key === "j") return jumpBoard(1);
			if (e.key === "k") return jumpBoard(-1);
		}
		switch (e.key) {
			case "c": e.preventDefault(); return focusNewCard();
			case "n": e.preventDefault(); return $("[data-action=new-board]").click();
			case "t": return toggleTheme();
			case "/": e.preventDefault(); { const s = $("#search"); if (s) s.focus(); } return;
			case "?": return $("#shortcuts-dialog").showModal();
			case "g": pendingG = true; setTimeout(() => { pendingG = false; }, 800); return;
		}
	});

	// Escape inside the filter field clears and leaves it, like Linear.
	document.addEventListener("keydown", (e) => {
		if (e.key === "Escape" && e.target.id === "search") e.target.blur();
	});

	// ---------- Markdown editor (card description) ----------
	function setMdMode(editor, mode) {
		editor.dataset.mode = mode;
		$$("[data-md-tab]", editor).forEach((t) => t.setAttribute("aria-selected", String(t.dataset.mdTab === mode)));
		if (mode === "write") {
			const ta = $("textarea[data-markdown]", editor);
			if (ta) ta.focus();
		}
	}

	document.addEventListener("click", (e) => {
		const tab = e.target.closest("[data-md-tab]");
		if (tab) setMdMode(tab.closest("[data-md-editor]"), tab.dataset.mdTab);
	});
	document.addEventListener("dblclick", (e) => {
		// Links inside the preview keep working; double-click elsewhere edits.
		const preview = e.target.closest(".md-preview");
		if (preview && !e.target.closest("a")) setMdMode(preview.closest("[data-md-editor]"), "write");
	});
	document.body.addEventListener("htmx:after:settle", () => {
		$$("[data-md-editor]").forEach((ed) => setMdMode(ed, ed.dataset.mode || "write"));
	});

	// replaceRange edits the textarea and notifies listeners (htmx, counters).
	function replaceRange(ta, start, end, text, caretStart, caretEnd) {
		ta.setRangeText(text, start, end, "end");
		if (caretStart !== undefined) ta.setSelectionRange(caretStart, caretEnd ?? caretStart);
		ta.dispatchEvent(new Event("input", { bubbles: true }));
	}

	function wrapSelection(ta, mark) {
		const { selectionStart: s, selectionEnd: e, value } = ta;
		const inner = value.slice(s, e) || "texto";
		replaceRange(ta, s, e, mark + inner + mark, s + mark.length, s + mark.length + inner.length);
	}

	const listRe = /^(\s*)([-*+]|(\d+)([.)]))\s+(\[[ xX]\]\s+)?(.*)$/;

	document.addEventListener("keydown", (e) => {
		const ta = e.target;
		if (!(ta instanceof HTMLTextAreaElement) || !ta.matches("[data-markdown]")) return;
		const mod = e.ctrlKey || e.metaKey;
		if (mod && (e.key === "b" || e.key === "B")) {
			e.preventDefault();
			return wrapSelection(ta, "**");
		}
		if (mod && (e.key === "i" || e.key === "I")) {
			e.preventDefault();
			return wrapSelection(ta, "_");
		}
		const { selectionStart: pos, value } = ta;
		const lineStart = value.lastIndexOf("\n", pos - 1) + 1;
		if (e.key === "Tab") {
			e.preventDefault();
			if (e.shiftKey) {
				const cut = value.slice(lineStart, lineStart + 2) === "  " ? 2 : value[lineStart] === "\t" ? 1 : 0;
				if (cut) replaceRange(ta, lineStart, lineStart + cut, "", Math.max(lineStart, pos - cut));
			} else {
				replaceRange(ta, lineStart, lineStart, "  ", pos + 2);
			}
			return;
		}
		if (e.key !== "Enter" || mod || e.shiftKey || ta.selectionStart !== ta.selectionEnd) return;
		// Enter on a list line continues the list (Linear/GitHub behaviour);
		// Enter on an empty item ends it.
		const m = value.slice(lineStart, pos).match(listRe);
		if (!m) return;
		e.preventDefault();
		const [, indent, bullet, num, sep, task, text] = m;
		if (!text.trim()) {
			// Leave a blank line so what follows is a new paragraph.
			replaceRange(ta, lineStart, pos, "\n", lineStart + 1);
			return;
		}
		const marker = num ? `${Number(num) + 1}${sep}` : bullet;
		const insert = `\n${indent}${marker} ${task ? "[ ] " : ""}`;
		replaceRange(ta, pos, pos, insert, pos + insert.length);
	});

	window.kanban = { toast };
})();
