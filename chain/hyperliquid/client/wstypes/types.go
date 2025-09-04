package wstypes

type Notification struct {
	Notification string `json:"notification"`
}

type CoinSubscription struct {
	Type string `json:"type"`
	Coin string `json:"coin"`
}

type Request struct {
	Method       string            `json:"method"`
	Subscription *CoinSubscription `json:"subscription,omitempty"`
}

type Message[T any] struct {
	Channel string `json:"channel"`
	Data    T      `json:"data"`
}

type Trade struct {
	Coin  string   `json:"coin"`
	Side  string   `json:"side"`
	Px    string   `json:"px"`
	Sz    string   `json:"sz"`
	Hash  string   `json:"hash"`
	Time  uint64   `json:"time"`
	Tid   uint64   `json:"tid"`
	Users []string `json:"users"`
}
