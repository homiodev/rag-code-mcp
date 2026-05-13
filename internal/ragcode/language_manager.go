package ragcode

import (
	"strings"

	"github.com/homiodev/rag-code-mcp/internal/codetypes"
	"github.com/homiodev/rag-code-mcp/internal/ragcode/analyzers/golang"
	htmlanalyzer "github.com/homiodev/rag-code-mcp/internal/ragcode/analyzers/html"
	"github.com/homiodev/rag-code-mcp/internal/ragcode/analyzers/java"
	"github.com/homiodev/rag-code-mcp/internal/ragcode/analyzers/php/laravel"
	"github.com/homiodev/rag-code-mcp/internal/ragcode/analyzers/python"
)

type Language string

const (
	LanguageGo         Language = "go"
	LanguageJava       Language = "java"
	LanguagePHP        Language = "php"
	LanguageHTML       Language = "html"
	LanguagePython     Language = "python"
	LanguageJavascript Language = "javascript"
	LanguageTypescript Language = "typescript"
)

type AnalyzerManager struct{}

func NewAnalyzerManager() *AnalyzerManager { return &AnalyzerManager{} }

func normalizeProjectType(projectType string) Language {
	pt := strings.ToLower(strings.TrimSpace(projectType))
	switch pt {
	case "", "go", "unknown":
		return LanguageGo
	case "java", "maven", "gradle", "spring", "springboot":
		return LanguageJava
	case "php", "php-laravel", "laravel":
		return LanguagePHP
	case "html", "web", "static-html":
		return LanguageHTML
	case "python", "py", "django", "flask", "fastapi":
		return LanguagePython
	case "node", "nodejs", "javascript", "js", "react":
		return LanguageJavascript
	case "typescript", "ts", "tsx":
		return LanguageTypescript
	default:
		return Language(pt)
	}
}

func (m *AnalyzerManager) CodeAnalyzerForProjectType(projectType string) codetypes.PathAnalyzer {
	lang := normalizeProjectType(projectType)
	switch lang {
	case LanguageGo:
		return golang.NewCodeAnalyzer()
	case LanguageJava:
		return java.NewCodeAnalyzer()
	case LanguagePHP:
		return laravel.NewAdapter()
	case LanguageHTML:
		return htmlanalyzer.NewCodeAnalyzer()
	case LanguagePython:
		return python.NewCodeAnalyzer()
	case LanguageJavascript, LanguageTypescript:
		return nil
	default:
		return nil
	}
}
