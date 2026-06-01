# Improved Solution: Multi-Phase Progress Tracking (No Backward Compatibility)

## The Better Approach

Instead of ONE progress bar that changes, track **multiple phases** separately and aggregate them!

## Current Problem
```
Call 1: total=82, indexed=1   (code phase)
Call 2: total=20, indexed=2   (different phase, total changed!) ❌
```

## Better Solution
```
Phase 1: Code Files
├─ total: 82
├─ indexed: 1
└─ status: indexing

Phase 2: Docs
├─ total: 5 (markdown files)
├─ indexed: 0
└─ status: pending

Aggregated View:
├─ total: 87 (82 + 5)
├─ indexed: 1
└─ percent: 1%
```

## Implementation

### 1. Replace IndexProgress Structure
```go
type IndexingPhase struct {
	Name     string `json:"name"`       // "code", "docs", etc.
	Status   string `json:"status"`     // idle|indexing|done|error
	Language string `json:"language,omitempty"`
	Indexed  int64  `json:"indexed"`
	Total    int64  `json:"total"`
	Percent  int    `json:"percent"`
	Error    string `json:"error,omitempty"`
}

// REPLACE OLD IndexProgress with this
type IndexProgress struct {
	IndexKey  string           `json:"index_key"`
	Status    string           `json:"status"` // overall status
	Language  string           `json:"language,omitempty"`
	Phases    []IndexingPhase  `json:"phases"` // individual phases
	// Aggregated across all phases:
	Indexed   int64            `json:"indexed_total"`
	Total     int64            `json:"total_files"`
	Percent   int              `json:"percent"`
	Error     string           `json:"error,omitempty"`
}
```

### 2. Update Manager Struct
```go
type Manager struct {
	detector *Detector
	cache    *Cache
	qdrant   *storage.QdrantClient
	llm      llm.Provider
	config   *config.Config

	// Indexing state
	indexingMu sync.RWMutex
	indexing   map[string]bool // workspace ID -> is indexing

	// REPLACE: progressMap with phased version
	progressMu  sync.RWMutex
	progressMap map[string]*IndexProgress // indexKey → phases + aggregates
	// (remove old progressMap structure)

	// ... rest of fields
}
```

### 3. Tracking Multiple Phases
```go
// In IndexLanguage() function around line 742-777

// Calculate total files upfront
totalCodeFiles := int64(len(filesToIndex))
totalDocFiles := int64(len(docsToIndex))

// Initialize phases at start of indexing
m.initializePhases(indexKey, language, map[string]int64{
	"code": totalCodeFiles,
	"docs": totalDocFiles,
})

// Process indexing (Code)
if len(filesToIndex) > 0 {
	log.Printf("📝 Indexing %d code files...", len(filesToIndex))
	m.setPhaseProgress(indexKey, "code", 0, totalCodeFiles, "indexing")
	
	for i, filePath := range filesToIndex {
		n, ferr := indexer.IndexPaths(ctx, []string{filePath}, collectionName)
		totalChunks += n
		m.setPhaseProgress(indexKey, "code", int64(i+1), totalCodeFiles, "indexing")
		if ferr != nil {
			log.Printf("[WARN] indexing skipped file %s: %v", filePath, ferr)
		}
	}
	m.setPhaseProgress(indexKey, "code", totalCodeFiles, totalCodeFiles, "done")
}

// Process indexing (Docs)
if len(docsToIndex) > 0 {
	log.Printf("📚 Indexing %d doc files...", len(docsToIndex))
	m.setPhaseProgress(indexKey, "docs", 0, totalDocFiles, "indexing")
	
	numDocs := m.indexMarkdownFiles(ctx, docsToIndex, collectionName, ltm)
	
	m.setPhaseProgress(indexKey, "docs", totalDocFiles, totalDocFiles, "done")
}
```

### 4. Replace Old Methods
```go
// REMOVE: setProgress()
// REPLACE with: initializePhases()
func (m *Manager) initializePhases(indexKey, language string, phases map[string]int64) {
	m.progressMu.Lock()
	defer m.progressMu.Unlock()
	
	phasesList := make([]IndexingPhase, 0, len(phases))
	totalFiles := int64(0)
	
	for phaseName, count := range phases {
		phasesList = append(phasesList, IndexingPhase{
			Name:     phaseName,
			Status:   "idle",
			Language: language,
			Total:    count,
		})
		totalFiles += count
	}
	
	m.progressMap[indexKey] = &IndexProgress{
		IndexKey: indexKey,
		Status:   "indexing",
		Language: language,
		Phases:   phasesList,
		Total:    totalFiles,
	}
}

// NEW: setPhaseProgress() - replaces setProgress()
func (m *Manager) setPhaseProgress(indexKey, phaseName string, indexed, total int64, status string) {
	m.progressMu.Lock()
	defer m.progressMu.Unlock()
	
	progress, ok := m.progressMap[indexKey]
	if !ok {
		return
	}
	
	// Update the phase
	for i, phase := range progress.Phases {
		if phase.Name == phaseName {
			pct := 0
			if total > 0 {
				pct = int(indexed * 100 / total)
			}
			progress.Phases[i] = IndexingPhase{
				Name:     phaseName,
				Status:   status,
				Language: phase.Language,
				Indexed:  indexed,
				Total:    total,
				Percent:  pct,
			}
			break
		}
	}
	
	// Recalculate aggregates
	totalIndexed := int64(0)
	allDone := true
	for _, phase := range progress.Phases {
		totalIndexed += phase.Indexed
		if phase.Status != "done" && phase.Status != "idle" {
			allDone = false
		}
	}
	
	overallPct := 0
	if progress.Total > 0 {
		overallPct = int(totalIndexed * 100 / progress.Total)
	}
	
	progress.Indexed = totalIndexed
	progress.Percent = overallPct
	
	if allDone {
		progress.Status = "done"
	}
}

// KEEP GetAllProgress() - same signature, new format
func (m *Manager) GetAllProgress() []IndexProgress {
	m.progressMu.RLock()
	defer m.progressMu.RUnlock()
	out := make([]IndexProgress, 0, len(m.progressMap))
	for _, p := range m.progressMap {
		out = append(out, *p)
	}
	return out
}
```

### 5. Update GetIndexStatusTool (no changes needed!)
```go
// internal/tools/get_index_status.go - no changes!
// It already calls GetAllProgress() which now returns multi-phase data
```

## API Response Example

**Call 1:**
```json
[
  {
    "index_key": "workspace1-java",
    "status": "indexing",
    "language": "java",
    "indexed_total": 1,
    "total_files": 87,
    "percent": 1,
    "phases": [
      {
        "name": "code",
        "status": "indexing",
        "indexed": 1,
        "total": 82,
        "percent": 1
      },
      {
        "name": "docs",
        "status": "idle",
        "indexed": 0,
        "total": 5,
        "percent": 0
      }
    ]
  }
]
```

**Call 2 (10 seconds later):**
```json
[
  {
    "index_key": "workspace1-java",
    "status": "indexing",
    "language": "java",
    "indexed_total": 35,
    "total_files": 87,
    "percent": 40,
    "phases": [
      {
        "name": "code",
        "status": "done",
        "indexed": 82,
        "total": 82,
        "percent": 100
      },
      {
        "name": "docs",
        "status": "indexing",
        "indexed": 1,
        "total": 5,
        "percent": 20
      }
    ]
  }
]
```

## Benefits

✅ **No changing totals** - Each phase has its own stable total  
✅ **Transparency** - Users see exactly which phases are happening  
✅ **Cleaner code** - Single responsibility per method  
✅ **Better UX** - Progressive bars show "Step 1 of 2", per-phase progress  
✅ **Accurate aggregates** - Overall progress is sum of all phases  
✅ **Per-phase details** - Know if stuck on "docs indexing" vs "code indexing"  
✅ **No duplication** - One data structure, one truth source  

## Migration Checklist

- [ ] Update `IndexProgress` struct in `internal/workspace/manager.go` (line 57)
- [ ] Update `Manager` struct progressMap type (line 38)
- [ ] Remove `setProgress()` method (line 276)
- [ ] Add `initializePhases()` method
- [ ] Add `setPhaseProgress()` method
- [ ] Update `IndexLanguage()` to use new methods (lines 742-777)
- [ ] Keep `GetAllProgress()` - no signature change
- [ ] No changes needed to `GetIndexStatusTool`
- [ ] Test with multiple workspaces

## Files to Modify

1. `internal/workspace/manager.go` - Main changes
   - Struct definitions (lines 24-65)
   - Remove setProgress() (line 276)
   - Add initializePhases() and setPhaseProgress()
   - Update IndexLanguage() calls (lines 742-777)

That's it! Clean, simple, no backward compat baggage.
