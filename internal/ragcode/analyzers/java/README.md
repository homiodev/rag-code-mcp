# Java Code Analyzer

Comprehensive code analyzer for extracting symbols, structure, and relationships from Java files. Indexes code for semantic search in Qdrant.

## Status: ✅ PRODUCTION READY

---

## 🎯 What This Analyzer Does

The Java analyzer parses `.java` files and extracts:
1. **Symbols** - classes, interfaces, methods, fields, constructors, enums, annotations
2. **Relationships** - inheritance, interface implementation, dependencies, method calls
3. **Metadata** - access modifiers, generics, annotations, documentation
4. **Documentation** - JavaDoc comments for all symbols

Information is converted to `CodeChunk`s which are then indexed in Qdrant for semantic search.

---

## 📊 Data Flow

```
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│   .java Files   │────▶│  Java Analyzer   │────▶│   CodeChunks    │
│  (source code)  │     │  (regex parsing) │     │  (structured)   │
└─────────────────┘     └──────────────────┘     └────────┬────────┘
                                                          │
                                                          ▼
                                                 ┌─────────────────┐
                                                 │     Qdrant      │
                                                 │  (vector store) │
                                                 └─────────────────┘
```

---

## 🔍 What We Index

### 1. Classes (`type: "class"`)

```java
/**
 * Represents a user in the system.
 * Handles authentication and profile management.
 */
public abstract class User extends BaseEntity implements Comparable<User> {
    private String username;
    protected String email;
    
    public User(String username) {
        this.username = username;
    }
}
```

**Extracted information:**
| Field | Value | Description |
|-------|-------|-------------|
| `name` | `"User"` | Class name |
| `fully_qualified` | `"com.example.User"` | Fully qualified name |
| `kind` | `"class"` | Type of declaration |
| `access_modifier` | `"public"` | Access level |
| `is_abstract` | `true` | If it's abstract |
| `is_final` | `false` | If it's final |
| `superclass` | `"BaseEntity"` | Parent class |
| `interfaces` | `["Comparable<User>"]` | Implemented interfaces |
| `description` | `"Represents a user..."` | JavaDoc documentation |

### 2. Interfaces (`type: "interface"`)

```java
/**
 * Contract for repository operations.
 */
public interface Repository<T> {
    /**
     * Finds an entity by ID.
     * @param id the entity ID
     * @return the found entity or null
     */
    T findById(Long id);
    
    /**
     * Saves an entity.
     * @param entity the entity to save
     * @return the saved entity
     */
    T save(T entity);
}
```

**Extracted information:**
| Field | Value | Description |
|-------|-------|-------------|
| `name` | `"Repository"` | Interface name |
| `kind` | `"interface"` | Type of declaration |
| `access_modifier` | `"public"` | Access level |
| `extends` | `[]` | Superinterfaces |
| `type_parameters` | `["T"]` | Generic type parameters |

### 3. Methods (`type: "method"`)

```java
/**
 * Authenticates a user with the given credentials.
 * @param username the username
 * @param password the password
 * @return the authenticated user
 * @throws AuthenticationException if credentials are invalid
 */
public synchronized static User authenticate(String username, String password) 
        throws AuthenticationException {
    // implementation
}
```

**Extracted information:**
| Field | Value | Description |
|-------|-------|-------------|
| `name` | `"authenticate"` | Method name |
| `signature` | `"public synchronized static User authenticate(...)"` | Complete signature |
| `return_type` | `"User"` | Return type |
| `access_modifier` | `"public"` | Access level |
| `is_static` | `true` | If it's static |
| `is_abstract` | `false` | If it's abstract |
| `is_synchronized` | `true` | If it's synchronized |
| `parameters` | `[{name: "username", type: "String"}, ...]` | Method parameters |
| `throws_exceptions` | `["AuthenticationException"]` | Checked exceptions |

### 4. Constructors (`type: "constructor"`)

```java
/**
 * Creates a new User with the given username.
 * @param username the username
 * @throws IllegalArgumentException if username is empty
 */
public User(String username) throws IllegalArgumentException {
    this.username = username;
}
```

**Extracted information:**
| Field | Value | Description |
|-------|-------|-------------|
| `name` | `"User"` | Constructor name (same as class) |
| `signature` | `"public User(String username)"` | Complete signature |
| `access_modifier` | `"public"` | Access level |
| `parameters` | `[{name: "username", type: "String"}]` | Constructor parameters |
| `throws_exceptions` | `["IllegalArgumentException"]` | Checked exceptions |

### 5. Fields (`type: "field"`)

```java
/**
 * The user's unique identifier.
 */
private final String username;

/**
 * User's email address.
 */
protected volatile String email;

/**
 * Default user instance.
 */
public static final User DEFAULT = new User("default");
```

**Extracted information:**
| Field | Value | Description |
|-------|-------|-------------|
| `name` | `"username"` | Field name |
| `type` | `"String"` | Field type |
| `access_modifier` | `"private"` | Access level |
| `is_static` | `false` | If it's static |
| `is_final` | `true` | If it's final |
| `is_volatile` | `false` | If it's volatile |
| `is_transient` | `false` | If it's transient |

### 6. Enums (`type: "enum"`)

```java
/**
 * Represents user roles in the system.
 */
public enum Role {
    /** Administrator role with full permissions */
    ADMIN("admin", 10),
    
    /** User role with limited permissions */
    USER("user", 1);
    
    private final String label;
    private final int level;
    
    Role(String label, int level) {
        this.label = label;
        this.level = level;
    }
}
```

**Extracted information:**
| Field | Value | Description |
|-------|-------|-------------|
| `name` | `"Role"` | Enum name |
| `access_modifier` | `"public"` | Access level |
| `constants` | `[{name: "ADMIN", ...}, {name: "USER", ...}]` | Enum constants |
| `methods` | `[{name: "Role", ...}]` | Enum methods |

### 7. Annotations (`type: "annotation"`)

```java
/**
 * Marks a method as deprecated.
 */
@Deprecated(since = "1.2.0", forRemoval = true)
public void oldMethod() { }
```

**Extracted information:**
| Field | Value | Description |
|-------|-------|-------------|
| `name` | `"Deprecated"` | Annotation name |
| `elements` | `[{name: "since", value: "1.2.0"}, ...]` | Annotation elements |

---

## 🏗️ File Structure

```
java/
├── types.go           # Types: ModuleInfo, ClassInfo, MethodInfo, etc.
├── analyzer.go        # PathAnalyzer implementation (1800+ lines)
├── analyzer_test.go   # Comprehensive test suite
└── README.md          # This documentation
```

---

## 💻 Usage

### Standard Analysis

```go
import "github.com/homiodev/rag-code-mcp/internal/ragcode/analyzers/java"

// Create analyzer (excludes test files by default)
analyzer := java.NewCodeAnalyzer()

// Analyze directories/files
chunks, err := analyzer.AnalyzePaths([]string{"./src"})

for _, chunk := range chunks {
    fmt.Printf("[%s] %s.%s\n", chunk.Type, chunk.Package, chunk.Name)
    fmt.Printf("  File: %s (lines %d-%d)\n", chunk.FilePath, chunk.StartLine, chunk.EndLine)
}
```

### With Options

```go
// Include test files in analysis
analyzer := java.NewCodeAnalyzerWithOptions(true)

// This will analyze both src/ and src/test directories
chunks, err := analyzer.AnalyzePaths([]string{"./src"})
```

### Analyze Single File

```go
chunks, err := analyzer.AnalyzePaths([]string{"./src/main/java/com/example/User.java"})
```

---

## 🔌 Integration

### Language Manager

The Java analyzer is automatically selected for:
- `java` - generic Java projects
- `maven` - Maven projects (pom.xml)
- `gradle` - Gradle projects (build.gradle)
- `spring` - Spring Boot projects

### Workspace Detection

Java projects are detected by:
| File | Description |
|------|-------------|
| `pom.xml` | Apache Maven projects |
| `build.gradle` | Gradle projects |
| `settings.gradle` | Gradle multi-module projects |
| `src/main/java` | Standard Java directory structure |
| `.java` | Java source files |

---

## 📋 CodeChunk Types

| Type | Description | Example |
|------|-------------|----------|
| `class` | Class definition | `public class User {}` |
| `interface` | Interface definition | `public interface Repository {}` |
| `method` | Instance or static method | `public void save()` |
| `constructor` | Constructor | `public User(String name)` |
| `field` | Class field/member variable | `private String username;` |
| `enum` | Enum definition | `public enum Status {}` |
| `enum_constant` | Enum constant | `ACTIVE("active")` |
| `annotation` | Annotation definition | `@FunctionalInterface` |

---

## 🏷️ Complete Metadata

### Class Metadata
```json
{
  "access_modifier": "public",
  "is_abstract": false,
  "is_final": false,
  "is_static": false,
  "is_sealed": false,
  "superclass": "BaseEntity",
  "interfaces": ["Comparable<User>", "Serializable"],
  "type_parameters": [{"name": "T", "bounds": ["Serializable"]}]
}
```

### Method Metadata
```json
{
  "access_modifier": "public",
  "is_abstract": false,
  "is_static": false,
  "is_final": false,
  "is_synchronized": false,
  "is_native": false,
  "return_type": "User",
  "throws_exceptions": ["IOException", "SQLException"],
  "parameters": [
    {"name": "id", "type": "Long"},
    {"name": "name", "type": "String"}
  ]
}
```

### Field Metadata
```json
{
  "type": "String",
  "access_modifier": "private",
  "is_static": false,
  "is_final": true,
  "is_volatile": false,
  "is_transient": false,
  "default_value": null
}
```

---

## 🧪 Testing

```bash
# Run all Java analyzer tests
go test ./internal/ragcode/analyzers/java/

# With verbose output
go test -v ./internal/ragcode/analyzers/java/

# Specific test
go test -v -run TestClassExtraction ./internal/ragcode/analyzers/java/

# With coverage
go test -cover ./internal/ragcode/analyzers/java/
```

---

## 🚫 Excluded Paths

The analyzer automatically skips:
- `target/` - Maven build directory
- `build/` - Gradle build directory
- `.gradle/`, `.maven/` - build tool caches
- `node_modules/`, `.git/` - unrelated dependencies
- `__pycache__/` - Python cache
- `Test.java`, `Tests.java` - test files (by default)

---

## ⚠️ Limitations

| Limitation | Description |
|------------|-------------|
| **Regex-based** | Uses regex parsing instead of full Java compiler AST (faster, lighter) |
| **No Type Resolution** | Type hints are extracted as strings, not resolved to actual types |
| **Single-file** | Each file is analyzed independently |
| **No Runtime Info** | Doesn't execute code, only static analysis |
| **Generics Simplified** | Handles basic generics but not complex nested generics |
| **Inner Classes** | Basic support for inner classes |

---

## 🔮 Future Improvements

- [ ] Full AST support using Java parser library
- [ ] Cross-file type resolution
- [ ] Complete generics support
- [ ] Inheritance graph analysis
- [ ] Method call graph
- [ ] Annotation processor detection
- [ ] Spring-specific analysis (beans, controllers, services)
- [ ] Lombok support (@Data, @Builder, etc.)
- [ ] Micronaut support
- [ ] Quarkus support

---

## 📚 See Also

- [Go Analyzer](../golang/README.md) - Go code analysis
- [Python Analyzer](../python/README.md) - Python code analysis
- [PHP Analyzer](../php/README.md) - PHP code analysis
- [Language Manager](../../language_manager.go) - Language detection and routing
- [CodeChunk Format](../../codetypes/types.go) - Standard code chunk structure
