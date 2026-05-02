package artifact_test

import (
	"strings"
	"testing"

	"github.com/gopherchan2006/go-triangle-detector/internal/artifact"
)

func TestNewNames_Paths(t *testing.T) {
	names := artifact.NewNames("tmp/out", "BTCUSDT_2026-01-01")
	if !strings.Contains(names.GroupDir, "BTCUSDT") {
		t.Errorf("GroupDir should contain symbol, got %q", names.GroupDir)
	}
	if !strings.HasSuffix(names.PNG, ".png") {
		t.Errorf("PNG should end with .png, got %q", names.PNG)
	}
	if !strings.HasSuffix(names.DebugTxt, ".txt") {
		t.Errorf("DebugTxt should end with .txt, got %q", names.DebugTxt)
	}
	if !strings.HasSuffix(names.HTMLTmp, ".html") {
		t.Errorf("HTMLTmp should end with .html, got %q", names.HTMLTmp)
	}
}
