package cli

import (
	"encoding/json"

	"github.com/archguard/project/client/review"
	c "github.com/archguard/project/shared/constants"
)

func countFiles(r review.ToolRunner) int {
	res, _ := r.ExecuteTool(c.ToolGetProjectStructure, nil, c.FileCountBudget)
	var p struct{ Total int }
	json.Unmarshal([]byte(res), &p)
	return p.Total
}
