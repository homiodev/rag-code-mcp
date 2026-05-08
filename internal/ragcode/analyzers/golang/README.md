# Go Code Analyzer

Analizor de cod Go pentru extragerea simbolurilor, structurii și documentației din fișiere Go. Folosește AST-ul nativ Go pentru parsare precisă. Indexează codul pentru căutare semantică în Qdrant.

## Status: ✅ PRODUCTION READY

---

## 🎯 Ce Face Acest Analizor?

Analizorul Go parsează fișierele `.go` și extrage:
1. **Simboluri** - funcții, metode, tipuri (struct/interface), constante, variabile
2. **Documentație** - comentarii GoDoc pentru toate simbolurile
3. **Metadate** - semnături, parametri, return types, receivers
4. **Exemple** - funcții Example* pentru documentație

---

## 📊 Fluxul de Date

```
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│  Fișiere .go    │────▶│   Go Analyzer    │────▶│   CodeChunks    │
│  (cod sursă)    │     │  (go/ast parser) │     │   (structurat)  │
└─────────────────┘     └──────────────────┘     └────────┬────────┘
                                                          │
                                                          ▼
                                                 ┌─────────────────┐
                                                 │     Qdrant      │
                                                 │  (vector store) │
                                                 └─────────────────┘
```

---

## 🔍 Ce Indexăm

### 1. Funcții (`type: "function"`)

```go
// ProcessData transformă datele de intrare în formatul dorit.
// Returnează eroare dacă datele sunt invalide.
func ProcessData(input []byte, options ...Option) (Result, error) {
    // implementare
}
```

**Informații extrase:**
| Câmp | Valoare | Descriere |
|------|---------|-----------|
| `name` | `"ProcessData"` | Numele funcției |
| `signature` | `"func ProcessData(input []byte, options ...Option) (Result, error)"` | Semnătura completă |
| `parameters` | `[{name: "input", type: "[]byte"}, {name: "options", type: "...Option"}]` | Parametri |
| `returns` | `[{type: "Result"}, {type: "error"}]` | Tipuri returnate |
| `is_exported` | `true` | Dacă e exportată (începe cu majusculă) |
| `docstring` | `"ProcessData transformă..."` | Comentariul GoDoc |

### 2. Metode (`type: "method"`)

```go
// Save persistă utilizatorul în baza de date.
func (u *User) Save(ctx context.Context) error {
    return u.db.Save(ctx, u)
}
```

**Informații extrase:**
| Câmp | Valoare | Descriere |
|------|---------|-----------|
| `name` | `"Save"` | Numele metodei |
| `receiver` | `"*User"` | Receiver-ul metodei |
| `is_method` | `true` | Este metodă, nu funcție |
| `parameters` | `[{name: "ctx", type: "context.Context"}]` | Parametri |
| `returns` | `[{type: "error"}]` | Tipuri returnate |

### 3. Structuri (`type: "struct"`)

```go
// User reprezintă un utilizator în sistem.
type User struct {
    ID        int64     `json:"id" db:"id"`
    Name      string    `json:"name" db:"name"`
    Email     string    `json:"email" db:"email"`
    CreatedAt time.Time `json:"created_at" db:"created_at"`
}
```

**Informații extrase:**
| Câmp | Valoare | Descriere |
|------|---------|-----------|
| `name` | `"User"` | Numele tipului |
| `kind` | `"struct"` | Tipul declarației |
| `fields` | `[{name: "ID", type: "int64", tag: "json:\"id\"..."}, ...]` | Câmpurile structurii |
| `methods` | `[{name: "Save", ...}, ...]` | Metodele asociate |
| `is_exported` | `true` | Dacă e exportat |
| `docstring` | `"User reprezintă..."` | Comentariul GoDoc |

### 4. Interfețe (`type: "interface"`)

```go
// Repository definește operațiile de persistență.
type Repository interface {
    // Find caută o entitate după ID.
    Find(ctx context.Context, id int64) (*Entity, error)
    // Save persistă o entitate.
    Save(ctx context.Context, entity *Entity) error
    // Delete șterge o entitate.
    Delete(ctx context.Context, id int64) error
}
```

**Informații extrase:**
| Câmp | Valoare | Descriere |
|------|---------|-----------|
| `name` | `"Repository"` | Numele interfeței |
| `kind` | `"interface"` | Tipul declarației |
| `methods` | `[{name: "Find", ...}, {name: "Save", ...}, ...]` | Metodele interfeței |

### 5. Constante (`type: "const"`)

```go
// StatusActive reprezintă un utilizator activ.
const StatusActive = "active"

const (
    // MaxRetries este numărul maxim de reîncercări.
    MaxRetries = 3
    // DefaultTimeout este timeout-ul implicit.
    DefaultTimeout = 30 * time.Second
)
```

**Informații extrase:**
| Câmp | Valoare | Descriere |
|------|---------|-----------|
| `name` | `"StatusActive"` | Numele constantei |
| `type` | `"string"` | Tipul (dacă e specificat) |
| `value` | `"active"` | Valoarea |
| `is_exported` | `true` | Dacă e exportată |

### 6. Variabile (`type: "var"`)

```go
// DefaultConfig conține configurația implicită.
var DefaultConfig = Config{
    Timeout: 30 * time.Second,
    Retries: 3,
}
```

---

## 🏗️ Structura Fișierelor

```
golang/
├── types.go           # Tipuri: PackageInfo, FunctionInfo, TypeInfo, etc.
├── analyzer.go        # PathAnalyzer implementation (800+ linii)
├── api_analyzer.go    # APIAnalyzer pentru documentație API
├── analyzer_test.go   # Teste CodeAnalyzer
├── api_analyzer_test.go # Teste APIAnalyzer
└── README.md          # Această documentație
```

---

## 💻 Utilizare

### Analiză Package

```go
import "github.com/homiodev/rag-code-mcp/internal/ragcode/analyzers/golang"

// Creare analizor
analyzer := golang.NewCodeAnalyzer()

// Analiză un package
pkgInfo, err := analyzer.AnalyzePackage("./internal/mypackage")
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Package: %s\n", pkgInfo.Name)
fmt.Printf("Functions: %d\n", len(pkgInfo.Functions))
fmt.Printf("Types: %d\n", len(pkgInfo.Types))
```

### Analiză Multiple Paths (PathAnalyzer interface)

```go
// Analiză directoare/fișiere
chunks, err := analyzer.AnalyzePaths([]string{"./internal/..."})

for _, chunk := range chunks {
    fmt.Printf("[%s] %s.%s\n", chunk.Type, chunk.Package, chunk.Name)
}
```

---

## 🔌 Integrare

### Language Manager

Analizorul Go este selectat automat pentru:
- `go` - proiecte Go
- `golang` - alternativă

### Detectare Workspace

| Fișier | Descriere |
|--------|-----------|
| `go.mod` | Go modules (Go 1.11+) |
| `go.sum` | Checksums dependențe |
| `*.go` | Fișiere sursă Go |

---

## 📋 Tipuri de CodeChunk

| Type | Descriere | Exemplu |
|------|-----------|---------|
| `function` | Funcție package-level | `func Process()` |
| `method` | Metodă pe tip | `func (u *User) Save()` |
| `struct` | Tip struct | `type User struct{}` |
| `interface` | Tip interface | `type Reader interface{}` |
| `const` | Constantă | `const MaxSize = 100` |
| `var` | Variabilă package-level | `var DefaultConfig = ...` |

---

## 🏷️ Metadate Complete

### Function/Method Metadata
```json
{
  "is_exported": true,
  "is_method": true,
  "receiver": "*User",
  "parameters": [
    {"name": "ctx", "type": "context.Context"},
    {"name": "id", "type": "int64"}
  ],
  "returns": [
    {"type": "*Entity"},
    {"type": "error"}
  ]
}
```

### Type Metadata
```json
{
  "kind": "struct",
  "is_exported": true,
  "fields": [
    {"name": "ID", "type": "int64", "tag": "json:\"id\""},
    {"name": "Name", "type": "string", "tag": "json:\"name\""}
  ],
  "methods": [
    {"name": "Save", "signature": "func (u *User) Save() error"}
  ]
}
```

---

## 🧪 Testare

```bash
# Rulează toate testele Go analyzer
go test ./internal/ragcode/analyzers/golang/...

# Cu output verbose
go test -v ./internal/ragcode/analyzers/golang/...

# Cu coverage
go test -cover ./internal/ragcode/analyzers/golang/...
```

---

## 📦 Dependențe

Folosește doar biblioteca standard Go:
- `go/ast` - Abstract Syntax Tree
- `go/parser` - Parser Go
- `go/doc` - Extragere documentație
- `go/token` - Tokenizare
- `go/types` - Informații despre tipuri

---

## 🚫 Căi Excluse

Analizorul sare automat:
- `*_test.go` - fișiere de test
- `vendor/` - dependențe vendored
- `testdata/` - date de test
- `.git/` - Git

---

## ⚠️ Limitări

| Limitare | Descriere |
|----------|-----------|
| **Package-level** | Analizează la nivel de package, nu fișier individual |
| **No Cross-package** | Nu rezolvă tipuri din alte package-uri |
| **No Generics** | Suport limitat pentru generics (Go 1.18+) |

---

## 🔮 Îmbunătățiri Viitoare

- [ ] Suport complet generics (Go 1.18+)
- [ ] Cross-package type resolution
- [ ] Dependency graph între packages
- [ ] Test coverage analysis
- [ ] Benchmark detection
