package java

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/homiodev/rag-code-mcp/internal/codetypes"
)

// CodeAnalyzer implements PathAnalyzer for Java code
type CodeAnalyzer struct {
	includeTests bool
}

// NewCodeAnalyzer creates a new Java code analyzer (excludes test files by default)
func NewCodeAnalyzer() *CodeAnalyzer {
	return &CodeAnalyzer{
		includeTests: false,
	}
}

// NewCodeAnalyzerWithOptions creates a Java analyzer with custom options
func NewCodeAnalyzerWithOptions(includeTests bool) *CodeAnalyzer {
	return &CodeAnalyzer{
		includeTests: includeTests,
	}
}

// AnalyzePaths analyzes Java files in the given paths
func (a *CodeAnalyzer) AnalyzePaths(paths []string) ([]codetypes.CodeChunk, error) {
	var chunks []codetypes.CodeChunk

	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}

		if info.IsDir() {
			dirChunks, err := a.analyzeDirectory(path)
			if err == nil {
				chunks = append(chunks, dirChunks...)
			}
		} else if strings.HasSuffix(path, ".java") {
			if !a.shouldSkipFile(path) {
				fileChunks, err := a.analyzeFile(path)
				if err == nil {
					chunks = append(chunks, fileChunks...)
				}
			}
		}
	}

	return chunks, nil
}

// analyzeDirectory recursively analyzes Java files in a directory
func (a *CodeAnalyzer) analyzeDirectory(dirPath string) ([]codetypes.CodeChunk, error) {
	var chunks []codetypes.CodeChunk

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			// Skip common directories
			if shouldSkipDir(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		if strings.HasSuffix(path, ".java") && !a.shouldSkipFile(path) {
			if fileChunks, err := a.analyzeFile(path); err == nil {
				chunks = append(chunks, fileChunks...)
			}
		}

		return nil
	})

	return chunks, err
}

// analyzeFile analyzes a single Java file
func (a *CodeAnalyzer) analyzeFile(filePath string) ([]codetypes.CodeChunk, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	fileContent := string(content)
	var chunks []codetypes.CodeChunk

	// Extract package name
	packageName := extractPackageName(fileContent)

	// Extract classes
	classes := a.extractClasses(fileContent, filePath, packageName)
	for _, class := range classes {
		chunks = append(chunks, a.classToChunks(class, packageName)...)
	}

	// Extract interfaces
	interfaces := a.extractInterfaces(fileContent, filePath, packageName)
	for _, iface := range interfaces {
		chunks = append(chunks, a.interfaceToChunks(iface, packageName)...)
	}

	// Extract enums
	enums := a.extractEnums(fileContent, filePath, packageName)
	for _, enum := range enums {
		chunks = append(chunks, a.enumToChunks(enum, packageName)...)
	}

	return chunks, nil
}

// extractClasses extracts all class definitions from Java source
func (a *CodeAnalyzer) extractClasses(content, filePath, packageName string) []ClassInfo {
	var classes []ClassInfo

	// Match class declarations with modifiers
	classPattern := regexp.MustCompile(`(?m)^\s*(public\s+)?(abstract\s+)?(final\s+)?(static\s+)?class\s+([A-Z][a-zA-Z0-9]*)\s*(?:<[^>]+>)?\s*(?:extends\s+([a-zA-Z0-9.]+))?\s*(?:implements\s+([^{]+))?\{`)

	matches := classPattern.FindAllStringSubmatchIndex(content, -1)
	for _, match := range matches {
		class := a.parseClassDefinition(content, match, filePath, packageName)
		if class != nil {
			classes = append(classes, *class)
		}
	}

	return classes
}

// extractInterfaces extracts all interface definitions
func (a *CodeAnalyzer) extractInterfaces(content, filePath, packageName string) []ClassInfo {
	var interfaces []ClassInfo

	interfacePattern := regexp.MustCompile(`(?m)^\s*(public\s+)?(abstract\s+)?interface\s+([A-Z][a-zA-Z0-9]*)\s*(?:<[^>]+>)?\s*(?:extends\s+([^{]+))?\{`)

	matches := interfacePattern.FindAllStringSubmatchIndex(content, -1)
	for _, match := range matches {
		iface := a.parseInterfaceDefinition(content, match, filePath, packageName)
		if iface != nil {
			interfaces = append(interfaces, *iface)
		}
	}

	return interfaces
}

// extractEnums extracts all enum definitions
func (a *CodeAnalyzer) extractEnums(content, filePath, packageName string) []EnumInfo {
	var enums []EnumInfo

	enumPattern := regexp.MustCompile(`(?m)^\s*(public\s+)?enum\s+([A-Z][a-zA-Z0-9]*)\s*(?:implements\s+([^{]+))?\{`)

	matches := enumPattern.FindAllStringSubmatchIndex(content, -1)
	for _, match := range matches {
		enum := a.parseEnumDefinition(content, match, filePath, packageName)
		if enum != nil {
			enums = append(enums, *enum)
		}
	}

	return enums
}

// parseClassDefinition parses a class definition from regex match
func (a *CodeAnalyzer) parseClassDefinition(content string, match []int, filePath, packageName string) *ClassInfo {
	classText := content[match[0]:match[1]]

	// Extract class name (last capturing group should be the name)
	namePattern := regexp.MustCompile(`class\s+([A-Z][a-zA-Z0-9]*)`)
	nameMatch := namePattern.FindStringSubmatch(classText)
	if len(nameMatch) < 2 {
		return nil
	}

	className := nameMatch[1]

	class := &ClassInfo{
		Name:           className,
		FullyQualified: packageName + "." + className,
		Kind:           "class",
		FilePath:       filePath,
		IsAbstract:     strings.Contains(classText, "abstract"),
		IsFinal:        strings.Contains(classText, "final"),
		IsStatic:       strings.Contains(classText, "static"),
		AccessModifier: extractAccessModifier(classText),
	}

	// Extract extends
	extendsPattern := regexp.MustCompile(`extends\s+([a-zA-Z0-9.<>]+)`)
	if extendsMatch := extendsPattern.FindStringSubmatch(classText); len(extendsMatch) > 1 {
		class.Superclass = strings.TrimSpace(extendsMatch[1])
	}

	// Extract implements
	implementsPattern := regexp.MustCompile(`implements\s+([^{]+)`)
	if implMatch := implementsPattern.FindStringSubmatch(classText); len(implMatch) > 1 {
		interfaces := strings.Split(implMatch[1], ",")
		for _, iface := range interfaces {
			class.Interfaces = append(class.Interfaces, strings.TrimSpace(iface))
		}
	}

	// Extract JavaDoc (simple approach)
	class.Description = extractJavaDoc(content, match[0])

	// Extract class body
	startBrace := match[1] - 1
	class.StartLine = strings.Count(content[:match[0]], "\n") + 1

	// Find matching closing brace
	endBrace := findMatchingBrace(content, startBrace)
	if endBrace > 0 {
		class.EndLine = strings.Count(content[:endBrace], "\n") + 1
		class.Code = content[match[0] : endBrace+1]

		// Extract methods
		methodContent := content[match[1]:endBrace]
		class.Methods = a.extractMethods(methodContent, filePath, className)

		// Extract fields
		class.Fields = a.extractFields(methodContent, filePath)

		// Extract constructors
		class.Constructors = a.extractConstructors(methodContent, filePath, className)
	}

	return class
}

// parseInterfaceDefinition parses an interface definition
func (a *CodeAnalyzer) parseInterfaceDefinition(content string, match []int, filePath, packageName string) *ClassInfo {
	interfaceText := content[match[0]:match[1]]

	namePattern := regexp.MustCompile(`interface\s+([A-Z][a-zA-Z0-9]*)`)
	nameMatch := namePattern.FindStringSubmatch(interfaceText)
	if len(nameMatch) < 2 {
		return nil
	}

	name := nameMatch[1]

	iface := &ClassInfo{
		Name:           name,
		FullyQualified: packageName + "." + name,
		Kind:           "interface",
		FilePath:       filePath,
		AccessModifier: extractAccessModifier(interfaceText),
		StartLine:      strings.Count(content[:match[0]], "\n") + 1,
	}

	// Extract extends
	extendsPattern := regexp.MustCompile(`extends\s+([^{]+)`)
	if extendsMatch := extendsPattern.FindStringSubmatch(interfaceText); len(extendsMatch) > 1 {
		interfaces := strings.Split(extendsMatch[1], ",")
		for _, i := range interfaces {
			iface.Interfaces = append(iface.Interfaces, strings.TrimSpace(i))
		}
	}

	iface.Description = extractJavaDoc(content, match[0])

	startBrace := match[1] - 1
	endBrace := findMatchingBrace(content, startBrace)
	if endBrace > 0 {
		iface.EndLine = strings.Count(content[:endBrace], "\n") + 1
		iface.Code = content[match[0] : endBrace+1]

		methodContent := content[match[1]:endBrace]
		iface.Methods = a.extractMethods(methodContent, filePath, "")
	}

	return iface
}

// parseEnumDefinition parses an enum definition
func (a *CodeAnalyzer) parseEnumDefinition(content string, match []int, filePath, packageName string) *EnumInfo {
	enumText := content[match[0]:match[1]]

	namePattern := regexp.MustCompile(`enum\s+([A-Z][a-zA-Z0-9]*)`)
	nameMatch := namePattern.FindStringSubmatch(enumText)
	if len(nameMatch) < 2 {
		return nil
	}

	name := nameMatch[1]

	enum := &EnumInfo{
		Name:           name,
		FullyQualified: packageName + "." + name,
		FilePath:       filePath,
		AccessModifier: extractAccessModifier(enumText),
		StartLine:      strings.Count(content[:match[0]], "\n") + 1,
	}

	enum.Description = extractJavaDoc(content, match[0])

	startBrace := match[1] - 1
	endBrace := findMatchingBrace(content, startBrace)
	if endBrace > 0 {
		enum.EndLine = strings.Count(content[:endBrace], "\n") + 1
		enum.Code = content[match[0] : endBrace+1]

		enumBody := content[match[1]:endBrace]

		// Extract constants
		enum.Constants = a.extractEnumConstants(enumBody)

		// Extract methods
		enum.Methods = a.extractMethods(enumBody, filePath, name)
	}

	return enum
}

// extractMethods extracts all methods from class/interface body
func (a *CodeAnalyzer) extractMethods(content, filePath, className string) []MethodInfo {
	var methods []MethodInfo

	// Match method signatures
	methodPattern := regexp.MustCompile(`(?m)^\s*(public|protected|private)?\s*(abstract\s+)?(static\s+)?(final\s+)?(synchronized\s+)?(native\s+)?([a-zA-Z0-9<>?,\s]+)\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*\(([^)]*)\)\s*(?:throws\s+([^{]+))?\s*[{;]`)

	matches := methodPattern.FindAllStringSubmatchIndex(content, -1)
	for _, match := range matches {
		method := a.parseMethod(content, match, filePath)
		if method != nil {
			methods = append(methods, *method)
		}
	}

	return methods
}

// parseMethod parses a single method signature
func (a *CodeAnalyzer) parseMethod(content string, match []int, filePath string) *MethodInfo {
	methodText := content[match[0]:match[1]]

	// Extract method name (second to last word before parentheses)
	namePattern := regexp.MustCompile(`([a-zA-Z_][a-zA-Z0-9_]*)\s*\(([^)]*)\)`)
	nameMatch := namePattern.FindStringSubmatch(methodText)
	if len(nameMatch) < 2 {
		return nil
	}

	methodName := nameMatch[1]
	paramsStr := nameMatch[2]

	method := &MethodInfo{
		Name:           methodName,
		Signature:      methodText,
		FilePath:       filePath,
		AccessModifier: extractAccessModifier(methodText),
		IsAbstract:     strings.Contains(methodText, "abstract"),
		IsStatic:       strings.Contains(methodText, "static"),
		IsFinal:        strings.Contains(methodText, "final"),
		IsSynchronized: strings.Contains(methodText, "synchronized"),
		IsNative:       strings.Contains(methodText, "native"),
		StartLine:      strings.Count(content[:match[0]], "\n") + 1,
	}

	// Extract return type
	returnTypePattern := regexp.MustCompile(`(public|protected|private)?\s*(abstract\s+)?(static\s+)?(final\s+)?(synchronized\s+)?(native\s+)?([a-zA-Z0-9<>?,\s]+)\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*\(`)
	if rtMatch := returnTypePattern.FindStringSubmatch(methodText); len(rtMatch) > 7 {
		method.ReturnType = strings.TrimSpace(rtMatch[7])
	}

	// Parse parameters
	method.Parameters = a.parseParameters(paramsStr)

	return method
}

// extractFields extracts all fields from class body
func (a *CodeAnalyzer) extractFields(content, filePath string) []FieldInfo {
	var fields []FieldInfo

	// Match field declarations
	fieldPattern := regexp.MustCompile(`(?m)^\s*(public|protected|private)?\s*(static\s+)?(final\s+)?(transient\s+)?(volatile\s+)?([a-zA-Z0-9<>?,\s]+)\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*(?:=\s*([^;]+))?;`)

	matches := fieldPattern.FindAllStringSubmatchIndex(content, -1)
	for _, match := range matches {
		fieldText := content[match[0]:match[1]]

		namePattern := regexp.MustCompile(`([a-zA-Z_][a-zA-Z0-9_]*)\s*(?:=|;)`)
		nameMatch := namePattern.FindStringSubmatch(fieldText)
		if len(nameMatch) < 2 {
			continue
		}

		fieldName := nameMatch[1]

		field := &FieldInfo{
			Name:           fieldName,
			FilePath:       filePath,
			AccessModifier: extractAccessModifier(fieldText),
			IsStatic:       strings.Contains(fieldText, "static"),
			IsFinal:        strings.Contains(fieldText, "final"),
			IsTransient:    strings.Contains(fieldText, "transient"),
			IsVolatile:     strings.Contains(fieldText, "volatile"),
			StartLine:      strings.Count(content[:match[0]], "\n") + 1,
		}

		// Extract type
		typePattern := regexp.MustCompile(`(public|protected|private)?\s*(static\s+)?(final\s+)?(transient\s+)?(volatile\s+)?([a-zA-Z0-9<>?,\s]+)\s+([a-zA-Z_][a-zA-Z0-9_]*)`)
		if typeMatch := typePattern.FindStringSubmatch(fieldText); len(typeMatch) > 6 {
			field.Type = strings.TrimSpace(typeMatch[6])
		}

		fields = append(fields, field)
	}

	return fields
}

// extractConstructors extracts all constructors from class body
func (a *CodeAnalyzer) extractConstructors(content, filePath, className string) []ConstructorInfo {
	var constructors []ConstructorInfo

	// Match constructor declarations (same name as class)
	ctorPattern := regexp.MustCompile(fmt.Sprintf(`(?m)^\s*(public|protected|private)?\s*%s\s*\(([^)]*)\)\s*(?:throws\s+([^{]+))?\s*\{`, className))

	matches := ctorPattern.FindAllStringSubmatchIndex(content, -1)
	for _, match := range matches {
		ctorText := content[match[0]:match[1]]

		ctor := &ConstructorInfo{
			Name:           className,
			Signature:      ctorText,
			FilePath:       filePath,
			AccessModifier: extractAccessModifier(ctorText),
			StartLine:      strings.Count(content[:match[0]], "\n") + 1,
		}

		// Parse parameters
		paramsPattern := regexp.MustCompile(`\(([^)]*)\)`)
		if paramsMatch := paramsPattern.FindStringSubmatch(ctorText); len(paramsMatch) > 1 {
			ctor.Parameters = a.parseParameters(paramsMatch[1])
		}

		constructors = append(constructors, ctor)
	}

	return constructors
}

// extractEnumConstants extracts enum constants
func (a *CodeAnalyzer) extractEnumConstants(content string) []EnumConstant {
	var constants []EnumConstant

	// Split by semicolon to find constants section
	parts := strings.Split(content, ";")
	if len(parts) == 0 {
		return constants
	}

	constantsSection := parts[0]

	// Split by comma
	lines := strings.Split(constantsSection, ",")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Extract constant name (first word)
		parts := strings.Fields(line)
		if len(parts) > 0 {
			constant := EnumConstant{
				Name:  parts[0],
				Value: line,
			}
			constants = append(constants, constant)
		}
	}

	return constants
}

// parseParameters parses method/constructor parameters
func (a *CodeAnalyzer) parseParameters(paramsStr string) []codetypes.ParamInfo {
	var params []codetypes.ParamInfo

	if strings.TrimSpace(paramsStr) == "" {
		return params
	}

	// Handle generic brackets properly
	paramsList := smartSplit(paramsStr, ",")

	for _, param := range paramsList {
		param = strings.TrimSpace(param)
		if param == "" {
			continue
		}

		// Extract type and name
		parts := strings.Fields(param)
		if len(parts) >= 2 {
			paramName := parts[len(parts)-1]
			paramType := strings.Join(parts[:len(parts)-1], " ")

			params = append(params, codetypes.ParamInfo{
				Name: paramName,
				Type: paramType,
			})
		}
	}

	return params
}

// classToChunks converts a class to CodeChunks
func (a *CodeAnalyzer) classToChunks(class ClassInfo, packageName string) []codetypes.CodeChunk {
	var chunks []codetypes.CodeChunk

	// Class declaration chunk
	chunk := codetypes.CodeChunk{
		Type:        "class",
		Name:        class.Name,
		Package:     packageName,
		Description: class.Description,
		FilePath:    class.FilePath,
		StartLine:   class.StartLine,
		EndLine:     class.EndLine,
		Code:        class.Code,
		Metadata: map[string]interface{}{
			"access_modifier": class.AccessModifier,
			"is_abstract":     class.IsAbstract,
			"is_final":        class.IsFinal,
			"is_static":       class.IsStatic,
			"superclass":      class.Superclass,
			"interfaces":      class.Interfaces,
		},
	}
	chunks = append(chunks, chunk)

	// Method chunks
	for _, method := range class.Methods {
		methodChunk := codetypes.CodeChunk{
			Type:        "method",
			Name:        method.Name,
			Package:     packageName,
			Description: method.Description,
			FilePath:    method.FilePath,
			StartLine:   method.StartLine,
			Code:        method.Signature,
			Metadata: map[string]interface{}{
				"class_name":      class.Name,
				"access_modifier": method.AccessModifier,
				"is_abstract":     method.IsAbstract,
				"is_static":       method.IsStatic,
				"is_final":        method.IsFinal,
				"is_synchronized": method.IsSynchronized,
				"return_type":     method.ReturnType,
			},
		}
		chunks = append(chunks, methodChunk)
	}

	// Field chunks
	for _, field := range class.Fields {
		fieldChunk := codetypes.CodeChunk{
			Type:        "field",
			Name:        field.Name,
			Package:     packageName,
			Description: field.Description,
			FilePath:    field.FilePath,
			StartLine:   field.StartLine,
			Metadata: map[string]interface{}{
				"class_name":      class.Name,
				"access_modifier": field.AccessModifier,
				"is_static":       field.IsStatic,
				"is_final":        field.IsFinal,
				"type":            field.Type,
			},
		}
		chunks = append(chunks, fieldChunk)
	}

	return chunks
}

// interfaceToChunks converts an interface to CodeChunks
func (a *CodeAnalyzer) interfaceToChunks(iface ClassInfo, packageName string) []codetypes.CodeChunk {
	var chunks []codetypes.CodeChunk

	chunk := codetypes.CodeChunk{
		Type:        "interface",
		Name:        iface.Name,
		Package:     packageName,
		Description: iface.Description,
		FilePath:    iface.FilePath,
		StartLine:   iface.StartLine,
		EndLine:     iface.EndLine,
		Code:        iface.Code,
		Metadata: map[string]interface{}{
			"access_modifier": iface.AccessModifier,
			"extends":         iface.Interfaces,
		},
	}
	chunks = append(chunks, chunk)

	// Method chunks
	for _, method := range iface.Methods {
		methodChunk := codetypes.CodeChunk{
			Type:        "method",
			Name:        method.Name,
			Package:     packageName,
			Description: method.Description,
			FilePath:    method.FilePath,
			StartLine:   method.StartLine,
			Code:        method.Signature,
			Metadata: map[string]interface{}{
				"interface_name": iface.Name,
				"return_type":    method.ReturnType,
			},
		}
		chunks = append(chunks, methodChunk)
	}

	return chunks
}

// enumToChunks converts an enum to CodeChunks
func (a *CodeAnalyzer) enumToChunks(enum EnumInfo, packageName string) []codetypes.CodeChunk {
	var chunks []codetypes.CodeChunk

	chunk := codetypes.CodeChunk{
		Type:        "enum",
		Name:        enum.Name,
		Package:     packageName,
		Description: enum.Description,
		FilePath:    enum.FilePath,
		StartLine:   enum.StartLine,
		EndLine:     enum.EndLine,
		Code:        enum.Code,
		Metadata: map[string]interface{}{
			"access_modifier": enum.AccessModifier,
			"constants":       len(enum.Constants),
		},
	}
	chunks = append(chunks, chunk)

	// Constant chunks
	for _, constant := range enum.Constants {
		constChunk := codetypes.CodeChunk{
			Type:    "enum_constant",
			Name:    constant.Name,
			Package: packageName,
			Metadata: map[string]interface{}{
				"enum_name": enum.Name,
			},
		}
		chunks = append(chunks, constChunk)
	}

	return chunks
}

// shouldSkipFile checks if a file should be skipped
func (a *CodeAnalyzer) shouldSkipFile(path string) bool {
	if !a.includeTests {
		filename := filepath.Base(path)
		if strings.HasSuffix(filename, "Test.java") || strings.HasSuffix(filename, "Tests.java") {
			return true
		}
	}
	return false
}

// shouldSkipDir checks if a directory should be skipped
func shouldSkipDir(dirName string) bool {
	skipDirs := []string{
		"target", "build", ".gradle", ".maven",
		"node_modules", ".git", "__pycache__",
		".vscode", ".idea", "out",
	}

	for _, skip := range skipDirs {
		if dirName == skip {
			return true
		}
	}
	return false
}

// Helper functions

func extractPackageName(content string) string {
	pattern := regexp.MustCompile(`(?m)^\s*package\s+([a-zA-Z0-9_.]+);`)
	if match := pattern.FindStringSubmatch(content); len(match) > 1 {
		return match[1]
	}
	return "default"
}

func extractAccessModifier(text string) string {
	if strings.Contains(text, "public") {
		return "public"
	} else if strings.Contains(text, "protected") {
		return "protected"
	} else if strings.Contains(text, "private") {
		return "private"
	}
	return "package-private"
}

func extractJavaDoc(content string, position int) string {
	// Look for /** ... */ before the position
	start := strings.LastIndex(content[:position], "/**")
	if start == -1 {
		return ""
	}

	end := strings.Index(content[start:], "*/")
	if end == -1 {
		return ""
	}

	docBlock := content[start : start+end+2]

	// Clean up JavaDoc markers
	lines := strings.Split(docBlock, "\n")
	var cleanedLines []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "/**")
		line = strings.TrimSuffix(line, "*/")
		line = strings.TrimPrefix(line, "*")
		line = strings.TrimSpace(line)

		if line != "" {
			cleanedLines = append(cleanedLines, line)
		}
	}

	return strings.Join(cleanedLines, " ")
}

func findMatchingBrace(content string, openBracePos int) int {
	count := 1
	for i := openBracePos + 1; i < len(content); i++ {
		if content[i] == '{' {
			count++
		} else if content[i] == '}' {
			count--
			if count == 0 {
				return i
			}
		}
	}
	return -1
}

func smartSplit(s, sep string) []string {
	var result []string
	var current strings.Builder
	var bracketDepth int

	for _, char := range s {
		switch char {
		case '<':
			bracketDepth++
			current.WriteRune(char)
		case '>':
			bracketDepth--
			current.WriteRune(char)
		case rune(sep[0]):
			if bracketDepth == 0 {
				result = append(result, current.String())
				current.Reset()
			} else {
				current.WriteRune(char)
			}
		default:
			current.WriteRune(char)
		}
	}

	if current.Len() > 0 {
		result = append(result, current.String())
	}

	return result
}
