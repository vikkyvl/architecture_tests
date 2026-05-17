package cli

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
)

func renderReportPaths(jsonPath, mdPath string) string {
	arrow := mutedStyle.Render(linkMarker)
	rows := []string{
		renderKVRaw(labelJSONReport, arrow+spaceSeparator+jsonPath),
		renderKVRaw(labelMarkdown, arrow+spaceSeparator+mdPath),
	}
	panel := renderPanel(panelReports, rows)
	panel = insertFileLink(panel, jsonPath)
	panel = insertFileLink(panel, mdPath)
	return panel
}

func insertFileLink(panel, path string) string {
	uri := fileURI(path)
	if uri == "" {
		return panel
	}
	styled := styledFileLinkText(path)
	linked := fmt.Sprintf(formatHyperlink, uri, styled)
	return strings.Replace(panel, path, linked, 1)
}

func styledFileLinkText(path string) string {
	return fmt.Sprintf(formatStyledLink, path)
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
