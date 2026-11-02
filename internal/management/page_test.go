package management

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEmbeddedPageUsesDashboardSessionWithoutKeyPrompt(t *testing.T) {
	w := httptest.NewRecorder()
	(&Handler{}).handlePage(w)
	body := w.Body.String()
	for _, want := range []string{
		"window.self !== window.top",
		"if (embeddedInDashboard)",
		"loadAuthFiles();",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page missing embedded behavior %q", want)
		}
	}
	if strings.Contains(body, "const key = managementKey(); if (!key) { return null; }") {
		t.Error("embedded requests must not unconditionally require a management key")
	}
}
