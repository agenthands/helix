// Package workflow provides prompt templates for onboarding and session handoff.
package workflow

import (
	"bytes"
	"strings"
	"text/template"
)

// OnboardingData holds the variables for the onboarding prompt template.
type OnboardingData struct {
	ProjectRoot        string
	DetectedLanguages  []string
	FileCount          int
	DirectoryStructure []string // top-level directories
}

// HandoffData holds the variables for the session handoff prompt template.
type HandoffData struct {
	SessionID       string
	ToolsUsed       []string
	FilesModified   []string
	MemoriesCreated []string
	OpenContext      string
}

var onboardingTemplate = template.Must(template.New("onboarding").Parse(`# Project Onboarding

You are onboarding a new project. Please analyze the following project information and create memories
that will help you and future sessions work effectively with this codebase.

## Project Root
{{.ProjectRoot}}

## Detected Languages
{{- if .DetectedLanguages}}
{{- range .DetectedLanguages}}
- {{.}}
{{- end}}
{{- else}}
- No languages detected yet
{{- end}}

## File Count
{{.FileCount}} files found in the project.

## Top-Level Directory Structure
{{- if .DirectoryStructure}}
{{- range .DirectoryStructure}}
- {{.}}
{{- end}}
{{- else}}
- Empty project
{{- end}}

## Instructions

Please perform the following onboarding steps:

1. **Explore the project structure** to understand the codebase organization.
2. **Identify key entry points** (main files, configuration, build scripts).
3. **Identify the build/test/run commands** for this project.
4. **Note coding conventions** (formatting, naming, patterns used).
5. **Create memories** for each piece of important information discovered:
   - Use "project-overview" for the high-level summary.
   - Use "build-and-test" for build, test, and run commands.
   - Use "conventions" for coding style and patterns.
   - Use "architecture" for structural decisions and key abstractions.

Focus on information that would help a developer (or AI agent) quickly become productive.
`))

var handoffTemplate = template.Must(template.New("handoff").Parse(`# Session Handoff Summary

This document summarizes the current session state for continuation in a new conversation.

## Session
{{- if .SessionID}}
ID: {{.SessionID}}
{{- end}}

## Tools Used This Session
{{- if .ToolsUsed}}
{{- range .ToolsUsed}}
- {{.}}
{{- end}}
{{- else}}
- None recorded
{{- end}}

## Files Modified
{{- if .FilesModified}}
{{- range .FilesModified}}
- {{.}}
{{- end}}
{{- else}}
- No files modified
{{- end}}

## Memories Created
{{- if .MemoriesCreated}}
{{- range .MemoriesCreated}}
- {{.}}
{{- end}}
{{- else}}
- No memories created
{{- end}}

## Open Context / Remaining Work
{{- if .OpenContext}}
{{.OpenContext}}
{{- else}}
No open items recorded.
{{- end}}

## Instructions for Next Session

Review the above summary to understand what was accomplished and what remains.
Read any memories created during this session if they are relevant to your task.
Continue from where this session left off.
`))

// renderOnboarding renders the onboarding template with the given data.
func renderOnboarding(data OnboardingData) string {
	var buf bytes.Buffer
	if err := onboardingTemplate.Execute(&buf, data); err != nil {
		return "Error rendering onboarding template: " + err.Error()
	}
	return strings.TrimSpace(buf.String())
}

// renderHandoff renders the handoff template with the given data.
func renderHandoff(data HandoffData) string {
	var buf bytes.Buffer
	if err := handoffTemplate.Execute(&buf, data); err != nil {
		return "Error rendering handoff template: " + err.Error()
	}
	return strings.TrimSpace(buf.String())
}
