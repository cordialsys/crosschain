package sui

import (
	"context"
	"errors"
	"fmt"
	"slices"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/go-sui-sdk/v2/move_types"
	"github.com/cordialsys/go-sui-sdk/v2/sui_types"
	"github.com/cordialsys/go-sui-sdk/v2/types"
	"github.com/sirupsen/logrus"
)

var _ xclient.StakingClient = &Client{}

func TestFetchUnstakeInput(t *testing.T) {
	vectors := []struct {
		name      string
		responses []string
	}{
		{
			name: "TestFullUnstake",
			responses: []string{
				"{'jsonrpc':'2.0','id':1,'result':[{'validatorAddress':'0x7e99c532dc9be514cf98650e6e80dd5a8d859eb298cc63f1bd18222694cbda1d','stakingPool':'0x3a307858c118155331c1aa48a90c3d871ca03263f9cd85c2e8d4a75d93003b1c','stakes':[{'stakedSuiId':'0x081fff51e64571ecd345f6481460de974c9f9700b9bf830d957636df4e3393a5','stakeRequestEpoch':'9','stakeActiveEpoch':'10','principal':'2000000000','status':'Active','estimatedReward':'80003040'},{'stakedSuiId':'0xf962d5dce6f16fd920f4de9cf53b33f30364ac1f8c5d8829aff2d3dc8e9db56b','stakeRequestEpoch':'14','stakeActiveEpoch':'15','principal':'1000000000','status':'Pending'},{'stakedSuiId':'0xfa9941e617e8e30c77d38b76ac6d2b03fc66b10c5fa2a0011f632bc7bb3fb9e6','stakeRequestEpoch':'14','stakeActiveEpoch':'15','principal':'2800000000','status':'Pending'}]}]}",
			},
		},
	}
}
