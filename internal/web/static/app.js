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

	// Server messages arrive through the HX-Trigger header ({"toast": {...}}).
	document.body.addEventListener("toast", (e) => toast(e.detail.message, e.detail.level));
	// Errors without a server toast (e.g. a 5xx from a proxy) still surface.
	document.body.addEventListener("htmx:response:error", (e) => {
		const res = e.detail.ctx && e.detail.ctx.response;
		if (!res || res.status === 422 || res.headers.get("HX-Trigger")) return;
		toast(`Erro ${res.status}`, "error");
	});
	document.body.addEventListener("htmx:error", (e) => {
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
		htmx.ajax("POST", `/cards/${select.dataset.moveCard}/move`, {
			target: "#columns",
			swap: "outerHTML",
			values: { column_id: select.value, position: "end" },
		}).then(() => toast("Status atualizado.", "success"));
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

	function toggleSidebar(force) {
		const nav = $("#sidebar");
		if (!nav) return;
		const show = force ?? nav.classList.contains("hidden");
		nav.classList.toggle("hidden", !show);
		nav.classList.toggle("flex", show);
	}

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

	function dropIndex(zone, y) {
		const cards = $$("li.card", zone).filter((c) => c !== dragged);
		const i = cards.findIndex((c) => {
			const r = c.getBoundingClientRect();
			return y < r.top + r.height / 2;
		});
		return { index: i === -1 ? cards.length : i, before: i === -1 ? null : cards[i] };
	}

	document.addEventListener("dragover", (e) => {
		const zone = e.target.closest && e.target.closest("[data-dropzone]");
		if (!zone || !dragged) return;
		e.preventDefault();
		e.dataTransfer.dropEffect = "move";
		$$(".drop-target").forEach((z) => z !== zone && z.classList.remove("drop-target"));
		zone.classList.add("drop-target");
		const { before } = dropIndex(zone, e.clientY);
		zone.insertBefore(marker, before);
	});

	document.addEventListener("drop", (e) => {
		const zone = e.target.closest && e.target.closest("[data-dropzone]");
		if (!zone || !dragged) return;
		e.preventDefault();
		const { index, before } = dropIndex(zone, e.clientY);
		const id = dragged.dataset.cardId;
		// Optimistic move so the UI responds instantly; the server response
		// re-renders the board (counts, WIP limits) right after.
		zone.insertBefore(dragged, before);
		marker.remove();
		htmx.ajax("POST", `/cards/${id}/move`, {
			target: "#columns",
			swap: "outerHTML",
			values: { column_id: zone.dataset.columnId, position: index },
		});
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

	window.kanban = { toast };
})();
