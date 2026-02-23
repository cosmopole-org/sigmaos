package game_inputs_store

type ZarinpalInput struct {
	GameKey string `json:"gameKey" validate:"required"`
	Product string `json:"product" validate:"required"`
}

func (d ZarinpalInput) GetSpaceId() string {
	return ""
}

func (d ZarinpalInput) GetTopicId() string {
	return ""
}

func (d ZarinpalInput) GetMemberId() string {
	return ""
}

func (d ZarinpalInput) Origin() string {
	return ""
}
