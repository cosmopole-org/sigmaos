package game_model

type Otp struct {
	UserId string `json:"userId" gorm:"primaryKey;column:user_id"`
	Code   string `json:"code" gorm:"column:code"`
	Count  int32  `json:"count" gorm:"column:count"`
	Time   int64  `json:"time" gorm:"column:time"`
}

func (m Otp) Type() string {
	return "Otp"
}
