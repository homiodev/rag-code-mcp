package java

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/homiodev/rag-code-mcp/internal/codetypes"
)

// CodeAnalyzer implements PathAnalyzer for Java source files.
type CodeAnalyzer struct {
	includeTests bool
}

func NewCodeAnalyzer() *CodeAnalyzer                  { return &CodeAnalyzer{} }
func NewCodeAnalyzerWithOptions(t bool) *CodeAnalyzer { return &CodeAnalyzer{includeTests: t} }

func (a *CodeAnalyzer) AnalyzePaths(paths []string) ([]codetypes.CodeChunk, error) {
	var chunks []codetypes.CodeChunk
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if info.IsDir() {
			if c, err := a.analyzeDirectory(path); err == nil {
				chunks = append(chunks, c...)
			}
		} else if strings.HasSuffix(path, ".java") && !a.shouldSkipFile(path) {
			if c, err := a.analyzeFile(path); err == nil {
				chunks = append(chunks, c...)
			}
		}
	}
	return chunks, nil
}

func (a *CodeAnalyzer) analyzeDirectory(dirPath string) ([]codetypes.CodeChunk, error) {
	var chunks []codetypes.CodeChunk
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if shouldSkipDir(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".java") && !a.shouldSkipFile(path) {
			if c, err := a.analyzeFile(path); err == nil {
				chunks = append(chunks, c...)
			}
		}
		return nil
	})
	return chunks, err
}

func (a *CodeAnalyzer) analyzeFile(filePath string) ([]codetypes.CodeChunk, error) {
	raw, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	src := string(raw)
	pkg := extractPackageName(src)
	var chunks []codetypes.CodeChunk

	for _, cls := range a.extractClasses(src, filePath, pkg) {
		chunks = append(chunks, a.classToChunks(cls, pkg)...)
	}
	for _, rec := range a.extractRecords(src, filePath, pkg) {
		chunks = append(chunks, a.classToChunks(rec, pkg)...)
	}
	for _, iface := range a.extractInterfaces(src, filePath, pkg) {
		chunks = append(chunks, a.interfaceToChunks(iface, pkg)...)
	}
	for _, enum := range a.extractEnums(src, filePath, pkg) {
		chunks = append(chunks, a.enumToChunks(enum, pkg)...)
	}
	for _, ann := range a.extractAnnotationTypes(src, filePath, pkg) {
		chunks = append(chunks, annotationTypeToChunk(ann, pkg))
	}
	return chunks, nil
}

// ─── Compiled regexes ────────────────────────────────────────────────────────

var (
	// Type declarations. `[^{;]*\{` captures the full header up to the opening brace,
	// handling extends/implements/permits/generics without separate capture groups.
	reClass = regexp.MustCompile(
		`(?m)^\s*` +
			`((?:(?:public|protected|private|abstract|final|static|sealed|strictfp)\s+|non-sealed\s+)*)` +
			`class\s+([A-Za-z_$][a-zA-Z0-9_$]*)[^{;]*\{`)

	// record Name(components) [implements ...] { — Java 16+
	reRecord = regexp.MustCompile(
		`(?m)^\s*` +
			`((?:(?:public|protected|private|static|final|strictfp)\s+)*)` +
			`record\s+([A-Za-z_$][a-zA-Z0-9_$]*)` +
			`\s*\(([^)]*)\)[^{;]*\{`)

	reInterface = regexp.MustCompile(
		`(?m)^\s*` +
			`((?:(?:public|protected|private|abstract|static|sealed|strictfp)\s+|non-sealed\s+)*)` +
			`interface\s+([A-Za-z_$][a-zA-Z0-9_$]*)[^{;]*\{`)

	// @interface — annotation type declaration
	reAnnotationType = regexp.MustCompile(
		`(?m)^\s*` +
			`((?:(?:public|protected|private|static)\s+)*)` +
			`@interface\s+([A-Za-z_$][a-zA-Z0-9_$]*)\s*\{`)

	reEnum = regexp.MustCompile(
		`(?m)^\s*` +
			`((?:(?:public|protected|private|static|strictfp)\s+)*)` +
			`enum\s+([A-Za-z_$][a-zA-Z0-9_$]*)[^{;]*\{`)

	// Method start: stops at opening `(` so we can manually scan for the full
	// parameter list. Handles all modifiers including default (interface), native,
	// synchronized, strictfp, and optional leading generic type params (<T>, <T extends Foo>).
	reMethodStart = regexp.MustCompile(
		`(?m)^\s*` +
			`((?:(?:public|protected|private|abstract|static|final|synchronized|native|default|strictfp)\s+)*)` +
			`(?:<[^(>]*(?:>[^(>]*)*>\s+)?` + // optional generic type params
			`([\w$][\w$<>\[\].,? ]*)` + // return type (arrays + generics)
			`\s+([\w$]+)` + // method name
			`\s*\(`) // opening paren

	// Field: modifiers, type (inc. arrays/generics), one or more names, optional initializer.
	reField = regexp.MustCompile(
		`(?m)^\s*` +
			`((?:(?:public|protected|private|static|final|transient|volatile|strictfp)\s+)*)` +
			`([\w$][\w$<>\[\].,? ]*)` + // type including int[], List<String>, etc.
			`\s+([\w$]+(?:\s*,\s*[\w$]+)*)` + // one or more variable names
			`\s*(?:=\s*([^;]+?))?` + // optional initializer (non-greedy to avoid eating next statement)
			`\s*;`)

	reAnnotationMarker = regexp.MustCompile(`@[A-Za-z_$][a-zA-Z0-9_$]*(?:\s*\([^)]*\))?`)
	reThrowsClause     = regexp.MustCompile(`throws\s+([\w$., ]+)`)
	reExtendsClause    = regexp.MustCompile(`extends\s+([\w$.<>, ]+?)(?:\s+(?:implements|permits|\{)|$)`)
	reImplementsClause = regexp.MustCompile(`implements\s+([\w$.<>, ]+?)(?:\s+(?:permits|\{)|$)`)
	rePermitsClause    = regexp.MustCompile(`permits\s+([\w$., ]+)`)
	reRetentionPolicy  = regexp.MustCompile(`@Retention\s*\(\s*RetentionPolicy\.(\w+)`)
	reTargetElements   = regexp.MustCompile(`@Target\s*\(\s*\{?([^})]+)\}?\s*\)`)
)

// ─── Class extraction ────────────────────────────────────────────────────────

func (a *CodeAnalyzer) extractClasses(content, filePath, pkg string) []ClassInfo {
	var out []ClassInfo
	for _, match := range reClass.FindAllStringSubmatchIndex(content, -1) {
		if cls := a.parseTypeBody(content, match, filePath, pkg, "class"); cls != nil {
			out = append(out, *cls)
		}
	}
	return out
}

// ─── Record extraction (Java 16+) ───────────────────────────────────────────

func (a *CodeAnalyzer) extractRecords(content, filePath, pkg string) []ClassInfo {
	var out []ClassInfo
	for _, match := range reRecord.FindAllStringSubmatchIndex(content, -1) {
		if rec := a.parseRecordBody(content, match, filePath, pkg); rec != nil {
			out = append(out, *rec)
		}
	}
	return out
}

func (a *CodeAnalyzer) parseRecordBody(content string, match []int, filePath, pkg string) *ClassInfo {
	if match[4] < 0 {
		return nil
	}
	header := content[match[0]:match[1]]
	name := content[match[4]:match[5]]

	rec := &ClassInfo{
		Name:           name,
		FullyQualified: pkg + "." + name,
		Kind:           "record",
		FilePath:       filePath,
		AccessModifier: extractAccessModifier(header),
		IsFinal:        true, // records are implicitly final
		StartLine:      strings.Count(content[:match[0]], "\n") + 1,
		Annotations:    extractAnnotationsBefore(content, match[0]),
		Description:    extractJavaDoc(content, match[0]),
	}

	// Record components become a canonical constructor.
	if match[6] >= 0 {
		componentsStr := content[match[6]:match[7]]
		rec.Constructors = []ConstructorInfo{{
			Name:       name,
			Signature:  name + "(" + componentsStr + ")",
			FilePath:   filePath,
			Parameters: parseParameters(componentsStr),
		}}
		// Also expose components as fields for searchability.
		for _, p := range rec.Constructors[0].Parameters {
			rec.Fields = append(rec.Fields, FieldInfo{
				Name:     p.Name,
				Type:     p.Type,
				FilePath: filePath,
			})
		}
	}

	if m := reImplementsClause.FindStringSubmatch(header); len(m) > 1 {
		for _, iface := range strings.Split(m[1], ",") {
			rec.Interfaces = append(rec.Interfaces, strings.TrimSpace(iface))
		}
	}
	if m := rePermitsClause.FindStringSubmatch(header); len(m) > 1 {
		rec.Description = appendIfNonempty(rec.Description, "permits "+strings.TrimSpace(m[1]))
	}

	startBrace := match[1] - 1
	endBrace := findMatchingBrace(content, startBrace)
	if endBrace > 0 {
		rec.EndLine = strings.Count(content[:endBrace], "\n") + 1
		rec.Code = content[match[0] : endBrace+1]
		body := content[match[1]:endBrace]
		rec.Methods = a.extractMethods(body, filePath, name)
	}
	expandLombok(rec)
	return rec
}

// ─── Interface extraction ────────────────────────────────────────────────────

func (a *CodeAnalyzer) extractInterfaces(content, filePath, pkg string) []ClassInfo {
	var out []ClassInfo
	for _, match := range reInterface.FindAllStringSubmatchIndex(content, -1) {
		if iface := a.parseTypeBody(content, match, filePath, pkg, "interface"); iface != nil {
			out = append(out, *iface)
		}
	}
	return out
}

// ─── Enum extraction ─────────────────────────────────────────────────────────

func (a *CodeAnalyzer) extractEnums(content, filePath, pkg string) []EnumInfo {
	var out []EnumInfo
	for _, match := range reEnum.FindAllStringSubmatchIndex(content, -1) {
		if e := a.parseEnumBody(content, match, filePath, pkg); e != nil {
			out = append(out, *e)
		}
	}
	return out
}

func (a *CodeAnalyzer) parseEnumBody(content string, match []int, filePath, pkg string) *EnumInfo {
	if match[4] < 0 {
		return nil
	}
	header := content[match[0]:match[1]]
	name := content[match[4]:match[5]]

	e := &EnumInfo{
		Name:           name,
		FullyQualified: pkg + "." + name,
		FilePath:       filePath,
		AccessModifier: extractAccessModifier(header),
		StartLine:      strings.Count(content[:match[0]], "\n") + 1,
		Annotations:    extractAnnotationsBefore(content, match[0]),
		Description:    extractJavaDoc(content, match[0]),
	}

	if m := reImplementsClause.FindStringSubmatch(header); len(m) > 1 {
		for _, iface := range strings.Split(m[1], ",") {
			e.Interfaces = append(e.Interfaces, strings.TrimSpace(iface))
		}
	}

	startBrace := match[1] - 1
	endBrace := findMatchingBrace(content, startBrace)
	if endBrace > 0 {
		e.EndLine = strings.Count(content[:endBrace], "\n") + 1
		e.Code = content[match[0] : endBrace+1]
		body := content[match[1]:endBrace]
		e.Constants = extractEnumConstants(body)
		e.Methods = a.extractMethods(body, filePath, name)
	}
	return e
}

// ─── Annotation type extraction (@interface) — Java 5+ ──────────────────────

func (a *CodeAnalyzer) extractAnnotationTypes(content, filePath, pkg string) []AnnotationInfo {
	var out []AnnotationInfo
	for _, match := range reAnnotationType.FindAllStringSubmatchIndex(content, -1) {
		if ann := a.parseAnnotationTypeBody(content, match, filePath, pkg); ann != nil {
			out = append(out, *ann)
		}
	}
	return out
}

func (a *CodeAnalyzer) parseAnnotationTypeBody(content string, match []int, filePath, pkg string) *AnnotationInfo {
	if match[4] < 0 {
		return nil
	}
	name := content[match[4]:match[5]]
	preceding := extractAnnotationsBefore(content, match[0])

	ann := &AnnotationInfo{
		Name:        name,
		FilePath:    filePath,
		StartLine:   strings.Count(content[:match[0]], "\n") + 1,
		Description: extractJavaDoc(content, match[0]),
	}

	// Retention and Target from meta-annotations preceding the declaration.
	joinedPreceding := strings.Join(preceding, " ")
	if m := reRetentionPolicy.FindStringSubmatch(joinedPreceding); len(m) > 1 {
		ann.Retention = m[1]
	}
	if m := reTargetElements.FindStringSubmatch(joinedPreceding); len(m) > 1 {
		for _, t := range strings.Split(m[1], ",") {
			t = strings.TrimSpace(t)
			t = strings.TrimPrefix(t, "ElementType.")
			if t != "" {
				ann.Target = append(ann.Target, t)
			}
		}
	}

	startBrace := match[1] - 1
	endBrace := findMatchingBrace(content, startBrace)
	if endBrace > 0 {
		body := content[match[1]:endBrace]
		// Extract annotation elements (look like methods ending with default or ;).
		elemPattern := regexp.MustCompile(`(?m)^\s*([\w$][\w$<>\[\]., ]*)\s+([\w$]+)\s*\(\s*\)(?:\s+default\s+([^;]+))?;`)
		for _, em := range elemPattern.FindAllStringSubmatch(body, -1) {
			elem := AnnotationElement{Type: strings.TrimSpace(em[1]), Name: em[2]}
			if len(em) > 3 {
				elem.DefaultValue = strings.TrimSpace(em[3])
			}
			ann.Elements = append(ann.Elements, elem)
		}
	}
	return ann
}

// ─── parseTypeBody — shared parser for class and interface ───────────────────

func (a *CodeAnalyzer) parseTypeBody(content string, match []int, filePath, pkg, kind string) *ClassInfo {
	if match[4] < 0 {
		return nil
	}
	header := content[match[0]:match[1]]
	name := content[match[4]:match[5]]

	cls := &ClassInfo{
		Name:           name,
		FullyQualified: pkg + "." + name,
		Kind:           kind,
		FilePath:       filePath,
		AccessModifier: extractAccessModifier(header),
		IsAbstract:     strings.Contains(header, "abstract"),
		IsFinal:        strings.Contains(header, "final"),
		IsStatic:       strings.Contains(header, "static"),
		IsSealed:       strings.Contains(header, "sealed") && !strings.Contains(header, "non-sealed"),
		StartLine:      strings.Count(content[:match[0]], "\n") + 1,
		Annotations:    extractAnnotationsBefore(content, match[0]),
		Description:    extractJavaDoc(content, match[0]),
	}

	if m := reExtendsClause.FindStringSubmatch(header); len(m) > 1 {
		cls.Superclass = strings.TrimSpace(m[1])
	}
	if m := reImplementsClause.FindStringSubmatch(header); len(m) > 1 {
		for _, iface := range strings.Split(m[1], ",") {
			cls.Interfaces = append(cls.Interfaces, strings.TrimSpace(iface))
		}
	}
	if m := rePermitsClause.FindStringSubmatch(header); len(m) > 1 {
		cls.Description = appendIfNonempty(cls.Description, "permits "+strings.TrimSpace(m[1]))
	}

	startBrace := match[1] - 1
	endBrace := findMatchingBrace(content, startBrace)
	if endBrace > 0 {
		cls.EndLine = strings.Count(content[:endBrace], "\n") + 1
		cls.Code = content[match[0] : endBrace+1]
		body := content[match[1]:endBrace]
		cls.Methods = a.extractMethods(body, filePath, name)
		if kind == "class" {
			cls.Fields = a.extractFields(body, filePath)
			cls.Constructors = a.extractConstructors(body, filePath, name)
		}
	}
	expandLombok(cls)
	return cls
}

// ─── Method extraction ───────────────────────────────────────────────────────

var javaKeywords = map[string]bool{
	"if": true, "else": true, "for": true, "while": true, "do": true,
	"switch": true, "case": true, "return": true, "new": true, "throw": true,
	"try": true, "catch": true, "finally": true, "assert": true,
	"instanceof": true, "this": true, "super": true, "class": true,
	"interface": true, "enum": true, "record": true, "import": true,
	"package": true, "extends": true, "implements": true, "permits": true,
	"var": true, "yield": true, "sealed": true, "when": true,
}

var skipReturnPrefixes = map[string]bool{
	"return": true, "throw": true, "new": true, "else": true,
}

func (a *CodeAnalyzer) extractMethods(content, filePath, className string) []MethodInfo {
	var methods []MethodInfo

	for _, match := range reMethodStart.FindAllStringSubmatchIndex(content, -1) {
		if match[6] < 0 {
			continue
		}

		modifiers := ""
		if match[2] >= 0 {
			modifiers = strings.TrimSpace(content[match[2]:match[3]])
		}
		returnType := strings.TrimSpace(content[match[4]:match[5]])
		methodName := content[match[6]:match[7]]

		// Filter out Java keywords and statement-level patterns.
		if javaKeywords[methodName] {
			continue
		}
		if firstWord := strings.Fields(returnType); len(firstWord) > 0 && skipReturnPrefixes[firstWord[0]] {
			continue
		}
		// Skip constructors (handled separately) and obvious non-methods.
		if methodName == className {
			continue
		}

		// The match ends with `(`. Find the matching `)`.
		openParen := match[1] - 1
		for openParen >= 0 && content[openParen] != '(' {
			openParen--
		}
		if openParen < 0 {
			continue
		}
		closeParen := findMatchingParen(content, openParen)
		if closeParen < 0 {
			continue
		}

		paramsStr := content[openParen+1 : closeParen]

		// Scan from closeParen+1 for optional throws clause, then `{` or `;`.
		var throwsExceptions []string
		pos := closeParen + 1
		if tm := reThrowsClause.FindStringIndex(content[pos:]); tm != nil {
			beforeBrace := strings.IndexAny(content[pos:], "{;")
			if beforeBrace < 0 || tm[0] < beforeBrace {
				if tsm := reThrowsClause.FindStringSubmatch(content[pos:]); len(tsm) > 1 {
					for _, t := range strings.Split(tsm[1], ",") {
						if t = strings.TrimSpace(t); t != "" {
							throwsExceptions = append(throwsExceptions, t)
						}
					}
				}
			}
		}

		// Find body `{` or abstract `;`. Scan at most 512 chars to avoid
		// runaway matching on false-positive method starts.
		var code string
		endLine := 0
		scanLimit := pos + 512
		if scanLimit > len(content) {
			scanLimit = len(content)
		}
		for i := pos; i < scanLimit; i++ {
			switch content[i] {
			case '{':
				end := findMatchingBrace(content, i)
				if end > 0 {
					code = content[match[0] : end+1]
					endLine = strings.Count(content[:end], "\n") + 1
				}
				i = scanLimit
			case ';':
				code = content[match[0] : i+1]
				endLine = strings.Count(content[:i], "\n") + 1
				i = scanLimit
			}
			// All other chars (whitespace, comments, annotations between
			// signature and body) are skipped transparently.
		}
		if endLine == 0 {
			continue
		}

		sig := strings.TrimSpace(content[match[0] : closeParen+1])

		methods = append(methods, MethodInfo{
			Name:             methodName,
			Signature:        sig,
			ReturnType:       returnType,
			FilePath:         filePath,
			AccessModifier:   extractAccessModifier(modifiers),
			IsAbstract:       strings.Contains(modifiers, "abstract"),
			IsStatic:         strings.Contains(modifiers, "static"),
			IsFinal:          strings.Contains(modifiers, "final"),
			IsSynchronized:   strings.Contains(modifiers, "synchronized"),
			IsNative:         strings.Contains(modifiers, "native"),
			ThrowsExceptions: throwsExceptions,
			Parameters:       parseParameters(paramsStr),
			Annotations:      extractAnnotationsBefore(content, match[0]),
			Description:      extractJavaDoc(content, match[0]),
			StartLine:        strings.Count(content[:match[0]], "\n") + 1,
			EndLine:          endLine,
			Code:             code,
		})
	}
	return methods
}

// ─── Field extraction ────────────────────────────────────────────────────────

func (a *CodeAnalyzer) extractFields(content, filePath string) []FieldInfo {
	var fields []FieldInfo

	for _, match := range reField.FindAllStringSubmatchIndex(content, -1) {
		if match[4] < 0 {
			continue
		}
		modifiers := ""
		if match[2] >= 0 {
			modifiers = content[match[2]:match[3]]
		}
		fieldType := strings.TrimSpace(content[match[4]:match[5]])
		namesStr := content[match[6]:match[7]]
		defaultValue := ""
		if match[8] >= 0 {
			defaultValue = strings.TrimSpace(content[match[8]:match[9]])
		}

		// Skip if type looks like a keyword (common false positive from statement patterns).
		if javaKeywords[fieldType] {
			continue
		}

		startLine := strings.Count(content[:match[0]], "\n") + 1
		annotations := extractAnnotationsBefore(content, match[0])

		for _, name := range strings.Split(namesStr, ",") {
			name = strings.TrimSpace(name)
			if name == "" || javaKeywords[name] {
				continue
			}
			fields = append(fields, FieldInfo{
				Name:           name,
				Type:           fieldType,
				FilePath:       filePath,
				AccessModifier: extractAccessModifier(modifiers),
				IsStatic:       strings.Contains(modifiers, "static"),
				IsFinal:        strings.Contains(modifiers, "final"),
				IsTransient:    strings.Contains(modifiers, "transient"),
				IsVolatile:     strings.Contains(modifiers, "volatile"),
				DefaultValue:   defaultValue,
				Annotations:    annotations,
				StartLine:      startLine,
			})
		}
	}
	return fields
}

// ─── Constructor extraction ──────────────────────────────────────────────────

func (a *CodeAnalyzer) extractConstructors(content, filePath, className string) []ConstructorInfo {
	var constructors []ConstructorInfo

	ctorPattern := regexp.MustCompile(fmt.Sprintf(
		`(?m)^\s*((?:(?:public|protected|private)\s+)*)%s\s*\(`,
		regexp.QuoteMeta(className)))

	for _, match := range ctorPattern.FindAllStringSubmatchIndex(content, -1) {
		modifiers := ""
		if match[2] >= 0 {
			modifiers = content[match[2]:match[3]]
		}

		openParen := match[1] - 1
		for openParen >= 0 && content[openParen] != '(' {
			openParen--
		}
		if openParen < 0 {
			continue
		}
		closeParen := findMatchingParen(content, openParen)
		if closeParen < 0 {
			continue
		}

		paramsStr := content[openParen+1 : closeParen]

		var throwsExceptions []string
		pos := closeParen + 1
		if tm := reThrowsClause.FindStringIndex(content[pos:]); tm != nil {
			beforeBrace := strings.IndexAny(content[pos:], "{;")
			if beforeBrace < 0 || tm[0] < beforeBrace {
				if tsm := reThrowsClause.FindStringSubmatch(content[pos:]); len(tsm) > 1 {
					for _, t := range strings.Split(tsm[1], ",") {
						if t = strings.TrimSpace(t); t != "" {
							throwsExceptions = append(throwsExceptions, t)
						}
					}
				}
			}
		}

		var code string
		endLine := 0
		scanLimit := pos + 512
		if scanLimit > len(content) {
			scanLimit = len(content)
		}
		for i := pos; i < scanLimit; i++ {
			if content[i] == '{' {
				end := findMatchingBrace(content, i)
				if end > 0 {
					code = content[match[0] : end+1]
					endLine = strings.Count(content[:end], "\n") + 1
				}
				break
			}
		}
		if endLine == 0 {
			continue
		}

		constructors = append(constructors, ConstructorInfo{
			Name:             className,
			Signature:        strings.TrimSpace(content[match[0] : closeParen+1]),
			FilePath:         filePath,
			AccessModifier:   extractAccessModifier(modifiers),
			Parameters:       parseParameters(paramsStr),
			ThrowsExceptions: throwsExceptions,
			Annotations:      extractAnnotationsBefore(content, match[0]),
			Description:      extractJavaDoc(content, match[0]),
			StartLine:        strings.Count(content[:match[0]], "\n") + 1,
			EndLine:          endLine,
			Code:             code,
		})
	}
	return constructors
}

// ─── Enum constant extraction ─────────────────────────────────────────────────

func extractEnumConstants(body string) []EnumConstant {
	var constants []EnumConstant
	// Constants appear before the first `;` (if any) in the enum body.
	section := body
	if idx := strings.Index(body, ";"); idx >= 0 {
		section = body[:idx]
	}
	for _, line := range strings.Split(section, ",") {
		line = strings.TrimSpace(line)
		line = strings.TrimLeft(line, "\n\r\t ")
		if line == "" {
			continue
		}
		// Strip any annotations or javadoc on preceding lines.
		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}
		name := parts[0]
		// Strip constructor-call suffix, e.g. CONSTANT("arg") → CONSTANT.
		if idx := strings.IndexAny(name, "("); idx >= 0 {
			name = name[:idx]
		}
		if name == "" || javaKeywords[name] || strings.HasPrefix(name, "//") || strings.HasPrefix(name, "/*") || strings.HasPrefix(name, "@") {
			continue
		}
		// Constant names are ALL_CAPS by convention but we accept any identifier.
		if !isIdentStart(rune(name[0])) {
			continue
		}
		constants = append(constants, EnumConstant{Name: name, Value: line})
	}
	return constants
}

// ─── Chunk converters ────────────────────────────────────────────────────────

func (a *CodeAnalyzer) classToChunks(cls ClassInfo, pkg string) []codetypes.CodeChunk {
	var chunks []codetypes.CodeChunk

	chunk := codetypes.CodeChunk{
		Type:      cls.Kind,
		Name:      cls.Name,
		Package:   pkg,
		Language:  "java",
		Docstring: cls.Description,
		Signature: buildClassSignature(cls),
		FilePath:  cls.FilePath,
		StartLine: cls.StartLine,
		EndLine:   cls.EndLine,
		Code:      cls.Code,
		Metadata: map[string]any{
			"fully_qualified": cls.FullyQualified,
			"access_modifier": cls.AccessModifier,
			"is_abstract":     cls.IsAbstract,
			"is_final":        cls.IsFinal,
			"is_static":       cls.IsStatic,
			"is_sealed":       cls.IsSealed,
			"superclass":      cls.Superclass,
			"interfaces":      cls.Interfaces,
			"annotations":     cls.Annotations,
		},
	}
	chunks = append(chunks, chunk)

	for _, method := range cls.Methods {
		chunks = append(chunks, methodToChunk(method, pkg, cls.Name))
	}
	for _, ctor := range cls.Constructors {
		chunks = append(chunks, constructorToChunk(ctor, pkg, cls.Name))
	}
	for _, field := range cls.Fields {
		chunks = append(chunks, fieldToChunk(field, pkg, cls.Name))
	}
	return chunks
}

func (a *CodeAnalyzer) interfaceToChunks(iface ClassInfo, pkg string) []codetypes.CodeChunk {
	var chunks []codetypes.CodeChunk

	chunk := codetypes.CodeChunk{
		Type:      "interface",
		Name:      iface.Name,
		Package:   pkg,
		Language:  "java",
		Docstring: iface.Description,
		Signature: buildClassSignature(iface),
		FilePath:  iface.FilePath,
		StartLine: iface.StartLine,
		EndLine:   iface.EndLine,
		Code:      iface.Code,
		Metadata: map[string]any{
			"fully_qualified": iface.FullyQualified,
			"access_modifier": iface.AccessModifier,
			"is_sealed":       iface.IsSealed,
			"extends":         iface.Interfaces,
			"annotations":     iface.Annotations,
		},
	}
	chunks = append(chunks, chunk)

	for _, method := range iface.Methods {
		chunks = append(chunks, methodToChunk(method, pkg, iface.Name))
	}
	return chunks
}

func (a *CodeAnalyzer) enumToChunks(enum EnumInfo, pkg string) []codetypes.CodeChunk {
	var chunks []codetypes.CodeChunk

	chunk := codetypes.CodeChunk{
		Type:      "enum",
		Name:      enum.Name,
		Package:   pkg,
		Language:  "java",
		Docstring: enum.Description,
		Signature: buildEnumSignature(enum),
		FilePath:  enum.FilePath,
		StartLine: enum.StartLine,
		EndLine:   enum.EndLine,
		Code:      enum.Code,
		Metadata: map[string]any{
			"fully_qualified": enum.FullyQualified,
			"access_modifier": enum.AccessModifier,
			"constants":       len(enum.Constants),
			"annotations":     enum.Annotations,
		},
	}
	chunks = append(chunks, chunk)

	for _, c := range enum.Constants {
		chunks = append(chunks, codetypes.CodeChunk{
			Type:     "enum_constant",
			Name:     c.Name,
			Package:  pkg,
			Language: "java",
			Code:     c.Value,
			Metadata: map[string]any{"enum_name": enum.Name},
		})
	}
	for _, method := range enum.Methods {
		chunks = append(chunks, methodToChunk(method, pkg, enum.Name))
	}
	return chunks
}

func annotationTypeToChunk(ann AnnotationInfo, pkg string) codetypes.CodeChunk {
	meta := map[string]any{
		"retention": ann.Retention,
		"target":    ann.Target,
		"elements":  len(ann.Elements),
	}
	return codetypes.CodeChunk{
		Type:      "annotation_type",
		Name:      ann.Name,
		Package:   pkg,
		Language:  "java",
		Docstring: ann.Description,
		FilePath:  ann.FilePath,
		StartLine: ann.StartLine,
		Metadata:  meta,
	}
}

func methodToChunk(m MethodInfo, pkg, className string) codetypes.CodeChunk {
	return codetypes.CodeChunk{
		Type:      "method",
		Name:      m.Name,
		Package:   pkg,
		Language:  "java",
		Docstring: m.Description,
		Signature: m.Signature,
		FilePath:  m.FilePath,
		StartLine: m.StartLine,
		EndLine:   m.EndLine,
		Code:      m.Code,
		Metadata: map[string]any{
			"class_name":        className,
			"access_modifier":   m.AccessModifier,
			"is_abstract":       m.IsAbstract,
			"is_static":         m.IsStatic,
			"is_final":          m.IsFinal,
			"is_synchronized":   m.IsSynchronized,
			"is_native":         m.IsNative,
			"return_type":       m.ReturnType,
			"throws_exceptions": m.ThrowsExceptions,
			"annotations":       m.Annotations,
		},
	}
}

func constructorToChunk(c ConstructorInfo, pkg, className string) codetypes.CodeChunk {
	return codetypes.CodeChunk{
		Type:      "constructor",
		Name:      c.Name,
		Package:   pkg,
		Language:  "java",
		Docstring: c.Description,
		Signature: c.Signature,
		FilePath:  c.FilePath,
		StartLine: c.StartLine,
		EndLine:   c.EndLine,
		Code:      c.Code,
		Metadata: map[string]any{
			"class_name":        className,
			"access_modifier":   c.AccessModifier,
			"throws_exceptions": c.ThrowsExceptions,
			"annotations":       c.Annotations,
		},
	}
}

func fieldToChunk(f FieldInfo, pkg, className string) codetypes.CodeChunk {
	return codetypes.CodeChunk{
		Type:      "field",
		Name:      f.Name,
		Package:   pkg,
		Language:  "java",
		Docstring: f.Description,
		FilePath:  f.FilePath,
		StartLine: f.StartLine,
		Metadata: map[string]any{
			"class_name":      className,
			"access_modifier": f.AccessModifier,
			"type":            f.Type,
			"is_static":       f.IsStatic,
			"is_final":        f.IsFinal,
			"default_value":   f.DefaultValue,
			"annotations":     f.Annotations,
		},
	}
}

// ─── Signature builders ───────────────────────────────────────────────────────

func buildClassSignature(cls ClassInfo) string {
	var b strings.Builder
	if cls.AccessModifier != "package-private" {
		b.WriteString(cls.AccessModifier + " ")
	}
	if cls.IsSealed {
		b.WriteString("sealed ")
	}
	if cls.IsAbstract {
		b.WriteString("abstract ")
	}
	if cls.IsFinal {
		b.WriteString("final ")
	}
	if cls.IsStatic {
		b.WriteString("static ")
	}
	b.WriteString(cls.Kind + " " + cls.Name)
	if cls.Superclass != "" {
		b.WriteString(" extends " + cls.Superclass)
	}
	if len(cls.Interfaces) > 0 {
		if cls.Kind == "interface" {
			b.WriteString(" extends " + strings.Join(cls.Interfaces, ", "))
		} else {
			b.WriteString(" implements " + strings.Join(cls.Interfaces, ", "))
		}
	}
	return b.String()
}

func buildEnumSignature(e EnumInfo) string {
	sig := e.AccessModifier + " enum " + e.Name
	if len(e.Interfaces) > 0 {
		sig += " implements " + strings.Join(e.Interfaces, ", ")
	}
	return sig
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func extractPackageName(content string) string {
	re := regexp.MustCompile(`(?m)^\s*package\s+([a-zA-Z0-9_.]+)\s*;`)
	if m := re.FindStringSubmatch(content); len(m) > 1 {
		return m[1]
	}
	return "default"
}

func extractAccessModifier(text string) string {
	switch {
	case strings.Contains(text, "public"):
		return "public"
	case strings.Contains(text, "protected"):
		return "protected"
	case strings.Contains(text, "private"):
		return "private"
	default:
		return "package-private"
	}
}

// extractJavaDoc finds the /** ... */ block immediately preceding position,
// allowing only whitespace and annotations between the comment and the declaration.
func extractJavaDoc(content string, position int) string {
	before := content[:position]

	lastEnd := strings.LastIndex(before, "*/")
	if lastEnd < 0 {
		return ""
	}
	// Reject if structural tokens appear between end of comment and the declaration.
	between := before[lastEnd+2:]
	if strings.ContainsAny(between, "{}") {
		return ""
	}

	start := strings.LastIndex(before[:lastEnd], "/**")
	if start < 0 {
		return ""
	}

	var lines []string
	for _, line := range strings.Split(content[start:lastEnd+2], "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "/**")
		line = strings.TrimSuffix(line, "*/")
		line = strings.TrimPrefix(line, "*")
		line = strings.TrimSpace(line)
		// Skip @param/@return/@throws tags — keep only the summary text.
		if line != "" && !strings.HasPrefix(line, "@") {
			lines = append(lines, line)
		}
	}
	return strings.TrimSpace(strings.Join(lines, " "))
}

// extractAnnotationsBefore collects all @Annotation(...) markers appearing after
// the last structural boundary (}, {, ;) and before position.
func extractAnnotationsBefore(content string, position int) []string {
	before := content[:position]
	lastBoundary := strings.LastIndexAny(before, "}{;")
	if lastBoundary >= 0 {
		before = before[lastBoundary+1:]
	}
	return reAnnotationMarker.FindAllString(before, -1)
}

// skipLiteral advances past a string literal, char literal, text block, or comment
// starting at position i. Returns the new position after the construct.
func skipLiteral(content string, i int) int {
	if i >= len(content) {
		return i
	}
	c := content[i]
	switch {
	case c == '/' && i+1 < len(content) && content[i+1] == '/':
		// Line comment — skip to end of line.
		for i < len(content) && content[i] != '\n' {
			i++
		}
	case c == '/' && i+1 < len(content) && content[i+1] == '*':
		// Block comment — skip to */.
		i += 2
		for i+1 < len(content) {
			if content[i] == '*' && content[i+1] == '/' {
				return i + 2
			}
			i++
		}
		return len(content)
	case c == '"' && i+2 < len(content) && content[i+1] == '"' && content[i+2] == '"':
		// Text block (Java 15+) — skip to closing """.
		i += 3
		for i+2 < len(content) {
			if content[i] == '"' && content[i+1] == '"' && content[i+2] == '"' {
				return i + 3
			}
			i++
		}
		return len(content)
	case c == '"':
		// String literal.
		i++
		for i < len(content) {
			if content[i] == '\\' {
				i += 2
				continue
			}
			if content[i] == '"' {
				return i + 1
			}
			i++
		}
		return len(content)
	case c == '\'':
		// Char literal.
		i++
		for i < len(content) {
			if content[i] == '\\' {
				i += 2
				continue
			}
			if content[i] == '\'' {
				return i + 1
			}
			i++
		}
		return len(content)
	default:
		return i + 1
	}
	return i
}

func findMatchingBrace(content string, openPos int) int {
	count := 1
	i := openPos + 1
	for i < len(content) {
		c := content[i]
		if c == '/' || c == '"' || c == '\'' {
			i = skipLiteral(content, i)
			continue
		}
		if c == '{' {
			count++
		} else if c == '}' {
			count--
			if count == 0 {
				return i
			}
		}
		i++
	}
	return -1
}

func findMatchingParen(content string, openPos int) int {
	count := 1
	i := openPos + 1
	for i < len(content) {
		c := content[i]
		if c == '/' || c == '"' || c == '\'' {
			i = skipLiteral(content, i)
			continue
		}
		if c == '(' {
			count++
		} else if c == ')' {
			count--
			if count == 0 {
				return i
			}
		}
		i++
	}
	return -1
}

// parseParameters splits a parameter list string into ParamInfo entries.
// Handles generics (via smartSplit), arrays (int[]), varargs (String...), annotations,
// and the final modifier.
func parseParameters(paramsStr string) []codetypes.ParamInfo {
	var params []codetypes.ParamInfo
	if strings.TrimSpace(paramsStr) == "" {
		return params
	}
	for _, param := range smartSplit(paramsStr, ",") {
		param = strings.TrimSpace(param)
		if param == "" {
			continue
		}
		// Strip annotations (e.g. @NotNull, @Nullable(value="x")).
		param = reAnnotationMarker.ReplaceAllString(param, "")
		// Strip final modifier.
		param = regexp.MustCompile(`\bfinal\b`).ReplaceAllString(param, "")
		param = strings.TrimSpace(param)

		parts := strings.Fields(param)
		if len(parts) < 2 {
			continue
		}
		paramName := parts[len(parts)-1]
		paramType := strings.Join(parts[:len(parts)-1], " ")
		params = append(params, codetypes.ParamInfo{Name: paramName, Type: paramType})
	}
	return params
}

// smartSplit splits s by sep while respecting angle-bracket depth (generics)
// and parenthesis depth, so commas inside <> or () are not treated as separators.
func smartSplit(s, sep string) []string {
	var result []string
	var current strings.Builder
	var angleDepth, parenDepth int

	for _, ch := range s {
		switch ch {
		case '<':
			angleDepth++
			current.WriteRune(ch)
		case '>':
			if angleDepth > 0 {
				angleDepth--
			}
			current.WriteRune(ch)
		case '(':
			parenDepth++
			current.WriteRune(ch)
		case ')':
			if parenDepth > 0 {
				parenDepth--
			}
			current.WriteRune(ch)
		case rune(sep[0]):
			if angleDepth == 0 && parenDepth == 0 {
				result = append(result, current.String())
				current.Reset()
			} else {
				current.WriteRune(ch)
			}
		default:
			current.WriteRune(ch)
		}
	}
	if current.Len() > 0 {
		result = append(result, current.String())
	}
	return result
}

func appendIfNonempty(base, suffix string) string {
	if base == "" {
		return suffix
	}
	return base + ". " + suffix
}

func isIdentStart(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_' || r == '$'
}

// ─── Lombok annotation expansion ─────────────────────────────────────────────

// hasLombokAnnotation reports whether annotations contains the given Lombok
// annotation by short name (e.g. "Data") or qualified form (e.g. "lombok.Data").
func hasLombokAnnotation(annotations []string, name string) bool {
	for _, ann := range annotations {
		bare := ann
		if i := strings.Index(bare, "("); i >= 0 {
			bare = bare[:i]
		}
		bare = strings.TrimPrefix(bare, "@")
		if bare == name || bare == "lombok."+name || strings.HasSuffix(bare, "."+name) {
			return true
		}
	}
	return false
}

func hasMethod(methods []MethodInfo, name string) bool {
	for _, m := range methods {
		if m.Name == name {
			return true
		}
	}
	return false
}

func lombokGetterName(fieldName, fieldType string) string {
	if (fieldType == "boolean" || fieldType == "Boolean") &&
		strings.HasPrefix(fieldName, "is") && len(fieldName) > 2 &&
		fieldName[2] >= 'A' && fieldName[2] <= 'Z' {
		return fieldName
	}
	return "get" + strings.ToUpper(fieldName[:1]) + fieldName[1:]
}

func lombokSetterName(fieldName string) string {
	return "set" + strings.ToUpper(fieldName[:1]) + fieldName[1:]
}

func lombokGeneratedMethod(name, returnType, sig, filePath string, startLine int) MethodInfo {
	return MethodInfo{
		Name:           name,
		Signature:      sig,
		ReturnType:     returnType,
		AccessModifier: "public",
		FilePath:       filePath,
		StartLine:      startLine,
		Annotations:    []string{"@lombok.Generated"},
		Code:           "// Lombok-generated",
	}
}

func buildLombokCtor(className string, fields []FieldInfo, filePath string, startLine int) ConstructorInfo {
	var params []codetypes.ParamInfo
	for _, f := range fields {
		params = append(params, codetypes.ParamInfo{Name: f.Name, Type: f.Type})
	}
	paramParts := make([]string, len(params))
	for i, p := range params {
		paramParts[i] = p.Type + " " + p.Name
	}
	return ConstructorInfo{
		Name:           className,
		Signature:      fmt.Sprintf("public %s(%s)", className, strings.Join(paramParts, ", ")),
		AccessModifier: "public",
		Parameters:     params,
		FilePath:       filePath,
		StartLine:      startLine,
		Annotations:    []string{"@lombok.Generated"},
		Code:           "// Lombok-generated",
	}
}

// expandLombok appends virtual methods and constructors to cls based on Lombok
// annotations present on the class or its fields. Only generates entries that
// don't already exist in the parsed source.
func expandLombok(cls *ClassInfo) {
	anns := cls.Annotations
	if len(anns) == 0 {
		// Still check field-level annotations
		hasAnyFieldLombok := false
		for _, f := range cls.Fields {
			if hasLombokAnnotation(f.Annotations, "Getter") || hasLombokAnnotation(f.Annotations, "Setter") {
				hasAnyFieldLombok = true
				break
			}
		}
		if !hasAnyFieldLombok {
			return
		}
	}

	hasData := hasLombokAnnotation(anns, "Data")
	hasValue := hasLombokAnnotation(anns, "Value")
	hasClassGetter := hasLombokAnnotation(anns, "Getter") || hasData || hasValue
	hasClassSetter := (hasLombokAnnotation(anns, "Setter") || hasData) && !hasValue
	hasToString := hasLombokAnnotation(anns, "ToString") || hasData || hasValue
	hasEqHash := hasLombokAnnotation(anns, "EqualsAndHashCode") || hasData || hasValue
	hasAllArgs := hasLombokAnnotation(anns, "AllArgsConstructor") || hasValue
	hasNoArgs := hasLombokAnnotation(anns, "NoArgsConstructor")
	hasReqArgs := hasLombokAnnotation(anns, "RequiredArgsConstructor") || hasData
	hasBuilder := hasLombokAnnotation(anns, "Builder") || hasLombokAnnotation(anns, "SuperBuilder")

	fp := cls.FilePath
	sl := cls.StartLine

	// ── Getters and Setters ──────────────────────────────────────────────────
	for _, f := range cls.Fields {
		if f.IsStatic {
			continue
		}
		if hasClassGetter || hasLombokAnnotation(f.Annotations, "Getter") {
			gName := lombokGetterName(f.Name, f.Type)
			if !hasMethod(cls.Methods, gName) {
				cls.Methods = append(cls.Methods, lombokGeneratedMethod(
					gName, f.Type,
					fmt.Sprintf("public %s %s()", f.Type, gName),
					fp, sl))
			}
		}
		if !f.IsFinal && (hasClassSetter || hasLombokAnnotation(f.Annotations, "Setter")) {
			sName := lombokSetterName(f.Name)
			if !hasMethod(cls.Methods, sName) {
				cls.Methods = append(cls.Methods, lombokGeneratedMethod(
					sName, "void",
					fmt.Sprintf("public void %s(%s %s)", sName, f.Type, f.Name),
					fp, sl))
			}
		}
	}

	// ── toString() ───────────────────────────────────────────────────────────
	if hasToString && !hasMethod(cls.Methods, "toString") {
		cls.Methods = append(cls.Methods, lombokGeneratedMethod(
			"toString", "String", "public String toString()", fp, sl))
	}

	// ── equals() and hashCode() ──────────────────────────────────────────────
	if hasEqHash {
		if !hasMethod(cls.Methods, "equals") {
			cls.Methods = append(cls.Methods, MethodInfo{
				Name:           "equals",
				Signature:      "public boolean equals(Object o)",
				ReturnType:     "boolean",
				AccessModifier: "public",
				Parameters:     []codetypes.ParamInfo{{Name: "o", Type: "Object"}},
				FilePath:       fp,
				StartLine:      sl,
				Annotations:    []string{"@lombok.Generated"},
				Code:           "// Lombok-generated",
			})
		}
		if !hasMethod(cls.Methods, "hashCode") {
			cls.Methods = append(cls.Methods, lombokGeneratedMethod(
				"hashCode", "int", "public int hashCode()", fp, sl))
		}
	}

	// ── Builder ──────────────────────────────────────────────────────────────
	if hasBuilder && !hasMethod(cls.Methods, "builder") {
		builderType := cls.Name + "Builder"
		cls.Methods = append(cls.Methods, MethodInfo{
			Name:           "builder",
			Signature:      fmt.Sprintf("public static %s builder()", builderType),
			ReturnType:     builderType,
			AccessModifier: "public",
			IsStatic:       true,
			FilePath:       fp,
			StartLine:      sl,
			Annotations:    []string{"@lombok.Generated"},
			Code:           "// Lombok-generated",
		})
	}

	// ── Constructors ─────────────────────────────────────────────────────────
	// Only inject when no constructors were found in source.
	if len(cls.Constructors) > 0 {
		return
	}

	var nonStaticFields, requiredFields []FieldInfo
	for _, f := range cls.Fields {
		if f.IsStatic {
			continue
		}
		nonStaticFields = append(nonStaticFields, f)
		if (f.IsFinal && f.DefaultValue == "") || hasLombokAnnotation(f.Annotations, "NonNull") {
			requiredFields = append(requiredFields, f)
		}
	}

	if hasAllArgs {
		cls.Constructors = append(cls.Constructors, buildLombokCtor(cls.Name, nonStaticFields, fp, sl))
	} else if hasNoArgs {
		cls.Constructors = append(cls.Constructors, buildLombokCtor(cls.Name, nil, fp, sl))
	} else if hasReqArgs {
		cls.Constructors = append(cls.Constructors, buildLombokCtor(cls.Name, requiredFields, fp, sl))
	}
}

func (a *CodeAnalyzer) shouldSkipFile(path string) bool {
	if a.includeTests {
		return false
	}
	normalized := filepath.ToSlash(path)
	if strings.Contains(normalized, "/src/test/") || strings.Contains(normalized, "/test/java/") {
		return true
	}
	base := filepath.Base(path)
	for _, suffix := range []string{
		"Test.java", "Tests.java", "IT.java", "ITCase.java",
		"Spec.java", "TestCase.java", "TestSuite.java",
	} {
		if strings.HasSuffix(base, suffix) {
			return true
		}
	}
	return false
}

func shouldSkipDir(name string) bool {
	for _, skip := range []string{
		"target", "build", ".gradle", ".maven",
		"node_modules", ".git", "__pycache__",
		".vscode", ".idea", "out",
	} {
		if name == skip {
			return true
		}
	}
	return false
}
