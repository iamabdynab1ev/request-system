package dto

type ADUsernameBindingFailedDTO struct {
	UserID   uint64      `json:"user_id"`
	FIO      string      `json:"fio"`
	Email    string      `json:"email"`
	Username string      `json:"username,omitempty"`
	Reason   string      `json:"reason"`
	Details  interface{} `json:"details,omitempty"`
}

type ADUsernameBindingManualReviewDTO struct {
	UserID          uint64      `json:"user_id"`
	FIO             string      `json:"fio"`
	Email           string      `json:"email"`
	LocalPart       string      `json:"local_part,omitempty"`
	CurrentUsername *string     `json:"current_username,omitempty"`
	ReasonCode      string      `json:"reason_code"`
	Reason          string      `json:"reason"`
	SuggestedSearch string      `json:"suggested_search"`
	Details         interface{} `json:"details,omitempty"`
}

type ADUsernameBindingResultDTO struct {
	TotalUsers         int                                `json:"total_users"`
	Updated            int                                `json:"updated"`
	AlreadyMatched     int                                `json:"already_matched"`
	NotFoundExactMatch []string                           `json:"not_found_exact_match_emails"`
	InvalidEmails      []string                           `json:"invalid_emails"`
	Failed             []ADUsernameBindingFailedDTO       `json:"failed"`
	ManualReview       []ADUsernameBindingManualReviewDTO `json:"manual_review"`
	ManualReviewCount  int                                `json:"manual_review_count"`
}
