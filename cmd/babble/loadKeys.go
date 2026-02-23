package keyloader

import (
	"fmt"
	models "kasper/src/shell/api/model"
	"kasper/src/shell/layer1/adapters"
	"kasper/src/shell/layer1/tools/signaler"
	"log"
	"strconv"
)

const memberTemplate = "member::%s::%s::%s"
const cityTemplate = "city::%s"

func convertRowIdToCode(rowId uint) string {
	idStr := fmt.Sprintf("%d", rowId)
	for len(idStr) < 6 {
		idStr = "0" + idStr
	}
	var c = ""
	for i := 0; i < len(idStr); i++ {
		if i < 3 {
			digit, err := strconv.ParseInt(idStr[i:i+1], 10, 32)
			if err != nil {
				fmt.Println(err)
				return ""
			}
			c += string(rune('A' + digit))
		} else {
			c += idStr[i : i+1]
		}
	}
	return c
}

func LoadCacheKeys(s adapters.IStorage, cache adapters.ICache, signaler *signaler.Signaler) {	
	var members []models.Member
	s.Db().Model(&models.Member{}).Find(&members)
	log.Println("members count:", len(members))
	counter := 0
	for _, member := range members {
		cache.Put(fmt.Sprintf(memberTemplate, member.SpaceId, member.UserId, member.Id), member.TopicId)
		signaler.JoinGroup(member.SpaceId, member.UserId)
		counter++
		if counter%100 == 0 {
			log.Println("loaded members of amount:", counter)
		}
	}

	sessions := []models.Session{}
	s.Db().Model(&models.Session{}).Find(&sessions)
	log.Println("sessions count:", len(sessions))
	counter = 0
	for _, session := range sessions {
		cache.Put("auth::"+session.Token, "human/"+session.UserId)
		counter++
		if counter%100 == 0 {
			log.Println("amount of loaded sessions:", counter)
		}
	}

	users := []models.User{}
	s.Db().Model(&models.User{}).Find(&users)
	log.Println("interaction codes count:", len(users))
	counter = 0
	for _, user := range users {
		cache.Put("code::"+convertRowIdToCode(uint(user.Number)), user.Id)
		counter++
		if counter%100 == 0 {
			log.Println("amount of loaded interaction codes:", counter)
		}
	}

	topics := []models.Topic{}
	s.Db().Model(&models.Topic{}).Find(&topics)
	log.Println("topics count:", len(topics))
	counter = 0
	for _, topic := range topics {
		cache.Put(fmt.Sprintf(cityTemplate, topic.Id), topic.SpaceId)
		counter++
		if counter%100 == 0 {
			log.Println("amount of loaded sessions:", counter)
		}
	}
}
