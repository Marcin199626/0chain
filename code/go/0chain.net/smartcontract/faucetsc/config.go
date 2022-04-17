package faucetsc

import (
	"time"

	"0chain.net/chaincore/config"
	"0chain.net/chaincore/state"
)

//go:generate msgp -io=false -tests=false -v

type Setting int

const (
	PourAmount Setting = iota
	MaxPourAmount
	PeriodicLimit
	GlobalLimit
	IndividualReset
	GlobalReset
	OwnerId
	Cost
)

var (
	Settings = []string{
		"pour_amount",
		"max_pour_amount",
		"periodic_limit",
		"global_limit",
		"individual_reset",
		"global_rest",
		"owner_id",
		"cost",
	}

	costFunctions = []string{
		"update-settings",
		"pour",
		"refill",
	}
)

type FaucetConfig struct {
	PourAmount      state.Balance  `json:"pour_amount"`
	MaxPourAmount   state.Balance  `json:"max_pour_amount"`
	PeriodicLimit   state.Balance  `json:"periodic_limit"`
	GlobalLimit     state.Balance  `json:"global_limit"`
	IndividualReset time.Duration  `json:"individual_reset"`
	GlobalReset     time.Duration  `json:"global_rest"`
	OwnerId         string         `json:"owner_id"`
	Cost            map[string]int `json:"cost"`
}

// configurations from sc.yaml
func getConfig() (conf *FaucetConfig) {
	conf = new(FaucetConfig)
	conf.PourAmount = state.Balance(config.SmartContractConfig.GetFloat64("smart_contracts.faucetsc.pour_amount") * 1e10)
	conf.MaxPourAmount = state.Balance(config.SmartContractConfig.GetFloat64("smart_contracts.faucetsc.max_pour_amount") * 1e10)
	conf.PeriodicLimit = state.Balance(config.SmartContractConfig.GetFloat64("smart_contracts.faucetsc.periodic_limit") * 1e10)
	conf.GlobalLimit = state.Balance(config.SmartContractConfig.GetFloat64("smart_contracts.faucetsc.global_limit") * 1e10)
	conf.IndividualReset = config.SmartContractConfig.GetDuration("smart_contracts.faucetsc.individual_reset")
	conf.GlobalReset = config.SmartContractConfig.GetDuration("smart_contracts.faucetsc.global_reset")
	conf.OwnerId = config.SmartContractConfig.GetString("smart_contracts.faucetsc.owner_id")
	conf.Cost = config.SmartContractConfig.GetStringMapInt("smart_contracts.faucetsc.cost")
	return
}
