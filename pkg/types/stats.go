package types

type UserOrderStats struct {
	InProgressCount      int     `json:"in_progress_count"`
	CompletedCount       int     `json:"completed_count"`
	ClosedCount          int     `json:"closed_count"`
	OverdueCount         int     `json:"overdue_count"`
	TotalCount           int     `json:"total_count"`
	AvgResolutionSeconds float64 `json:"avg_resolution_seconds"`
}
