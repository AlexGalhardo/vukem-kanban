//go:build race

package tests

// raceEnabled lets timing budgets skip under -race, which slows code ~10x.
const raceEnabled = true
