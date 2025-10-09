package graphql

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/cordialsys/crosschain/chain/substrate/client/api"
	xcclient "github.com/cordialsys/crosschain/client"
)

type Time struct {
	time.Time
}

func (ct *Time) UnmarshalJSON(data []byte) error {
	str := string(data)
	str = str[1 : len(str)-1] // strip quotes

	// subquery uses multiple custom timestamp formats
	layout := "2006-01-02T15:04:05.000"
	parsedTime, err := time.Parse(layout, str)
	if err == nil {
		ct.Time = parsedTime
		return nil
	}

	layout = "2006-01-02T15:04:05"
	parsedTime, err = time.Parse(layout, str)
	if err == nil {
		ct.Time = parsedTime
		return nil
	}

	// Try to parse as RFC3339Nano
	parsedTime, err = time.Parse(time.RFC3339Nano, str)
	if err == nil {
		ct.Time = parsedTime
		return nil
	}

	// Try to parse as RFC3339
	parsedTime, err = time.Parse(time.RFC3339, str)
	if err != nil {
		return err
	}

	ct.Time = parsedTime
	return nil
}

// GraphQL extrinsic response
type SubqueryExtrinsicResponse struct {
	Data struct {
		Extrinsics struct {
			Nodes []struct {
				ID     api.BlockAndOffset `json:"id"`
				TxHash string             `json:"txHash"`
				Tip    string             `json:"tip"`
			} `json:"nodes"`
		} `json:"extrinsics"`
	} `json:"data"`
}

type SubqueryEvent struct {
	Module string `json:"module"`
	Event  string `json:"event"`
	Data   string `json:"data"`

	parsedParams []interface{} `json:"-"`
}

var _ api.EventI = &SubqueryEvent{}

// GraphQL event response
type SubqueryEventResponse struct {
	Data struct {
		Events struct {
			Nodes []*SubqueryEvent `json:"nodes"`
		} `json:"events"`
		Blocks struct {
			Nodes []struct {
				Timestamp Time   `json:"timestamp"`
				Hash      string `json:"hash"`
			} `json:"nodes"`
		} `json:"blocks"`
	} `json:"data"`
}

func (s *SubqueryEvent) ParseParams() ([]interface{}, error) {
	var paramsPre []json.RawMessage
	var params []interface{}
	err := json.Unmarshal([]byte(s.Data), &paramsPre)
	if err != nil {
		return params, err
	}
	// we have to special case the numbers because graphql serializes big integers
	// as float64 which will run into truncation/precision issues.  so we manually
	// parse them as strings instead.
	for _, p := range paramsPre {
		var badFloat float64
		if err := json.Unmarshal(p, &badFloat); err == nil {
			// bad!  treat as string instead
			params = append(params, string(p))
		} else {
			// use normal deserialization
			var norm interface{}
			_ = json.Unmarshal(p, &norm)
			params = append(params, norm)
		}
	}
	s.parsedParams = params
	return params, nil
}
func (ev *SubqueryEvent) GetEvent() string {
	return ev.Event
}
func (ev *SubqueryEvent) GetModule() string {
	return ev.Module
}
func (ev *SubqueryEvent) GetParam(name string, index int) (interface{}, bool) {
	if len(ev.parsedParams) <= index {
		return nil, false
	}
	return ev.parsedParams[index], true
}
func GetSubqueryParam[T any](ev *SubqueryEvent, index int) (T, error) {
	var zero T
	if len(ev.parsedParams) <= index {
		return zero, fmt.Errorf("event %s.%s does not have expected event at index %d", ev.Module, ev.Event, index)
	}
	value, ok := ev.parsedParams[index].(T)
	if !ok {
		return value, fmt.Errorf("unexpected type for event %s.%s param %d, recieved %T but expected %T", ev.Module, ev.Event, index, ev.parsedParams[index], value)
	}
	return value, nil
}

func (ev *SubqueryEvent) GetEventDescriptor() (*xcclient.Event, bool) {
	return nil, false
}
