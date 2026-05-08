package java

import "github.com/homiodev/rag-code-mcp/internal/codetypes"

// ModuleInfo contains comprehensive information about a Java module/package
type ModuleInfo struct {
	Name        string           `json:"name"`
	Path        string           `json:"path"`
	Description string           `json:"description"`
	Classes     []ClassInfo      `json:"classes"`
	Interfaces  []ClassInfo      `json:"interfaces"`
	Enums       []EnumInfo       `json:"enums"`
	Annotations []AnnotationInfo `json:"annotations"`
	Imports     []string         `json:"imports"`
}

// ClassInfo describes a Java class, interface, or enum
type ClassInfo struct {
	Name           string            `json:"name"`
	FullyQualified string            `json:"fully_qualified"`
	Kind           string            `json:"kind"` // class, interface, record, enum
	Description    string            `json:"description"`
	AccessModifier string            `json:"access_modifier"` // public, protected, private, package-private
	IsAbstract     bool              `json:"is_abstract"`
	IsFinal        bool              `json:"is_final"`
	IsStatic       bool              `json:"is_static"`
	IsSealed       bool              `json:"is_sealed"`
	Superclass     string            `json:"superclass,omitempty"`
	Interfaces     []string          `json:"interfaces,omitempty"`
	TypeParameters []TypeParameter   `json:"type_parameters,omitempty"` // Generics
	Fields         []FieldInfo       `json:"fields"`
	Methods        []MethodInfo      `json:"methods"`
	Constructors   []ConstructorInfo `json:"constructors"`
	Annotations    []string          `json:"annotations,omitempty"`
	InnerClasses   []ClassInfo       `json:"inner_classes,omitempty"`
	FilePath       string            `json:"file_path,omitempty"`
	StartLine      int               `json:"start_line,omitempty"`
	EndLine        int               `json:"end_line,omitempty"`
	Code           string            `json:"code,omitempty"`
}

// FieldInfo describes a class field/member variable
type FieldInfo struct {
	Name           string   `json:"name"`
	Type           string   `json:"type"`
	AccessModifier string   `json:"access_modifier"`
	IsStatic       bool     `json:"is_static"`
	IsFinal        bool     `json:"is_final"`
	IsTransient    bool     `json:"is_transient"`
	IsVolatile     bool     `json:"is_volatile"`
	DefaultValue   string   `json:"default_value,omitempty"`
	Annotations    []string `json:"annotations,omitempty"`
	Description    string   `json:"description"`
	FilePath       string   `json:"file_path,omitempty"`
	StartLine      int      `json:"start_line,omitempty"`
}

// MethodInfo describes a class method
type MethodInfo struct {
	Name             string                 `json:"name"`
	Signature        string                 `json:"signature"`
	ReturnType       string                 `json:"return_type"`
	AccessModifier   string                 `json:"access_modifier"`
	IsAbstract       bool                   `json:"is_abstract"`
	IsStatic         bool                   `json:"is_static"`
	IsFinal          bool                   `json:"is_final"`
	IsSynchronized   bool                   `json:"is_synchronized"`
	IsNative         bool                   `json:"is_native"`
	Parameters       []codetypes.ParamInfo  `json:"parameters"`
	ReturnTypes      []codetypes.ReturnInfo `json:"return_types"`
	TypeParameters   []TypeParameter        `json:"type_parameters,omitempty"` // Generics
	ThrowsExceptions []string               `json:"throws_exceptions,omitempty"`
	Annotations      []string               `json:"annotations,omitempty"`
	Description      string                 `json:"description"`
	FilePath         string                 `json:"file_path,omitempty"`
	StartLine        int                    `json:"start_line,omitempty"`
	EndLine          int                    `json:"end_line,omitempty"`
	Code             string                 `json:"code,omitempty"`
}

// ConstructorInfo describes a class constructor
type ConstructorInfo struct {
	Name             string                `json:"name"`
	Signature        string                `json:"signature"`
	AccessModifier   string                `json:"access_modifier"`
	Parameters       []codetypes.ParamInfo `json:"parameters"`
	ThrowsExceptions []string              `json:"throws_exceptions,omitempty"`
	Annotations      []string              `json:"annotations,omitempty"`
	Description      string                `json:"description"`
	FilePath         string                `json:"file_path,omitempty"`
	StartLine        int                   `json:"start_line,omitempty"`
	EndLine          int                   `json:"end_line,omitempty"`
	Code             string                `json:"code,omitempty"`
}

// EnumInfo describes a Java enum
type EnumInfo struct {
	Name           string         `json:"name"`
	FullyQualified string         `json:"fully_qualified"`
	Description    string         `json:"description"`
	AccessModifier string         `json:"access_modifier"`
	Constants      []EnumConstant `json:"constants"`
	Methods        []MethodInfo   `json:"methods"`
	Interfaces     []string       `json:"interfaces,omitempty"`
	Annotations    []string       `json:"annotations,omitempty"`
	FilePath       string         `json:"file_path,omitempty"`
	StartLine      int            `json:"start_line,omitempty"`
	EndLine        int            `json:"end_line,omitempty"`
	Code           string         `json:"code,omitempty"`
}

// EnumConstant represents an enum constant
type EnumConstant struct {
	Name        string   `json:"name"`
	Value       string   `json:"value,omitempty"`
	Description string   `json:"description"`
	Annotations []string `json:"annotations,omitempty"`
}

// AnnotationInfo describes a Java annotation
type AnnotationInfo struct {
	Name        string              `json:"name"`
	Elements    []AnnotationElement `json:"elements"`
	Retention   string              `json:"retention,omitempty"`
	Target      []string            `json:"target,omitempty"`
	Description string              `json:"description"`
	FilePath    string              `json:"file_path,omitempty"`
	StartLine   int                 `json:"start_line,omitempty"`
}

// AnnotationElement represents an annotation element
type AnnotationElement struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	DefaultValue string `json:"default_value,omitempty"`
}

// TypeParameter represents a generic type parameter (e.g., T, K extends Comparable)
type TypeParameter struct {
	Name   string   `json:"name"`
	Bounds []string `json:"bounds,omitempty"` // Upper bounds for type parameter
}
