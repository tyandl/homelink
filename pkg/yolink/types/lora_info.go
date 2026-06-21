package types

import "fmt"

type LoraInfo struct {
	NetId      string `json:"netId"`
	DevNetType string `json:"devNetType"`
	Signal     int    `json:"signal"`
	GatewayId  string `json:"gatewayId"`
	Gateways   int    `json:"gateways"`
}

func (l LoraInfo) String() string {
	return fmt.Sprintf("{NetId:%s DevNetType:%s Signal:%d GatewayId:%s Gateways:%d}",
		l.NetId, l.DevNetType, l.Signal, l.GatewayId, l.Gateways)
}
