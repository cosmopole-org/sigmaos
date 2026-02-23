package game_inputs_auth

type StartValidationInput struct {
	Phone string `json:"phone" validate:"required"`
}

func (d StartValidationInput) GetSpaceId() string {
	return ""
}

func (d StartValidationInput) GetTopicId() string {
	return ""
}

func (d StartValidationInput) GetMemberId() string {
	return ""
}

func (d StartValidationInput) Origin() string {
	return ""
}
