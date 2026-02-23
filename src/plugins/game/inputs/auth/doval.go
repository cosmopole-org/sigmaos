package game_inputs_auth

type DoValidationInput struct {
	Code string `json:"code" validate:"required"`
}

func (d DoValidationInput) GetSpaceId() string {
	return ""
}

func (d DoValidationInput) GetTopicId() string {
	return ""
}

func (d DoValidationInput) GetMemberId() string {
	return ""
}

func (d DoValidationInput) Origin() string {
	return ""
}
