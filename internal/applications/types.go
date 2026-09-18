package applications

type CreateApplicationRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
}

type Application struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	SseEnable   bool   `json:"sseEnabled,omitempty"`
	CreatedAt   string `json:"createdAt,omitempty"`
	UpdatedAt   string `json:"updatedAt,omitempty"`
}
