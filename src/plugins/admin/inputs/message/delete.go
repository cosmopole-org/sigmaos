package admin_inputs_message

type DeleteMessageInput struct {
	SpaceId   string `json:"spaceId" validate:"required"`
	TopicId   string `json:"topicId" validate:"required"`
	MessageId string `json:"messageId" validate:"required"`
}

func (d DeleteMessageInput) GetSpaceId() string {
	return d.SpaceId
}

func (d DeleteMessageInput) GetTopicId() string {
	return d.TopicId
}

func (d DeleteMessageInput) GetMemberId() string {
	return ""
}

func (d DeleteMessageInput) Origin() string {
	return ""
}
