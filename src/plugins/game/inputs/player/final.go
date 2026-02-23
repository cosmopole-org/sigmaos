package game_inputs_player

type ClaimFinalRewardInput struct {
	GameKey string `json:"gameKey" validate:"required"`
	IncKey  string `json:"incKey" validate:"required"`
}

func (d ClaimFinalRewardInput) GetSpaceId() string {
	return ""
}

func (d ClaimFinalRewardInput) GetTopicId() string {
	return ""
}

func (d ClaimFinalRewardInput) GetMemberId() string {
	return ""
}

func (d ClaimFinalRewardInput) Origin() string {
	return ""
}
