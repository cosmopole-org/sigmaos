package actions_store

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"kasper/src/abstract"
	game_inputs_store "kasper/src/plugins/game/inputs/store"
	game_model "kasper/src/plugins/game/model"
	model "kasper/src/shell/api/model"
	"kasper/src/shell/layer1/adapters"
	states "kasper/src/shell/layer1/module/state"
	"kasper/src/shell/utils/crypto"
	"kasper/src/shell/utils/future"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"kasper/src/shell/layer1/module/toolbox"

	"gorm.io/gorm"
)

type Actions struct {
	Layer abstract.ILayer
}

func Install(s adapters.IStorage, a *Actions) error {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		payment := r.URL.Query().Get("Authority")
		tb := abstract.UseToolbox[*toolbox.ToolboxL1](a.Layer.Core().Get(1).Tools())
		userId := tb.Cache().Get("payment::" + payment)
		if userId != "" {
			log.Println(userId)
			client := &http.Client{}
			url := "https://payment.zarinpal.com/pg/v4/payment/verify.json"
			method := "POST"
			price := tb.Cache().Get("price::" + payment)
			req, err := http.NewRequest(method, url, bytes.NewBuffer([]byte(
				`{
			  		"merchant_id": "d10b3812-b3da-4b6e-8b36-6165ef4cfb9e",
  					"amount": `+price+`0,
					"authority": "` + payment + `"
				 }`,
			)))
			if err != nil {
				fmt.Println(err)
				w.Write([]byte(err.Error()))
				return
			}
			req.Header.Add("Content-Type", "application/json")
			res, err := client.Do(req)
			if err != nil {
				fmt.Println(err)
				w.Write([]byte(err.Error()))
				return
			}
			defer res.Body.Close()
			body, err := ioutil.ReadAll(res.Body)
			if err != nil {
				fmt.Println(err)
				w.Write([]byte(err.Error()))
				return
			}
			log.Println(string(body))
			output := map[string]any{}
			err3 := json.Unmarshal(body, &output)
			if err3 != nil {
				fmt.Println(err3)
				w.Write([]byte(err3.Error()))
				return
			}

			code := output["data"].(map[string]any)["code"].(float64)
			if code != 100 {
				err := errors.New("payment not verified")
				log.Println(err)
				w.Write([]byte(err3.Error()))
				return
			}

			err = tb.Storage().DoTrx(func(trx adapters.ITrx) error {

				product := ""
				trx.Db().Model(&model.User{}).Select(adapters.BuildJsonFetcher("metadata", "game.payment.product")).Where("id = ?", userId).First(&product)
				log.Println(product)
				trx.ClearError()

				input := game_inputs_store.ZarinpalInput{GameKey: "game", Product: product}

				effects := map[string]float64{}
				userData := map[string]interface{}{}

				meta := game_model.Meta{Id: input.GameKey + "::buy"}
				err4 := trx.Db().First(&meta).Error
				if err4 != nil {
					return err4
				}
				val := meta.Data[input.Product].(string)
				data := strings.Split(val, ".")
				for i := range data {
					if i%2 == 0 {
						number, err5 := strconv.ParseFloat(data[i+1], 64)
						if err5 != nil {
							fmt.Println(err5)
							continue
						}
						effects[data[i]] = number
					}
				}
				trx.ClearError()
				gameDataStr := ""
				trx.Db().Model(&model.User{}).Select(adapters.BuildJsonFetcher("metadata", input.GameKey)).Where("id = ?", userId).First(&gameDataStr)
				trx.ClearError()
				err6 := json.Unmarshal([]byte(gameDataStr), &userData)
				if err6 != nil {
					log.Println(err6)
					return err6
				}

				registerTime := userData["registerDate"].(float64)
				_, ok := userData["firstBuy"]
				if !ok {
					err := adapters.UpdateJson(func() *gorm.DB { return trx.Db().Model(&model.User{}).Where("id = ?", userId) }, &model.User{Id: userId}, "metadata", input.GameKey+".firstBuy", math.Ceil((float64(time.Now().UnixMilli())-registerTime)/(24*60*60*1000)))
					if err != nil {
						log.Println(err)
						return err
					}
				}

				for k, v := range effects {
					if v == 0 {
						continue
					}
					timeKey := "last" + (strings.ToUpper(string(k[0])) + k[1:]) + "Buy"
					now := int64(time.Now().UnixMilli())
					oldValRaw, ok := userData[k]
					if !ok {
						continue
					}
					oldVal := oldValRaw.(float64)
					newVal := v + oldVal
					lastBuyTimeRaw, ok2 := userData[timeKey]
					if k == "chat" && ok2 {
						lastBuyTime := lastBuyTimeRaw.(float64)
						if float64(now) < (lastBuyTime + oldVal) {
							newVal = math.Ceil((v * 24 * 60 * 60 * 1000) + oldVal - (float64(now) - lastBuyTime))
						} else {
							newVal = v * 24 * 60 * 60 * 1000
						}
					}
					err := adapters.UpdateJson(func() *gorm.DB { return trx.Db().Model(&model.User{}).Where("id = ?", userId) }, &model.User{Id: userId}, "metadata", input.GameKey+"."+k, newVal)
					if err != nil {
						log.Println(err)
						return err
					}
					trx.ClearError()
					err2 := adapters.UpdateJson(func() *gorm.DB { return trx.Db().Model(&model.User{}).Where("id = ?", userId) }, &model.User{Id: userId}, "metadata", input.GameKey+"."+timeKey, now)
					if err2 != nil {
						log.Println(err2)
						return err2
					}
					trx.ClearError()
				}
				trx.Mem().Del("payment::" + payment)
				adapters.UpdateJson(func() *gorm.DB { return trx.Db().Model(&model.User{}).Where("id = ?", userId) }, &model.User{Id: userId}, "metadata", "game.payment.product", "")
				adapters.UpdateJson(func() *gorm.DB { return trx.Db().Model(&model.User{}).Where("id = ?", userId) }, &model.User{Id: userId}, "metadata", "game.payment.success", true)

				p := game_model.Payment{Token: "-", Market: "google", Id: crypto.SecureUniqueId(a.Layer.Core().Id()), UserId: userId, Product: input.Product, GameKey: input.GameKey, Time: time.Now().UnixMilli()}
				trx.Db().Create(&p)

				return nil
			})
			log.Println(err)
			if err != nil {
				w.Write([]byte(err.Error()))
			} else {
				content, err := os.ReadFile("/app/payment/index.html")
				if err != nil {
					log.Println(err)
				}
				w.Write(content)
			}
		} else {
			w.Write([]byte("payment not found"))
		}
	})
	future.Async(func() {
		http.ListenAndServe(":8078", nil)
	}, false)
	return s.AutoMigrate(&game_model.Payment{})
}

// ZarinpalState /store/zarinpalState check [ true false false ] access [ true false false false POST ]
func (a *Actions) ZarinpalState(s abstract.IState, input game_inputs_store.ZarinpalStateInput) (any, error) {
	var state = abstract.UseState[states.IStateL1](s)
	trx := state.Trx()
	var success bool = false
	trx.Db().Model(&model.User{}).Select(adapters.BuildJsonFetcher("metadata", "game.payment.success")).Where("id = ?", state.Info().UserId()).First(&success)
	adapters.UpdateJson(func() *gorm.DB { return trx.Db().Model(&model.User{}).Where("id = ?", state.Info().UserId()) }, &model.User{Id: state.Info().UserId()}, "metadata", "game.payment.success", false)
	return map[string]any{"paymentSuccessful": success}, nil
}

// Zarinpal /store/zarinpal check [ true false false ] access [ true false false false POST ]
func (a *Actions) Zarinpal(s abstract.IState, input game_inputs_store.ZarinpalInput) (any, error) {
	var state = abstract.UseState[states.IStateL1](s)
	trx := state.Trx()
	meta := game_model.Meta{Id: input.GameKey + "::buy"}
	err4 := trx.Db().First(&meta).Error
	if err4 != nil {
		return nil, err4
	}
	priceData := strings.Split(meta.Data[input.Product].(string), ".")
	price := priceData[len(priceData)-1]
	client := &http.Client{}
	url := "https://payment.zarinpal.com/pg/v4/payment/request.json"
	method := "POST"
	req, err := http.NewRequest(method, url, bytes.NewBuffer([]byte(
		`{
  		"merchant_id": "",
  		"amount": `+price+`0,
  		"callback_url": "",
  		"description": "",
		"order_id": "`+state.Info().UserId()+`"
	}`,
	)))
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	req.Header.Add("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	log.Println(string(body))
	output := map[string]any{}
	err3 := json.Unmarshal(body, &output)
	if err3 != nil {
		fmt.Println(err3)
		return nil, err3
	}
	payment := output["data"].(map[string]any)["authority"].(string)
	adapters.UpdateJson(func() *gorm.DB { return trx.Db().Model(&model.User{}).Where("id = ?", state.Info().UserId()) }, &model.User{Id: state.Info().UserId()}, "metadata", "game.payment", input)
	trx.Mem().Put("payment::"+payment, state.Info().UserId())
	trx.Mem().Put("price::"+payment, price)
	return map[string]any{"url": "https://payment.zarinpal.com/pg/StartPay/" + payment}, nil
}

// Buy /store/buy check [ true false false ] access [ true false false false POST ]
func (a *Actions) Buy(s abstract.IState, input game_inputs_store.BuyInput) (any, error) {
	var state = abstract.UseState[states.IStateL1](s)
	trx := state.Trx()

	user := model.User{Id: state.Info().UserId()}
	trx.Db().First(&user)
	trx.ClearError()

	market := input.Market
	if market == "" {
		market = "bazar"
	}

	if market == "bazar" {
		url := "https://pardakht.cafebazaar.ir/devapi/v2/auth/token/"
		method := "POST"
		payload := strings.NewReader(`{
    	"grant_type": "refresh_token",
    	"client_id": "",
    	"client_secret": "",
    	"refresh_token": ""
	}`)
		client := &http.Client{}
		req, err := http.NewRequest(method, url, payload)
		if err != nil {
			fmt.Println(err)
			return nil, err
		}
		req.Header.Add("Content-Type", "application/json")
		res, err := client.Do(req)
		if err != nil {
			fmt.Println(err)
			return nil, err
		}
		defer res.Body.Close()
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			fmt.Println(err)
			return nil, err
		}
		output := map[string]any{}
		err2 := json.Unmarshal(body, &output)
		if err2 != nil {
			fmt.Println(err)
			return nil, err
		}
		atRaw, ok := output["access_token"]
		if !ok {
			err := errors.New("access token not returned")
			fmt.Println(err)
			return nil, err
		}
		accessToken := atRaw.(string)

		url2 := "https://pardakht.cafebazaar.ir/devapi/v2/api/validate/{packageName}/inapp/" + input.Product + "/purchases/" + input.PurchaseToken + "/?access_token=" + accessToken
		method2 := "GET"
		req2, err := http.NewRequest(method2, url2, nil)
		if err != nil {
			fmt.Println(err)
			return nil, err
		}
		res2, err := client.Do(req2)
		if err != nil {
			fmt.Println(err)
			return nil, err
		}
		defer res2.Body.Close()
		body2, err := ioutil.ReadAll(res2.Body)
		if err != nil {
			fmt.Println(err)
			return nil, err
		}
		output2 := map[string]any{}
		err3 := json.Unmarshal(body2, &output2)
		if err3 != nil {
			fmt.Println(err3)
			return nil, err3
		}
		_, ok2 := output2["purchaseTime"]
		if !ok2 {
			err := errors.New("purchase not found")
			fmt.Println(err)
			return nil, err
		}

		url3 := "https://pardakht.cafebazaar.ir/devapi/v2/api/consume/{packageName}/purchases/?access_token=" + accessToken
		method3 := "POST"
		payload3 := strings.NewReader(`{
    		"token": "` + input.PurchaseToken + `"
		}`)
		req3, err := http.NewRequest(method3, url3, payload3)
		if err != nil {
			fmt.Println(err)
			return nil, err
		}
		req3.Header.Add("Content-Type", "application/json")
		res3, err := client.Do(req3)
		if err != nil {
			fmt.Println(err)
			return nil, err
		}
		defer res3.Body.Close()
		if res3.StatusCode != http.StatusOK {
			errC := errors.New("consuming bazar error")
			fmt.Println(errC)
			return nil, errC
		}

		body3, err := ioutil.ReadAll(res3.Body)
		if err != nil {
			fmt.Println(err)
			return nil, err
		}
		log.Println(string(body3))
	} else if market == "myket" {
		url := "https://developer.myket.ir/api/partners/applications/{packageName}/purchases/products/" + input.Product + "/verify"
		method := "POST"
		payload := strings.NewReader(`{
    		"tokenId": "` + input.PurchaseToken + `"
		}`)
		client := &http.Client{}
		req, err := http.NewRequest(method, url, payload)
		if err != nil {
			fmt.Println(err)
			return nil, err
		}
		req.Header.Add("Content-Type", "application/json")
		req.Header.Add("X-Access-Token", "")
		res, err := client.Do(req)
		if err != nil {
			fmt.Println(err)
			return nil, err
		}
		defer res.Body.Close()
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			fmt.Println(err)
			return nil, err
		}
		output := map[string]any{}
		err2 := json.Unmarshal(body, &output)
		if err2 != nil {
			fmt.Println(err)
			return nil, err
		}
		o, oko := output["purchaseState"]
		if !oko || (o.(float64) != 0) {
			err := errors.New("unsucessful myket purchase")
			fmt.Println(err)
			return nil, err
		}

		url2 := "https://developer.myket.ir/api/partners/applications/{packageName}/purchases/products/" + input.Product + "/tokens/" + input.PurchaseToken + "/consume"
		method2 := "PUT"
		req2, err := http.NewRequest(method2, url2, nil)
		if err != nil {
			fmt.Println(err)
			return nil, err
		}
		req2.Header.Add("Content-Type", "application/json")
		req2.Header.Add("X-Access-Token", "")
		res2, err := client.Do(req2)
		if err != nil {
			fmt.Println(err)
			return nil, err
		}
		defer res2.Body.Close()
		body2, err := ioutil.ReadAll(res2.Body)
		if err != nil {
			fmt.Println(err)
			return nil, err
		}
		output2 := map[string]any{}
		err3 := json.Unmarshal(body2, &output2)
		if err3 != nil {
			fmt.Println(err3)
			return nil, err3
		}
		o2, oko2 := output2["code"]
		if !oko2 || (o2.(float64) != 200) {
			errC := errors.New("consuming myket error")
			fmt.Println(errC)
			return nil, errC
		}
	}

	effects := map[string]float64{}
	userData := map[string]interface{}{}

	meta := game_model.Meta{Id: input.GameKey + "::buy"}
	err4 := trx.Db().First(&meta).Error
	if err4 != nil {
		return nil, err4
	}
	val := meta.Data[input.Product].(string)
	data := strings.Split(val, ".")
	for i := range data {
		if i%2 == 0 {
			number, err5 := strconv.ParseFloat(data[i+1], 64)
			if err5 != nil {
				fmt.Println(err5)
				continue
			}
			effects[data[i]] = number
		}
	}
	trx.ClearError()
	gameDataStr := ""
	trx.Db().Model(&model.User{}).Select(adapters.BuildJsonFetcher("metadata", input.GameKey)).Where("id = ?", state.Info().UserId()).First(&gameDataStr)
	trx.ClearError()
	err6 := json.Unmarshal([]byte(gameDataStr), &userData)
	if err6 != nil {
		log.Println(err6)
		return nil, err6
	}

	registerTime := userData["registerDate"].(float64)
	_, ok := userData["firstBuy"]
	if !ok {
		err := adapters.UpdateJson(func() *gorm.DB { return trx.Db().Model(&model.User{}).Where("id = ?", state.Info().UserId()) }, &user, "metadata", input.GameKey+".firstBuy", math.Ceil((float64(time.Now().UnixMilli())-registerTime)/(24*60*60*1000)))
		if err != nil {
			log.Println(err)
			return map[string]any{}, err
		}
	}

	for k, v := range effects {
		if v == 0 {
			continue
		}
		timeKey := "last" + (strings.ToUpper(string(k[0])) + k[1:]) + "Buy"
		now := int64(time.Now().UnixMilli())
		oldValRaw, ok := userData[k]
		if !ok {
			continue
		}
		oldVal := oldValRaw.(float64)
		newVal := v + oldVal
		lastBuyTimeRaw, ok2 := userData[timeKey]
		if k == "chat" && ok2 {
			lastBuyTime := lastBuyTimeRaw.(float64)
			if float64(now) < (lastBuyTime + oldVal) {
				newVal = math.Ceil((v * 24 * 60 * 60 * 1000) + oldVal - (float64(now) - lastBuyTime))
			} else {
				newVal = v * 24 * 60 * 60 * 1000
			}
		}
		err := adapters.UpdateJson(func() *gorm.DB { return trx.Db().Model(&model.User{}).Where("id = ?", state.Info().UserId()) }, &user, "metadata", input.GameKey+"."+k, newVal)
		if err != nil {
			log.Println(err)
			return map[string]any{}, err
		}
		trx.ClearError()
		err2 := adapters.UpdateJson(func() *gorm.DB { return trx.Db().Model(&model.User{}).Where("id = ?", state.Info().UserId()) }, &user, "metadata", input.GameKey+"."+timeKey, now)
		if err2 != nil {
			log.Println(err2)
			return map[string]any{}, err2
		}
		trx.ClearError()
	}
	p := game_model.Payment{Token: input.PurchaseToken, Market: market, Id: crypto.SecureUniqueId(a.Layer.Core().Id()), UserId: state.Info().UserId(), Product: input.Product, GameKey: input.GameKey, Time: time.Now().UnixMilli()}
	trx.Db().Create(&p)
	return map[string]any{}, nil
}
