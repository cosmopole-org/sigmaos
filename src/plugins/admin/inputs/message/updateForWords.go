package admin_inputs_message

type UpdateForWordsInput struct {
	Words map[string]bool `json:"words" validate:"required"`
}

func (d UpdateForWordsInput) GetSpaceId() string {
	return ""
}

func (d UpdateForWordsInput) GetTopicId() string {
	return ""
}

func (d UpdateForWordsInput) GetMemberId() string {
	return ""
}

func (d UpdateForWordsInput) Origin() string {
	return ""
}
