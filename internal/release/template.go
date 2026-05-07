package release

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
)

const DefaultTemplate = `Release {{ now }}
{{- range .PullRequests }}
{{ .ToChecklistItem }}
{{- end }}
`

type templatePullRequest struct {
	PullRequest
	mentionType string
}

func (pr templatePullRequest) ToChecklistItem() string {
	return pr.PullRequest.ToChecklistItem(false, pr.mentionType)
}

func (pr templatePullRequest) ToChecklistItemWithTitle() string {
	return pr.PullRequest.ToChecklistItem(true, pr.mentionType)
}

func (pr templatePullRequest) Mention() string {
	return pr.PullRequest.Mention(pr.mentionType)
}

func (pr templatePullRequest) TargetUserLoginNames() []string {
	return pr.PullRequest.TargetUserLoginNames(pr.mentionType)
}

func BuildTitleAndBody(
	repoRoot string,
	releasePR *PullRequest,
	mergedPRs []PullRequest,
	changedFiles []ChangedFile,
	templatePath string,
	mentionType string,
) (string, string, error) {
	templateText := DefaultTemplate
	if templatePath != "" {
		fullPath := templatePath
		if !filepath.IsAbs(templatePath) {
			fullPath = filepath.Join(repoRoot, templatePath)
		}
		body, err := os.ReadFile(fullPath)
		if err != nil {
			return "", "", err
		}
		templateText = string(body)
	}

	data := makeTemplateData(releasePR, mergedPRs, changedFiles, mentionType)

	tmpl, err := template.New("release").Funcs(sprig.FuncMap()).Parse(templateText)
	if err != nil {
		return "", "", err
	}

	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, data); err != nil {
		return "", "", err
	}

	content := strings.ReplaceAll(rendered.String(), "\r\n", "\n")
	parts := strings.SplitN(content, "\n", 2)
	title := strings.TrimSpace(parts[0])
	if title == "" {
		return "", "", fmt.Errorf("template title is empty")
	}

	body := ""
	if len(parts) == 2 {
		body = strings.TrimSuffix(parts[1], "\n")
	}

	return title, body, nil
}

func makeTemplateData(
	releasePR *PullRequest,
	mergedPRs []PullRequest,
	changedFiles []ChangedFile,
	mentionType string,
) map[string]any {
	releaseView := templatePullRequest{mentionType: mentionType}
	if releasePR != nil {
		releaseView = templatePullRequest{PullRequest: *releasePR, mentionType: mentionType}
	}

	mergedViews := make([]templatePullRequest, 0, len(mergedPRs))
	for _, pr := range mergedPRs {
		mergedViews = append(mergedViews, templatePullRequest{PullRequest: pr, mentionType: mentionType})
	}

	return map[string]any{
		"ReleasePullRequest":   releaseView,
		"TargetPullRequest":    releaseView,
		"MergedPullRequests":   mergedViews,
		"PullRequests":         mergedViews,
		"ChangedFiles":         changedFiles,
		"release_pull_request": releaseView,
		"target_pull_request":  releaseView,
		"merged_pull_requests": mergedViews,
		"pull_requests":        mergedViews,
		"changed_files":        changedFiles,
	}
}

var checklistLinePattern = regexp.MustCompile(`^- \[(?P<check>[ x])\] #(?P<number>\d+)\b`)

func MergeBodies(oldBody, newBody string) string {
	checkStatus := map[string]string{}
	for _, line := range splitLines(oldBody) {
		matches := checklistLinePattern.FindStringSubmatch(line)
		if len(matches) != 3 {
			continue
		}
		checkStatus[matches[2]] = matches[1]
	}

	oldUnchecked := checklistLinePattern.ReplaceAllString(oldBody, "- [ ] #$2")
	oldLines := splitLines(oldUnchecked)
	newLines := splitLines(newBody)

	ops := diffLines(oldLines, newLines)
	mergedLines := make([]string, 0, len(oldLines)+len(newLines))
	for i := 0; i < len(ops); {
		if ops[i].kind == diffEqual {
			mergedLines = append(mergedLines, ops[i].newLine)
			i++
			continue
		}

		var deletes []string
		var inserts []string
		for i < len(ops) && ops[i].kind != diffEqual {
			switch ops[i].kind {
			case diffDelete:
				deletes = append(deletes, ops[i].oldLine)
			case diffInsert:
				inserts = append(inserts, ops[i].newLine)
			}
			i++
		}

		pairCount := len(deletes)
		if len(inserts) < pairCount {
			pairCount = len(inserts)
		}

		for idx := 0; idx < pairCount; idx++ {
			oldLine := deletes[idx]
			newLine := inserts[idx]
			if isChecklistLine(oldLine) && isChecklistLine(newLine) {
				mergedLines = append(mergedLines, oldLine)
				continue
			}
			mergedLines = append(mergedLines, oldLine, newLine)
		}

		mergedLines = append(mergedLines, deletes[pairCount:]...)
		mergedLines = append(mergedLines, inserts[pairCount:]...)
	}

	mergedBody := strings.Join(mergedLines, "\n")
	for number, status := range checkStatus {
		pattern := regexp.MustCompile(`(?m)^- \[ \] #` + regexp.QuoteMeta(number) + `\b`)
		mergedBody = pattern.ReplaceAllString(mergedBody, "- ["+status+"] #"+number)
	}

	return mergedBody
}

func isChecklistLine(line string) bool {
	return checklistLinePattern.MatchString(line)
}

func splitLines(body string) []string {
	if body == "" {
		return nil
	}
	normalized := strings.ReplaceAll(body, "\r\n", "\n")
	normalized = strings.TrimSuffix(normalized, "\n")
	if normalized == "" {
		return nil
	}
	return strings.Split(normalized, "\n")
}

type diffKind string

const (
	diffEqual  diffKind = "="
	diffDelete diffKind = "-"
	diffInsert diffKind = "+"
)

type diffOp struct {
	kind    diffKind
	oldLine string
	newLine string
}

func diffLines(oldLines, newLines []string) []diffOp {
	matrix := buildLCSMatrix(oldLines, newLines)

	ops := make([]diffOp, 0, len(oldLines)+len(newLines))
	i, j := 0, 0
	for i < len(oldLines) && j < len(newLines) {
		switch {
		case oldLines[i] == newLines[j]:
			ops = append(ops, diffOp{kind: diffEqual, oldLine: oldLines[i], newLine: newLines[j]})
			i++
			j++
		case matrix[i+1][j] >= matrix[i][j+1]:
			ops = append(ops, diffOp{kind: diffDelete, oldLine: oldLines[i]})
			i++
		default:
			ops = append(ops, diffOp{kind: diffInsert, newLine: newLines[j]})
			j++
		}
	}
	for ; i < len(oldLines); i++ {
		ops = append(ops, diffOp{kind: diffDelete, oldLine: oldLines[i]})
	}
	for ; j < len(newLines); j++ {
		ops = append(ops, diffOp{kind: diffInsert, newLine: newLines[j]})
	}
	return ops
}

func buildLCSMatrix(oldLines, newLines []string) [][]int {
	matrix := make([][]int, len(oldLines)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(newLines)+1)
	}

	for i := len(oldLines) - 1; i >= 0; i-- {
		for j := len(newLines) - 1; j >= 0; j-- {
			if oldLines[i] == newLines[j] {
				matrix[i][j] = matrix[i+1][j+1] + 1
				continue
			}
			if matrix[i+1][j] >= matrix[i][j+1] {
				matrix[i][j] = matrix[i+1][j]
			} else {
				matrix[i][j] = matrix[i][j+1]
			}
		}
	}

	return matrix
}
