package admin_inputs_player

type BanPhoneInput struct {
	BannedPhones map[string]bool `json:"bannedPhones" validate:"required"`
}

func (d BanPhoneInput) GetSpaceId() string {
	return ""
}

func (d BanPhoneInput) GetTopicId() string {
	return ""
}

func (d BanPhoneInput) GetMemberId() string {
	return ""
}

func (d BanPhoneInput) Origin() string {
	return ""
}
