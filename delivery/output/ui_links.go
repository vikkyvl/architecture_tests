package output

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
)

func RenderReportPaths(jsonPath, mdPath string) string {
	arrow := mutedStyle.Render(linkMarker)
	rows := []string{
		renderKVRaw(labelJSONReport, arrow+spaceSeparator+jsonPath),
		renderKVRaw(labelMarkdown, arrow+spaceSeparator+mdPath),
	}
	panel := RenderPanel(panelReports, rows)
	panel = insertFileLink(panel, jsonPath)
	panel = insertFileLink(panel, mdPath)
	return panel
}

func insertFileLink(panel, path string) string {
	uri := fileURI(path)
	if uri == "" {
		return panel
	}

	osc8 := fmt.Sprintf(formatHyperlink, uri, path)
	linked := fmt.Sprintf(formatStyledLink, osc8)
	return strings.Replace(panel, path, linked, 1)
}

func fileURI(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return ""
	}
	return (&url.URL{
		Scheme: fileURLScheme,
		Path:   abs,
	}).String()
}
