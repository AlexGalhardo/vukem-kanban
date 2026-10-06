//go:build race

package web

// raceEnabled lets timing budgets skip under -race, which slows code ~10x.
const raceEnabled = true
