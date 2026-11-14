package controllers

import (
	"strings"
	"testing"
)

func TestHelpers(t *testing.T) {
	if prefix("trello golang") != "TRE" || prefix("a1") != "A1" || prefix("!!") != "KAN" || prefix("Ação") != "AÇÃ" || prefix("日本語テスト") != "日本語" {
		t.Fatal("prefix")
	}
	if initial("ágil") != "Á" || initial("") != "?" {
		t.Fatal("initial")
	}
	kinds := []string{}
	for i := range 5 {
		kinds = append(kinds, statusKind(i, 5))
	}
	if strings.Join(kinds, ",") != "backlog,todo,progress,review,done" {
		t.Fatalf("status kinds = %v", kinds)
	}
}
