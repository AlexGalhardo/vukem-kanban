// Runs synchronously in <head> so the theme class is set before first paint.
(function () {
	var stored = null;
	try {
		stored = localStorage.getItem("theme");
	} catch (e) {
		// Storage can be blocked; fall back to the system preference.
	}
	var dark = stored ? stored === "dark" : window.matchMedia("(prefers-color-scheme: dark)").matches;
	document.documentElement.classList.toggle("dark", dark);
})();
