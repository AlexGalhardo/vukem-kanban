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

	// ---------- Focus management ----------
	// htmx swaps often replace the focused element (a label chip, a dialog,
	// a card). A selector for it is remembered before each request and focus
	// is restored after the swap instead of falling back to <body>.
	function selectorFor(el) {
		if (!el || el === document.body || el === document.documentElement) return null;
		if (el.id) return `#${CSS.escape(el.id)}`;
		for (const attr of ["hx-post", "hx-put", "hx-delete", "hx-get"]) {
			const v = el.getAttribute(attr);
			if (v) return `[${attr}="${CSS.escape(v)}"]`;
		}
		const card = el.closest && el.closest("li.card");
		return card ? `#${card.id} .card-open` : null;
	}
	const visible = (el) => el && el.isConnected && el.getClientRects().length > 0;
	// When the focused node is removed, focus falls back to <body> or, inside
	// a modal, to the <dialog> itself.
	const focusLost = () => {
		const el = document.activeElement;
		return !el || el === document.body || el.tagName === "DIALOG";
	};

	// focusSelector lists where focus should go back: the element itself and,
	// inside a card, the card's open button (for swaps that replace it).
	let focusSelector = null;
	document.addEventListener("htmx:before:request", () => {
		const el = document.activeElement;
		const card = el && el.closest && el.closest("li.card");
		focusSelector = [selectorFor(el), card ? `#${card.id} .card-open` : null].filter(Boolean);
	});
	const findFocus = (sels, root = document) => {
		for (const s of sels || []) {
			const el = $(s, root);
			if (visible(el)) return el;
		}
		return null;
	};

	// ---------- Dialogs loaded into #modal ----------
	let openDialog = null;
	let opener = null; // where focus goes back when the dialog goes away

	function focusOpener() {
		const cardId = openDialog && openDialog.dataset.cardId;
		const target = (cardId && $(`#card-${cardId} .card-open`)) || findFocus(opener) || $("#main");
		if (visible(target) || target === $("#main")) target.focus();
	}

	// formSnapshot serializes a guarded form, ignoring fields that act
	// immediately (the status select moves the card on change).
	function formSnapshot(form) {
		const data = new URLSearchParams(new FormData(form));
		data.delete("column_id");
		return data.toString();
	}
	function isDirty(dialog) {
		const form = $("form[data-dirty-guard]", dialog);
		return !!form && form.dataset.snapshot !== undefined && formSnapshot(form) !== form.dataset.snapshot;
	}

	function openModalDialog(previous) {
		const dialog = $("#modal dialog");
		if (!dialog || dialog.open) return null;
		if (!previous) opener = focusSelector;
		dialog.addEventListener("close", () => {
			if (dialog.isConnected) $("#modal").replaceChildren();
			if (openDialog === dialog) {
				focusOpener();
				openDialog = null;
			}
		}, { once: true });
		// Esc and backdrop clicks ask before discarding unsaved edits.
		dialog.addEventListener("cancel", (e) => {
			if (isDirty(dialog) && !window.confirm("Descartar as alterações não salvas?")) e.preventDefault();
		});
		$$("form[method=dialog] button", dialog).forEach((btn) => btn.addEventListener("click", (e) => {
			if (isDirty(dialog) && !window.confirm("Descartar as alterações não salvas?")) e.preventDefault();
		}));
		if (previous && previous.dataset.cardId) dialog.dataset.cardId ||= previous.dataset.cardId;
		openDialog = dialog;
		dialog.showModal();
		$$("form[data-dirty-guard]", dialog).forEach((f) => { f.dataset.snapshot = formSnapshot(f); });
		const focus = $("[autofocus]", dialog);
		if (focus) focus.focus();
		return dialog;
	}

	function afterSwap(sel = focusSelector) {
		const previous = openDialog && !openDialog.isConnected ? openDialog : null;
		const opened = openModalDialog(previous);
		if (opened && previous) {
			// The dialog was re-rendered (validation, labels): keep the place.
			const el = findFocus(sel, opened);
			if (el) el.focus();
		} else if (!opened && previous) {
			// The dialog was removed by the response (saved, deleted).
			openDialog = previous;
			focusOpener();
			openDialog = null;
		} else if (focusLost()) {
			const el = findFocus(sel);
			if (el) el.focus();
		}
		closeMenus();
	}

	// htmx:after:settle is dispatched on the swapped element; when the swap
	// replaced the element that made the request (outerHTML), that node is
	// detached and the event never reaches document. A short timer started on
	// htmx:after:request (still attached) runs the same work in that case.
	let settled = false;
	document.body.addEventListener("htmx:after:settle", () => {
		settled = true;
		afterSwap();
	});
	// The timer always runs once more (a response can settle several swaps,
	// main and out-of-band, in any order) and then forgets the selector.
	// Each timer only handles its own request's selector: a newer request
	// (e.g. a quick Enter right after a dialog opened) must not be cleared.
	document.body.addEventListener("htmx:after:request", () => {
		settled = false;
		const mine = focusSelector;
		setTimeout(() => {
			if (focusLost() || !settled) afterSwap(mine);
			if (focusSelector === mine) focusSelector = null;
		}, 60);
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
	// Quick-add inputs (new card, new column): after a successful create the
	// input is cleared only if it still holds what was sent. Text typed while
	// the request was in flight is kept, also across the #columns swap that
	// replaces the new-column input.
	const sent = new WeakMap();
	let carryOver = null;
	// Values are already encoded when htmx:before:request fires, so clearing
	// here is safe: whatever the user types next is a new title.
	document.body.addEventListener("htmx:before:request", (e) => {
		const form = e.target;
		if (!form.matches || !form.matches("form[data-reset-on-success]")) return;
		const input = $("input", form);
		if (!input) return;
		sent.set(form, input.value);
		input.value = "";
	});
	const restoreSent = (form) => {
		const input = $("input", form);
		if (input && input.value === "" && sent.has(form)) input.value = sent.get(form);
	};
	document.body.addEventListener("htmx:after:request", (e) => {
		const form = e.target;
		if (!form.matches || !form.matches("form[data-reset-on-success]")) return;
		const input = $("input", form);
		const res = e.detail.ctx && e.detail.ctx.response;
		if (!res || res.status >= 400) {
			restoreSent(form);
			return;
		}
		if (input) carryOver = { id: input.id, value: input.value, focused: document.activeElement === input };
	});
	document.body.addEventListener("htmx:error", (e) => {
		if (e.target.matches && e.target.matches("form[data-reset-on-success]")) restoreSent(e.target);
	});
	document.body.addEventListener("htmx:after:settle", () => {
		if (!carryOver) return;
		const input = carryOver.id && document.getElementById(carryOver.id);
		if (input) {
			if (input.value !== carryOver.value) input.value = carryOver.value;
			if (carryOver.focused || focusLost()) input.focus();
		}
		carryOver = null;
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

	function closeMenus(except) {
		$$("details.col-menu[open]").forEach((d) => {
			if (d !== except) d.open = false;
		});
	}
	// Esc closes an open column menu and returns focus to its toggle.
	document.addEventListener("keydown", (e) => {
		if (e.key !== "Escape") return;
		const menu = $("details.col-menu[open]");
		if (!menu) return;
		e.preventDefault();
		menu.open = false;
		$("summary", menu).focus();
	});
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
		if (!backdrop) return;
		backdrop.classList.toggle("hidden", !show);
		// While the overlay is open the page behind it is inert, so Tab stays
		// in the sidebar; closing it returns focus to the toggle. On wide
		// screens the backdrop is display:none and nothing becomes inert.
		const overlay = show && getComputedStyle(backdrop).display !== "none";
		const main = $("#main");
		if (main) main.inert = overlay;
		if (show && !overlay) return;
		if (show) {
			const first = $("a, button", nav);
			if (first) first.focus();
		} else {
			const toggle = $("[data-action=sidebar]");
			if (visible(toggle)) toggle.focus();
		}
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
	function moveCard(id, columnId, beforeId, revert) {
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
				// Undo the optimistic DOM move first: the re-fetch below also
				// fails when the server is unreachable.
				if (revert) revert();
				htmx.ajax("GET", location.href, { target: "#columns", select: "#columns", swap: "outerHTML" }).catch(() => {});
			}
		});
	}

	// moveOptimistic moves card in the DOM now and on the server; on failure
	// the card goes back to where it was.
	function moveOptimistic(card, zone, before) {
		const from = card.parentElement;
		const next = card.nextElementSibling;
		zone.insertBefore(card, before);
		moveCard(card.dataset.cardId, zone.dataset.columnId, before ? before.dataset.cardId : 0, () => {
			if (from && from.isConnected) from.insertBefore(card, next && next.isConnected ? next : null);
		});
	}

	// Alt+Arrow moves the focused card: up/down inside its column, left/right
	// to the end of the neighbouring column (keyboard alternative to DnD).
	document.addEventListener("keydown", (e) => {
		if (!e.altKey || !e.key.startsWith("Arrow")) return;
		const card = document.activeElement && document.activeElement.closest && document.activeElement.closest("li.card");
		if (!card || !card.parentElement.matches("[data-dropzone]")) return;
		e.preventDefault();
		const zone = card.parentElement;
		const siblings = $$("li.card", zone);
		const i = siblings.indexOf(card);
		if (e.key === "ArrowUp" && i > 0) {
			moveOptimistic(card, zone, siblings[i - 1]);
		} else if (e.key === "ArrowDown" && i < siblings.length - 1) {
			moveOptimistic(card, zone, siblings[i + 2] || null);
		} else if (e.key === "ArrowLeft" || e.key === "ArrowRight") {
			const zones = $$("[data-dropzone]");
			const target = zones[zones.indexOf(zone) + (e.key === "ArrowLeft" ? -1 : 1)];
			if (target) moveOptimistic(card, target, null);
		}
		$(".card-open", card).focus();
	});

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
		marker.remove();
		// Optimistic move so the UI responds instantly; the server response
		// re-renders the two columns (counts, WIP limits) right after.
		moveOptimistic(dragged, zone, before);
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
			if (dlg && dlg.open) dlg.close();
			// Never stack the palette on top of another open dialog.
			else if (!$("dialog[open]")) openPalette();
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
	// setMdMode switches the editor pane; it moves focus only when the user
	// asked for it (tab click, double-click), never on background re-syncs.
	function setMdMode(editor, mode, focus = false) {
		editor.dataset.mode = mode;
		$$("[data-md-tab]", editor).forEach((t) => t.setAttribute("aria-selected", String(t.dataset.mdTab === mode)));
		if (focus && mode === "write") {
			const ta = $("textarea[data-markdown]", editor);
			if (ta) ta.focus();
		}
	}

	document.addEventListener("click", (e) => {
		const tab = e.target.closest("[data-md-tab]");
		if (tab) setMdMode(tab.closest("[data-md-editor]"), tab.dataset.mdTab, true);
	});
	document.addEventListener("dblclick", (e) => {
		// Links inside the preview keep working; double-click elsewhere edits.
		const preview = e.target.closest(".md-preview");
		if (preview && !e.target.closest("a")) setMdMode(preview.closest("[data-md-editor]"), "write", true);
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
