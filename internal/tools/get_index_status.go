package tools

import (
	"context"
	"encoding/json"

	"github.com/homiodev/rag-code-mcp/internal/workspace"
)

// GetIndexStatusTool reports live embedding progress for all active indexing jobs.
type GetIndexStatusTool struct {
	workspaceManager *workspace.Manager
}

// NewGetIndexStatusTool creates a new GetIndexStatusTool.
func NewGetIndexStatusTool(wm *workspace.Manager) *GetIndexStatusTool {
	return &GetIndexStatusTool{workspaceManager: wm}
}

// Name returns the tool name.
func (t *GetIndexStatusTool) Name() string {
	return "get_index_status"
}

// Description returns the tool description.
func (t *GetIndexStatusTool) Description() string {
	return "Returns live indexing progress for all active workspace/language pairs. " +
		"Each entry has: status (idle|indexing|done|error), language, indexed, total, percent."
}

// Execute returns JSON array of IndexProgress entries, or [{"status":"idle"}] when empty.
func (t *GetIndexStatusTool) Execute(_ context.Context, _ map[string]interface{}) (string, error) {
	if t.workspaceManager == nil {
		b, _ := json.Marshal([]map[string]string{{"status": "idle"}})
		return string(b), nil
	}
	progress := t.workspaceManager.GetAllProgress()
	if len(progress) == 0 {
		b, _ := json.Marshal([]map[string]string{{"status": "idle"}})
		return string(b), nil
	}
	b, err := json.Marshal(progress)
	if err != nil {
		return `[{"status":"idle","error":"marshal failed"}]`, nil
	}
	return string(b), nil
}
