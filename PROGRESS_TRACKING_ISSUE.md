# Issue: Status Progress Returns Different Total Values

## Problem
When calling `get_index_status`, the `total` field changes between calls for the same indexing job:
- Call 1: `total: 82, indexed: 1`
- Call 2: `total: 20, indexed: 2`

## Root Cause
In `internal/workspace/manager.go`, the `IndexLanguage()` function determines `total` based on **what files need indexing at that moment**:

```go
// Line 749
m.setProgress(indexKey, language, 0, int64(len(filesToIndex)))
```

This causes:
1. **Initial indexing**: `filesToIndex` = all changed files = 82
2. **Incremental re-index** triggered: `filesToIndex` = newly modified files only = 20

The same `indexKey` can have different totals if:
- Incremental indexing is triggered (`checkAndReindexIfNeeded`)
- Workspace has multiple languages with overlapping file counts
- Progress map is read during the transition between two indexing phases

## Why the Total Changes

1. When you call `get_index_status`, it reads the current progress snapshot
2. If a new indexing job started, it has a different `filesToIndex` list
3. The progress `Total` reflects the **current job's file count**, not a persistent value

## Recommended Fix

Store the initial `Total` persistently in the progress object:

```go
// IndexProgress struct should track initial total
type IndexProgress struct {
	Status    string `json:"status"`
	Language  string `json:"language,omitempty"`
	Indexed   int64  `json:"indexed"`
	Total     int64  `json:"total"`
	Percent   int    `json:"percent"`
	InitialTotal int64 `json:"initial_total,omitempty"` // Add this
	Error     string `json:"error,omitempty"`
}
```

Then update `setProgress()` to NOT change `Total` mid-indexing:

```go
func (m *Manager) setProgress(indexKey, language string, indexed, total int64) {
	pct := 0
	if total > 0 {
		pct = int(indexed * 100 / total)
	}
	m.progressMu.Lock()
	
	// Check if this is an update to existing progress
	if existing, ok := m.progressMap[indexKey]; ok && existing.Total > 0 {
		// Keep the original total
		total = existing.Total
	}
	
	m.progressMap[indexKey] = &IndexProgress{
		Status:   "indexing",
		Language: language,
		Indexed:  indexed,
		Total:    total,  // Now stable across calls
		Percent:  pct,
	}
	m.progressMu.Unlock()
}
```

## Expected Behavior After Fix
- First call: `total: 82, indexed: 1, percent: 1`
- Second call: `total: 82, indexed: 2, percent: 2` (total stays the same!)
- Subsequent calls: Total remains 82 until indexing completes
