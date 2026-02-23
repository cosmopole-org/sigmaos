package actions_report

import (
	"errors"
	"kasper/src/abstract"
	inputs_report "kasper/src/plugins/social/inputs/report"
	"kasper/src/plugins/social/model"
	outputs_report "kasper/src/plugins/social/outputs/report"
	"kasper/src/shell/layer1/adapters"
	module_state "kasper/src/shell/layer1/module/state"
	"kasper/src/shell/utils/crypto"
)

type Actions struct {
	Layer abstract.ILayer
}

func Install(s adapters.IStorage, a *Actions) error {
	return s.AutoMigrate(&model.Report{})
}

// Report /report/report check [ true false false ] access [ true false false false PUT ]
func (a *Actions) Report(s abstract.IState, input inputs_report.ReportInput) (any, error) {
	state := abstract.UseState[module_state.IStateL1](s)
	trx := state.Trx()
	typRaw, ok := input.Data["type"]
	if ok {
		typ, ok2 := typRaw.(string)
		if ok2 {
			if typ == "requestUnban" {
				reps := []model.Report{}
				trx.Db().Where("reporter_id = ?", state.Info().UserId()).Find(&reps)
				for _, rep := range reps {
					typRawRep, ok := rep.Data["type"]
					if ok {
						typRep, ok2 := typRawRep.(string)
						if ok2 {
							if typRep == "requestUnban" {
								return nil, errors.New("unban request already exists")
							}
						}
					}
				}
			}
		}
	}
	report := model.Report{GameKey: "game", Id: crypto.SecureUniqueId(a.Layer.Core().Id()), ReporterId: state.Info().UserId(), Data: input.Data}
	err := trx.Db().Create(&report).Error
	if err != nil {
		return nil, err
	}
	return outputs_report.ReportOutput{Report: report}, nil
}
