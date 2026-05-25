package bootstrap

import (
	"encoding/json"
	"fmt"

	"github.com/archguard/project/application/review"
	c "github.com/archguard/project/shared/constants"
)

func countFiles(r review.ToolRunner) (int, error) {
	res, err := r.ExecuteTool(c.ToolGetProjectStructure, nil, c.FileCountBudget)
	if err != nil {
		return 0, fmt.Errorf("get project structure: %w", err)
	}
	var p struct{ Total int }
	if err := json.Unmarshal([]byte(res), &p); err != nil {
		return 0, fmt.Errorf("parse project structure: %w", err)
	}
	return p.Total, nil
}
