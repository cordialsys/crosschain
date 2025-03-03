package ton_test

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder/buildertest"
	"github.com/cordialsys/crosschain/chain/ton"
	"github.com/cordialsys/crosschain/chain/ton/api"
	xcclient "github.com/cordialsys/crosschain/client"
	testtypes "github.com/cordialsys/crosschain/testutil/types"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

func fromHex(s string) []byte {
	s = strings.TrimPrefix(s, "0x")
	bz, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return bz
}

// reserialize will drop internal fields set by constructors
func reserialize(tx *xcclient.TxInfo) *xcclient.TxInfo {
	bz, _ := json.Marshal(tx)
	var info xcclient.TxInfo
	json.Unmarshal(bz, &info)
	return &info
}

func TestFetchTxInput(t *testing.T) {

	chain := xc.NewChainConfig(xc.TON).WithDecimals(9)
	vectors := []struct {
		asset      *xc.ChainConfig
		contract   xc.ContractAddress
		desc       string
		resp       interface{}
		txInput    *ton.TxInput
		err        string
		httpStatus int
	}{
		{
			asset: chain,
			resp: []string{
				// get account
				`{"balance":"587833680","code":"te6cckEBAQEAcQAA3v8AIN0gggFMl7ohggEznLqxn3Gw7UTQ0x/THzHXC//jBOCk8mCDCNcYINMf0x/TH/gjE7vyY+1E0NMf0x/T/9FRMrryoVFEuvKiBPkBVBBV+RDyo/gAkyDXSpbTB9QC+wDo0QGkyMsfyx/L/8ntVBC9ba0=","data":"te6cckEBAQEAKgAAUAAAABEpqaMXwRcreSYRbSo5a9fWm5iAzAZX6LotufYrTCEMUYMhyLF+KRtN","last_transaction_lt":"23693722000001","last_transaction_hash":"mVuNwFVC4eIWjS+lIAkfinkXUQz8k1lqFZ+lQqvAZK8=","frozen_hash":null,"status":"active"}`,
				// get sequence
				`{"gas_used":549,"exit_code":0,"stack":[{"type":"num","value":"0x11"}]}`,
				// get public-key
				`{"gas_used":549,"exit_code":0,"stack":[{"type":"num","value":"0xc1172b7926116d2a396bd7d69b9880cc0657e8ba2db9f62b4c210c518321c8b1"}]}`,
			},
			txInput: &ton.TxInput{
				TxInputEnvelope: ton.NewTxInput().TxInputEnvelope,
				AccountStatus:   api.Active,
				Sequence:        0x11,
				PublicKey:       fromHex("0xc1172b7926116d2a396bd7d69b9880cc0657e8ba2db9f62b4c210c518321c8b1"),
				Memo:            "",
				Timestamp:       0,
				TokenWallet:     "",
				EstimatedMaxFee: xc.AmountBlockchain{},
				TonBalance:      xc.NewAmountBlockchainFromStr("587833680"),
			},
		},
		{
			asset: chain,
			desc:  "no_public_key_uninit",
			resp: []string{
				// get account
				`{"balance":"587833680","code":"te6cckEBAQEAcQAA3v8AIN0gggFMl7ohggEznLqxn3Gw7UTQ0x/THzHXC//jBOCk8mCDCNcYINMf0x/TH/gjE7vyY+1E0NMf0x/T/9FRMrryoVFEuvKiBPkBVBBV+RDyo/gAkyDXSpbTB9QC+wDo0QGkyMsfyx/L/8ntVBC9ba0=","data":"te6cckEBAQEAKgAAUAAAABEpqaMXwRcreSYRbSo5a9fWm5iAzAZX6LotufYrTCEMUYMhyLF+KRtN","last_transaction_lt":"23693722000001","last_transaction_hash":"mVuNwFVC4eIWjS+lIAkfinkXUQz8k1lqFZ+lQqvAZK8=","frozen_hash":null,"status":"uninit"}`,
				// get sequence
				`{"gas_used":549,"exit_code":0,"stack":[{"type":"num","value":"0x11"}]}`,
				// get public-key
				`{"gas_used":549,"exit_code":-14,"stack":[{"type":"num","value":"0x0"}]}`,
			},
			txInput: &ton.TxInput{
				TxInputEnvelope: ton.NewTxInput().TxInputEnvelope,
				AccountStatus:   api.Uninit,
				Sequence:        0x11,
				PublicKey:       nil,
				Memo:            "",
				Timestamp:       0,
				TokenWallet:     "",
				EstimatedMaxFee: xc.AmountBlockchain{},
				TonBalance:      xc.NewAmountBlockchainFromStr("587833680"),
			},
		},
		{
			desc:     "fetch_token_info",
			asset:    chain,
			contract: xc.ContractAddress("kQAiboDEv_qRrcEdrYdwbVLNOXBHwShFbtKGbQVJ2OKxY_Di"),
			resp: []string{
				// get account
				`{"balance":"587833680","code":"te6cckEBAQEAcQAA3v8AIN0gggFMl7ohggEznLqxn3Gw7UTQ0x/THzHXC//jBOCk8mCDCNcYINMf0x/TH/gjE7vyY+1E0NMf0x/T/9FRMrryoVFEuvKiBPkBVBBV+RDyo/gAkyDXSpbTB9QC+wDo0QGkyMsfyx/L/8ntVBC9ba0=","data":"te6cckEBAQEAKgAAUAAAABEpqaMXwRcreSYRbSo5a9fWm5iAzAZX6LotufYrTCEMUYMhyLF+KRtN","last_transaction_lt":"23693722000001","last_transaction_hash":"mVuNwFVC4eIWjS+lIAkfinkXUQz8k1lqFZ+lQqvAZK8=","frozen_hash":null,"status":"uninit"}`,
				// get sequence
				`{"gas_used":549,"exit_code":0,"stack":[{"type":"num","value":"0x11"}]}`,
				// get jetton data
				`{"gas_used":685,"exit_code":0,"stack":[{"type":"num","value":"0xc9f2c9cd04674ede89e2be380"},{"type":"num","value":"-0x1"},{"type":"cell","value":"te6cckEBAQEAJAAAQ4AAkIGoV1JlPdxhyj449EPdlQBht2uKB2Tr/j6D/wCdMjAO5J3r"},{"type":"cell","value":"te6cckEBDAEA0AABAwDAAQIBWAIDAgEgBAUCASAICQFBv0VGpv/ht5z92GutPbh0MT3N4vsF5qdKp/NVLZYXx50TBgFBv27U+UKnhIziywZrd6ESjGof+MQ/Q4otziRhK6n/q4sDBwAMAEFpb3R4AAwAQUlPVFgBQb9SCN70b1odT53OZqswn0qFEwXxZvke952SPvWONPmiCQoBQb9dAfpePAaQHEUEbGst3Opa92T+oO7XKhDUBPIxLOskfQsALABUZXN0bmV0IHRva2VuIHRvIHRlc3QABAA5QJmctQ=="},{"type":"cell","value":"te6cckECEQEAAyMAART/APSkE/S88sgLAQIBYgIDAgLMBAUAG6D2BdqJofQB9IH0gahhAgHUBgcCASAICQDDCDHAJJfBOAB0NMDAXGwlRNfA/AM4PpA+kAx+gAxcdch+gAx+gAwc6m0AALTH4IQD4p+pVIgupUxNFnwCeCCEBeNRRlSILqWMUREA/AK4DWCEFlfB7y6k1nwC+BfBIQP8vCAAET6RDBwuvLhTYAIBIAoLAIPUAQa5D2omh9AH0gfSBqGAJpj8EIC8aijKkQXUEIPe7L7wndCVj5cWLpn5j9ABgJ0CgR5CgCfQEsZ4sA54tmZPaqQB8VA9M/+gD6QCHwAe1E0PoA+kD6QNQwUTahUirHBfLiwSjC//LiwlQ0QnBUIBNUFAPIUAT6AljPFgHPFszJIsjLARL0APQAywDJIPkAcHTIywLKB8v/ydAE+kD0BDH6ACDXScIA8uLEd4AYyMsFUAjPFnD6AhfLaxPMgMAgEgDQ4AnoIQF41FGcjLHxnLP1AH+gIizxZQBs8WJfoCUAPPFslQBcwjkXKRceJQCKgToIIJycOAoBS88uLFBMmAQPsAECPIUAT6AljPFgHPFszJ7VQC9ztRND6APpA+kDUMAjTP/oAUVGgBfpA+kBTW8cFVHNtcFQgE1QUA8hQBPoCWM8WAc8WzMkiyMsBEvQA9ADLAMn5AHB0yMsCygfL/8nQUA3HBRyx8uLDCvoAUaihggiYloBmtgihggiYloCgGKEnlxBJEDg3XwTjDSXXCwGAPEADXO1E0PoA+kD6QNQwB9M/+gD6QDBRUaFSSccF8uLBJ8L/8uLCBYIJMS0AoBa88uLDghB73ZfeyMsfFcs/UAP6AiLPFgHPFslxgBjIywUkzxZw+gLLaszJgED7AEATyFAE+gJYzxYBzxbMye1UgAHBSeaAYoYIQc2LQnMjLH1Iwyz9Y+gJQB88WUAfPFslxgBDIywUkzxZQBvoCFctqFMzJcfsAECQQIwB8wwAjwgCwjiGCENUydttwgBDIywVQCM8WUAT6AhbLahLLHxLLP8ly+wCTNWwh4gPIUAT6AljPFgHPFszJ7VSV6u3X"}]}`,
				// get token wallet
				`{"gas_used":549,"exit_code":0,"stack":[{"type":"num","value":"te6cckEBAQEAJAAAQ4AHpoFiodWAcl6+hRV/++eGd9rQjRT+/sD4lCewAn3q1jDw9q/m"}]}`,
				// estimate fees
				`{"source_fees":{"in_fwd_fee":699200,"storage_fee":3549,"gas_fee":0,"fwd_fee":0},"destination_fees":[]}`,
				// get public-key
				`{"gas_used":549,"exit_code":-14,"stack":[{"type":"num","value":"0x0"}]}`,
			},
			txInput: &ton.TxInput{
				TxInputEnvelope:  ton.NewTxInput().TxInputEnvelope,
				AccountStatus:    api.Uninit,
				Sequence:         0x11,
				PublicKey:        nil,
				Memo:             "",
				Timestamp:        0,
				TokenWallet:      "EQA9NAsVDqwDkvX0KKv_3zwzvtaEaKf39gfEoT2AE-9WsVbM",
				JettonWalletCode: fromHex("b5ee9c7241021101000323000114ff00f4a413f4bcf2c80b0102016202030202cc0405001ba0f605da89a1f401f481f481a8610201d40607020120080900c30831c02497c138007434c0c05c6c2544d7c0fc03383e903e900c7e800c5c75c87e800c7e800c1cea6d0000b4c7e08403e29fa954882ea54c4d167c0278208405e3514654882ea58c511100fc02b80d60841657c1ef2ea4d67c02f817c12103fcbc2000113e910c1c2ebcb853600201200a0b0083d40106b90f6a2687d007d207d206a1802698fc1080bc6a28ca9105d41083deecbef09dd0958f97162e99f98fd001809d02811e428027d012c678b00e78b6664f6aa401f1503d33ffa00fa4021f001ed44d0fa00fa40fa40d4305136a1522ac705f2e2c128c2fff2e2c254344270542013541403c85004fa0258cf1601cf16ccc922c8cb0112f400f400cb00c920f9007074c8cb02ca07cbffc9d004fa40f40431fa0020d749c200f2e2c4778018c8cb055008cf1670fa0217cb6b13cc80c0201200d0e009e8210178d4519c8cb1f19cb3f5007fa0222cf165006cf1625fa025003cf16c95005cc2391729171e25008a813a08209c9c380a014bcf2e2c504c98040fb001023c85004fa0258cf1601cf16ccc9ed5402f73b51343e803e903e90350c0234cffe80145468017e903e9014d6f1c1551cdb5c150804d50500f214013e809633c58073c5b33248b232c044bd003d0032c0327e401c1d3232c0b281f2fff274140371c1472c7cb8b0c2be80146a2860822625a019ad822860822625a028062849e5c412440e0dd7c138c34975c2c0600f1000d73b51343e803e903e90350c01f4cffe803e900c145468549271c17cb8b049f0bffcb8b08160824c4b402805af3cb8b0e0841ef765f7b232c7c572cfd400fe8088b3c58073c5b25c60063232c14933c59c3e80b2dab33260103ec01004f214013e809633c58073c5b3327b552000705279a018a182107362d09cc8cb1f5230cb3f58fa025007cf165007cf16c9718010c8cb0524cf165006fa0215cb6a14ccc971fb0010241023007cc30023c200b08e218210d53276db708010c8cb055008cf165004fa0216cb6a12cb1f12cb3fc972fb0093356c21e203c85004fa0258cf1601cf16ccc9ed5495eaedd7"),
				EstimatedMaxFee:  xc.NewAmountBlockchainFromUint64(10 * (699200 + 3549)),
				TonBalance:       xc.NewAmountBlockchainFromStr("587833680"),
			},
		},
		{
			desc:     "fetch_token_info_missing_token_wallet",
			asset:    chain,
			contract: xc.ContractAddress("kQAiboDEv_qRrcEdrYdwbVLNOXBHwShFbtKGbQVJ2OKxY_Di"),
			resp: []string{
				// get account
				`{"balance":"587833680","code":"te6cckEBAQEAcQAA3v8AIN0gggFMl7ohggEznLqxn3Gw7UTQ0x/THzHXC//jBOCk8mCDCNcYINMf0x/TH/gjE7vyY+1E0NMf0x/T/9FRMrryoVFEuvKiBPkBVBBV+RDyo/gAkyDXSpbTB9QC+wDo0QGkyMsfyx/L/8ntVBC9ba0=","data":"te6cckEBAQEAKgAAUAAAABEpqaMXwRcreSYRbSo5a9fWm5iAzAZX6LotufYrTCEMUYMhyLF+KRtN","last_transaction_lt":"23693722000001","last_transaction_hash":"mVuNwFVC4eIWjS+lIAkfinkXUQz8k1lqFZ+lQqvAZK8=","frozen_hash":null,"status":"uninit"}`,
				// get sequence
				`{"gas_used":549,"exit_code":0,"stack":[{"type":"num","value":"0x11"}]}`,
				// get token wallet
				`{"gas_used":549,"exit_code":-13,"stack":[{"type":"num","value":""}]}`,
			},
			err: "could not lookup jetton wallet code",
		},
		{
			desc:     "reports_error",
			asset:    chain,
			contract: xc.ContractAddress("kQAiboDEv_qRrcEdrYdwbVLNOXBHwShFbtKGbQVJ2OKxY_Di"),

			resp: []string{
				// get account
				`{"error":"bad stuff"}`,
			},
			httpStatus: 400,
			err:        "bad stuff",
		},
	}
	for i, v := range vectors {
		t.Run(fmt.Sprintf("testcase_%d_%s", i, v.desc), func(t *testing.T) {
			httpStatus := 200
			if v.httpStatus > 0 {
				httpStatus = v.httpStatus
			}
			server, close := testtypes.MockHTTP(t, v.resp, httpStatus)
			defer close()
			chain.URL = server.URL
			v.asset.URL = server.URL
			v.asset.GetChain().Limiter = rate.NewLimiter(rate.Inf, 1)

			client, err := ton.NewClient(v.asset)
			require.NoError(t, err)
			from := xc.Address("EQAjflEZ_6KgKMxPlcnKN1ZoUvHdTT6hVwTW95EGVQfeSha2")
			to := xc.Address("0QChotyiAtSPqs0BbPD851Mys9_LdMVM7N-atsFYvUMc48Jm")

			amount := xc.NewAmountBlockchainFromUint64(1)
			args := buildertest.MustNewTransferArgs(from, to, amount)
			if v.contract != "" {
				args.SetContract(v.contract)
			}

			input, err := client.FetchTransferInput(context.Background(), args)

			if v.err != "" {
				require.ErrorContains(t, err, v.err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, input)
				input.(xc.TxInputWithUnix).SetUnix(0)
				require.Equal(t, v.txInput, input)
			}
		})
	}
}

func TestFetchTxInfo(t *testing.T) {

	chain := xc.NewChainConfig(xc.TON).WithDecimals(9)
	vectors := []struct {
		hash       string
		desc       string
		resp       interface{}
		tx         *xcclient.TxInfo
		err        string
		httpStatus int
	}{
		{
			desc: "get_tx",
			hash: "5a4431eb12a936144130c7c75f292170f92749cf08b8d7259821171418a5cbef",
			resp: []string{
				// get chain info
				`{"last":{"workchain":-1,"shard":"8000000000000000","seqno":21082664,"root_hash":"SMroEPt+MFtk85CpRUmyeogmrVDmHa6WbJm9Wz9OmMA=","file_hash":"TRya8nOmld4LaZVKlJC3Kq1apB4a4HmVYOxewte6a/k=","global_id":-3,"version":0,"after_merge":false,"before_split":false,"after_split":false,"want_merge":true,"want_split":false,"key_block":false,"vert_seqno_incr":false,"flags":1,"gen_utime":"1721068768","start_lt":"23694519000000","end_lt":"23694519000004","validator_list_hash_short":197321932,"gen_catchain_seqno":288848,"min_ref_mc_seqno":21082657,"prev_key_block_seqno":21082243,"vert_seqno":0,"master_ref_seqno":0,"rand_seed":"dN8oWdq3z/UfiufHNqwjHAA2J7fDhuqsZjz4ZsDKMMo=","created_by":"EIs7uyFACFwaIqs9Jw3Rm0LmtoEkV6GkIr/y9gnE/hk=","tx_count":3,"masterchain_block_ref":{"workchain":-1,"shard":"8000000000000000","seqno":21082664},"prev_blocks":[{"workchain":-1,"shard":"8000000000000000","seqno":21082663}]},"first":{"workchain":-1,"shard":"8000000000000000","seqno":3,"root_hash":"N1MtB3dREOndUsEfXY6U7EUmgG7KTawIjoeM69iLuCc=","file_hash":"MaP4koxBb5lcYfR9ubrBRUCyE7SEeakagA9Tg7aKK+A=","global_id":-3,"version":0,"after_merge":false,"before_split":false,"after_split":false,"want_merge":false,"want_split":false,"key_block":false,"vert_seqno_incr":false,"flags":1,"gen_utime":"1653238862","start_lt":"3000000","end_lt":"3000004","validator_list_hash_short":1253667756,"gen_catchain_seqno":0,"min_ref_mc_seqno":1,"prev_key_block_seqno":0,"vert_seqno":0,"master_ref_seqno":0,"rand_seed":"VyIDzkSrtLP+ji2OzWNhBmDZuPHdCDdeT8B/bhiwFuE=","created_by":"Bu4LLZ5LqTqQFFgS1P0DR4Fay0jcqNu1N34tZ9TFjMo=","tx_count":3,"masterchain_block_ref":{"workchain":-1,"shard":"8000000000000000","seqno":3},"prev_blocks":[{"workchain":-1,"shard":"8000000000000000","seqno":2}]}}`,
				// get transactions
				`{"transactions":[{"account":"0:237E5119FFA2A028CC4F95C9CA37566852F1DD4D3EA15704D6F791065507DE4A","hash":"zDx7EjkQvC/Bi2QM5yqmaKDrKf+tN20o3k8u8/e6T3E=","lt":"23692407000001","now":1721063820,"orig_status":"active","end_status":"active","total_fees":"1960116","prev_trans_hash":"A2KdHQ6PD7mcRfmWKIoiPNFFooBCzLOCFXoj+4hAjh0=","prev_trans_lt":"23688590000001","description":{"type":"ord","action":{"valid":true,"success":true,"no_funds":false,"result_code":0,"tot_actions":1,"msgs_created":1,"spec_actions":0,"tot_msg_size":{"bits":"761","cells":"1"},"status_change":"unchanged","total_fwd_fees":"400000","skipped_actions":0,"action_list_hash":"XSD5j5fbmBMcz2qKGqBU7K8PcybknHZls74NH2I+Qo0=","total_action_fees":"133331"},"aborted":false,"credit_ph":{"credit":"16298225961743024383"},"destroyed":false,"compute_ph":{"mode":0,"type":"vm","success":true,"gas_fees":"1197600","gas_used":"2994","vm_steps":66,"exit_code":0,"gas_limit":"0","gas_credit":"10000","msg_state_used":false,"account_activated":false,"vm_init_state_hash":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=","vm_final_state_hash":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="},"storage_ph":{"status_change":"unchanged","storage_fees_collected":"385"},"credit_first":true},"block_ref":{"workchain":0,"shard":"2000000000000000","seqno":22624217},"in_msg":{"hash":"WkQx6xKpNhRBMMfHXykhcPknSc8IuNclmCEXFBily+8=","source":null,"destination":"0:237E5119FFA2A028CC4F95C9CA37566852F1DD4D3EA15704D6F791065507DE4A","value":null,"fwd_fee":null,"ihr_fee":null,"created_lt":null,"created_at":null,"opcode":"0x3235ff9b","ihr_disabled":null,"bounce":null,"bounced":null,"import_fee":"0","message_content":{"hash":"pSgu2pfIEFeWo31xHfheCFt4EG+mSA2vEvMSq/K/PXo=","body":"te6cckEBAgEAjQABmjI1/5uuQ3CjJrTQdqqr0xo0vViP5AYh+2aKxqNmwmRYM3Kx99Qdft6cPlj64q88NjfJNEyxCLVcdWRegP7/mw8pqaMXZpV1qgAAAA0DAQB2QgBQ0W5RAWpH1WaAtnh+c6mZWe/lumKmdm/NW2CsXqGOcaAKfYwAAAAAAAAAAAAAAAAAAAAAAABoaWkpu9mU","decoded":null},"init_state":null},"out_msgs":[{"hash":"XexvuJ+FOdF7mNAC0/zjXEtrwO6Kh+XRlTa+FinuLuI=","source":"0:237E5119FFA2A028CC4F95C9CA37566852F1DD4D3EA15704D6F791065507DE4A","destination":"0:A1A2DCA202D48FAACD016CF0FCE75332B3DFCB74C54CECDF9AB6C158BD431CE3","value":"22000000","fwd_fee":"266669","ihr_fee":"0","created_lt":"23692407000002","created_at":"1721063820","opcode":"0x00000000","ihr_disabled":true,"bounce":false,"bounced":false,"import_fee":null,"message_content":{"hash":"JwaRlXkBSbE/FgE1Cic6QMP4p4grFvQ05deNWMgRx2E=","body":"te6cckEBAQEACQAADgAAAABoaWne1AAn","decoded":{"type":"text_comment","comment":"hii"}},"init_state":null}],"account_state_before":{"hash":"Bzi9TGeoGnU0mzAFJmVZYk4jSALMK946n9dpq33SdwQ=","balance":"684739799","account_status":"active","frozen_hash":null,"code_hash":"hNr6RJ+Ypph3ibojI1gHK8D3bcRSQAKl0JGLmnXS1Zk=","data_hash":"frmHnmWTx6frq0IYa2yjcUZfEI3BXT25xvIEKzK9nUw="},"account_state_after":{"hash":"RgAvzmATiYRNS8GJ102gwwSjwJmQ0ZBBUHXUkayIIiE=","balance":"660513014","account_status":"active","frozen_hash":null,"code_hash":"hNr6RJ+Ypph3ibojI1gHK8D3bcRSQAKl0JGLmnXS1Zk=","data_hash":"NrUyclIYraMXbxHHxy6NrK40cmR1sFexCd6KWC9eRqM="},"mc_block_seqno":21080779}],"address_book":{"0:237E5119FFA2A028CC4F95C9CA37566852F1DD4D3EA15704D6F791065507DE4A":{"user_friendly":"0QAjflEZ_6KgKMxPlcnKN1ZoUvHdTT6hVwTW95EGVQfeSvD5"},"0:A1A2DCA202D48FAACD016CF0FCE75332B3DFCB74C54CECDF9AB6C158BD431CE3":{"user_friendly":"0QChotyiAtSPqs0BbPD851Mys9_LdMVM7N-atsFYvUMc48Jm"}}}`,
			},
			tx: &xcclient.TxInfo{
				Name:   "chains/TON/transactions/5a4431eb12a936144130c7c75f292170f92749cf08b8d7259821171418a5cbef",
				Hash:   "5a4431eb12a936144130c7c75f292170f92749cf08b8d7259821171418a5cbef",
				XChain: xc.TON,
				State:  xcclient.Succeeded,
				Final:  true,
				Block: &xcclient.Block{
					Chain:  "TON",
					Height: xc.NewAmountBlockchainFromUint64(21080779),
					Hash:   "0:2000000000000000:22624217",
					Time:   time.Unix(1721063820, 0),
				},
				Movements: []*xcclient.Movement{
					{
						XAsset:    "chains/TON/assets/TON",
						XContract: "TON",
						AssetId:   "TON",
						From: []*xcclient.BalanceChange{
							{

								Balance:   xc.NewAmountBlockchainFromUint64(22000000),
								XAddress:  xcclient.NewAddressName(xc.TON, "0QAjflEZ_6KgKMxPlcnKN1ZoUvHdTT6hVwTW95EGVQfeSvD5"),
								AddressId: "0QAjflEZ_6KgKMxPlcnKN1ZoUvHdTT6hVwTW95EGVQfeSvD5",
							},
						},
						To: []*xcclient.BalanceChange{
							{

								Balance:   xc.NewAmountBlockchainFromUint64(22000000),
								XAddress:  xcclient.NewAddressName(xc.TON, "0QChotyiAtSPqs0BbPD851Mys9_LdMVM7N-atsFYvUMc48Jm"),
								AddressId: "0QChotyiAtSPqs0BbPD851Mys9_LdMVM7N-atsFYvUMc48Jm",
							},
						},
						Memo: "hii",
					},
					// fee
					{
						XAsset:    "chains/TON/assets/TON",
						XContract: "TON",
						AssetId:   "TON",
						To:        []*xcclient.BalanceChange{},
						From: []*xcclient.BalanceChange{
							{

								Balance:   xc.NewAmountBlockchainFromUint64(1960116),
								XAddress:  xcclient.NewAddressName(xc.TON, "0QAjflEZ_6KgKMxPlcnKN1ZoUvHdTT6hVwTW95EGVQfeSvD5"),
								AddressId: "0QAjflEZ_6KgKMxPlcnKN1ZoUvHdTT6hVwTW95EGVQfeSvD5",
							},
						},
					},
				},
				Confirmations: 1885,
			},
		},
		{
			desc: "get_token_tx",
			hash: "d464953f3bfd87058b0f3b531cbce11b77921387642e7eabccdd20e6ff6e80cb",
			resp: []string{
				// get chain info
				`{"last":{"workchain":-1,"shard":"8000000000000000","seqno":21082664,"root_hash":"SMroEPt+MFtk85CpRUmyeogmrVDmHa6WbJm9Wz9OmMA=","file_hash":"TRya8nOmld4LaZVKlJC3Kq1apB4a4HmVYOxewte6a/k=","global_id":-3,"version":0,"after_merge":false,"before_split":false,"after_split":false,"want_merge":true,"want_split":false,"key_block":false,"vert_seqno_incr":false,"flags":1,"gen_utime":"1721068768","start_lt":"23694519000000","end_lt":"23694519000004","validator_list_hash_short":197321932,"gen_catchain_seqno":288848,"min_ref_mc_seqno":21082657,"prev_key_block_seqno":21082243,"vert_seqno":0,"master_ref_seqno":0,"rand_seed":"dN8oWdq3z/UfiufHNqwjHAA2J7fDhuqsZjz4ZsDKMMo=","created_by":"EIs7uyFACFwaIqs9Jw3Rm0LmtoEkV6GkIr/y9gnE/hk=","tx_count":3,"masterchain_block_ref":{"workchain":-1,"shard":"8000000000000000","seqno":21082664},"prev_blocks":[{"workchain":-1,"shard":"8000000000000000","seqno":21082663}]},"first":{"workchain":-1,"shard":"8000000000000000","seqno":3,"root_hash":"N1MtB3dREOndUsEfXY6U7EUmgG7KTawIjoeM69iLuCc=","file_hash":"MaP4koxBb5lcYfR9ubrBRUCyE7SEeakagA9Tg7aKK+A=","global_id":-3,"version":0,"after_merge":false,"before_split":false,"after_split":false,"want_merge":false,"want_split":false,"key_block":false,"vert_seqno_incr":false,"flags":1,"gen_utime":"1653238862","start_lt":"3000000","end_lt":"3000004","validator_list_hash_short":1253667756,"gen_catchain_seqno":0,"min_ref_mc_seqno":1,"prev_key_block_seqno":0,"vert_seqno":0,"master_ref_seqno":0,"rand_seed":"VyIDzkSrtLP+ji2OzWNhBmDZuPHdCDdeT8B/bhiwFuE=","created_by":"Bu4LLZ5LqTqQFFgS1P0DR4Fay0jcqNu1N34tZ9TFjMo=","tx_count":3,"masterchain_block_ref":{"workchain":-1,"shard":"8000000000000000","seqno":3},"prev_blocks":[{"workchain":-1,"shard":"8000000000000000","seqno":2}]}}`,
				// not found via msg-hash index
				`{"transactions":[],"address_book":{}}`,
				// get transactions (normal index)
				`{"transactions":[{"account":"0:237E5119FFA2A028CC4F95C9CA37566852F1DD4D3EA15704D6F791065507DE4A","hash":"1GSVPzv9hwWLDztTHLzhG3eSE4dkLn6rzN0g5v9ugMs=","lt":"23694319000001","now":1721068303,"orig_status":"active","end_status":"active","total_fees":"2432322","prev_trans_hash":"ZIl+I3MgozmsPM20J2tQEC6ZD8uAqs2YOI/YbdxKzZ0=","prev_trans_lt":"23693735000001","description":{"type":"ord","action":{"valid":true,"success":true,"no_funds":false,"result_code":0,"tot_actions":1,"msgs_created":1,"spec_actions":0,"tot_msg_size":{"bits":"1433","cells":"3"},"status_change":"unchanged","total_fwd_fees":"771200","skipped_actions":0,"action_list_hash":"09sug8ZnYsaxKQ4Oig3JvW+b5zmI0nrrGrKVacNY3fs=","total_action_fees":"257062"},"aborted":false,"credit_ph":{"credit":"14424983580017492223"},"destroyed":false,"compute_ph":{"mode":0,"type":"vm","success":true,"gas_fees":"1197600","gas_used":"2994","vm_steps":66,"exit_code":0,"gas_limit":"0","gas_credit":"10000","msg_state_used":false,"account_activated":false,"vm_init_state_hash":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=","vm_final_state_hash":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="},"storage_ph":{"status_change":"unchanged","storage_fees_collected":"60"},"credit_first":true},"block_ref":{"workchain":0,"shard":"2000000000000000","seqno":22626042},"in_msg":{"hash":"ziwO6w3WnVpdEIhp+in5lDBV5CwSe2clautu3baIAk8=","source":null,"destination":"0:237E5119FFA2A028CC4F95C9CA37566852F1DD4D3EA15704D6F791065507DE4A","value":null,"fwd_fee":null,"ihr_fee":null,"created_lt":null,"created_at":null,"opcode":"0x34dc0fd1","ihr_disabled":null,"bounce":null,"bounced":null,"import_fee":"0","message_content":{"hash":"OdbHlUGU/Ta17opAIScvC3fLs7jLGQSnP0NjPDBH/V4=","body":"te6cckEBBAEA5wABmjTcD9FZwSoMNOO73pTZoc4G/qQ3Bv5UBtKx3aoqECRKL4FqCCheex116bIoj9gJyYT6QMobi2XtPAOwg5QWfQQpqaMXZpWHJwAAABIDAQFoYgAemgWKh1YByXr6FFX/754Z32tCNFP7+wPiUJ7ACferWKBfXhAAAAAAAAAAAAAAAAAAAQIBqA+KfqUAAAAAZpVrB0AU+xgIAUNFuUQFqR9VmgLZ4fnOpmVnv5bpipnZvzVtgrF6hjnHAAjflEZ/6KgKMxPlcnKN1ZoUvHdTT6hVwTW95EGVQfeSoAMADgAAAABoaWk0d0kJ","decoded":null},"init_state":null},"out_msgs":[{"hash":"uO68NVBHlx6vHJ7IqIIp0qfeDszXtUkQi5bJoSPWL8g=","source":"0:237E5119FFA2A028CC4F95C9CA37566852F1DD4D3EA15704D6F791065507DE4A","destination":"0:3D340B150EAC0392F5F428ABFFDF3C33BED68468A7F7F607C4A13D8013EF56B1","value":"200000000","fwd_fee":"514138","ihr_fee":"0","created_lt":"23694319000002","created_at":"1721068303","opcode":"0x0f8a7ea5","ihr_disabled":true,"bounce":true,"bounced":false,"import_fee":null,"message_content":{"hash":"ulACZHpEYu49vd9fRZbSv25dPlQVqyuTtaFcC+Sm2Co=","body":"te6cckEBAgEAYAABqA+KfqUAAAAAZpVrB0AU+xgIAUNFuUQFqR9VmgLZ4fnOpmVnv5bpipnZvzVtgrF6hjnHAAjflEZ/6KgKMxPlcnKN1ZoUvHdTT6hVwTW95EGVQfeSoAEADgAAAABoaWmTsiGj","decoded":null},"init_state":null}],"account_state_before":{"hash":"fvFnh/lTSqR5ek9cLBhwdWwt9gCX7w7fTbPbJLf8644=","balance":"563607278","account_status":"active","frozen_hash":null,"code_hash":"hNr6RJ+Ypph3ibojI1gHK8D3bcRSQAKl0JGLmnXS1Zk=","data_hash":"owTZCJ7DHIY5GC0P8u6S+1kZG49GbjbB5Ag5cI+MiQE="},"account_state_after":{"hash":"QqCCPF1ZarYkIqG0GPEfW7LOoHlHgl8AukzQiuaDqXo=","balance":"360660818","account_status":"active","frozen_hash":null,"code_hash":"hNr6RJ+Ypph3ibojI1gHK8D3bcRSQAKl0JGLmnXS1Zk=","data_hash":"Kz4Q0d9/FDEF4fBZLVi9XeGZUnu8g5iXEeLf0LPS7Yg="},"mc_block_seqno":21082496}],"address_book":{"0:237E5119FFA2A028CC4F95C9CA37566852F1DD4D3EA15704D6F791065507DE4A":{"user_friendly":"0QAjflEZ_6KgKMxPlcnKN1ZoUvHdTT6hVwTW95EGVQfeSvD5"},"0:3D340B150EAC0392F5F428ABFFDF3C33BED68468A7F7F607C4A13D8013EF56B1":{"user_friendly":"kQA9NAsVDqwDkvX0KKv_3zwzvtaEaKf39gfEoT2AE-9Wse1G"}}}`,
				// get jetton transfers (to resolve mint token addr)
				`{"jetton_transfers":[{"query_id":"1721068295","source":"0:237E5119FFA2A028CC4F95C9CA37566852F1DD4D3EA15704D6F791065507DE4A","destination":"0:A1A2DCA202D48FAACD016CF0FCE75332B3DFCB74C54CECDF9AB6C158BD431CE3","amount":"22000000","source_wallet":"0:3D340B150EAC0392F5F428ABFFDF3C33BED68468A7F7F607C4A13D8013EF56B1","jetton_master":"0:226E80C4BFFA91ADC11DAD87706D52CD397047C128456ED2866D0549D8E2B163","transaction_hash":"A6ekLavrmSQOHKrmXCX396NJx0RPP4dtZlHVcrXxhlc=","transaction_lt":"23694319000003","transaction_now":1721068303,"response_destination":"0:237E5119FFA2A028CC4F95C9CA37566852F1DD4D3EA15704D6F791065507DE4A","custom_payload":"te6cckEBAQEACQAADgAAAABoaWne1AAn","forward_ton_amount":null,"forward_payload":null}]}`,
			},
			tx: &xcclient.TxInfo{
				// should report/use the msg-hash
				Name:   "chains/TON/transactions/ce2c0eeb0dd69d5a5d108869fa29f9943055e42c127b67256aeb6eddb688024f",
				Hash:   "ce2c0eeb0dd69d5a5d108869fa29f9943055e42c127b67256aeb6eddb688024f",
				XChain: xc.TON,
				State:  xcclient.Succeeded,
				Final:  true,
				Block: &xcclient.Block{
					Chain:  "TON",
					Height: xc.NewAmountBlockchainFromUint64(21082496),
					Hash:   "0:2000000000000000:22626042",
					Time:   time.Unix(1721068303, 0),
				},
				Movements: []*xcclient.Movement{
					// input TON movement
					{
						XAsset:    "chains/TON/assets/TON",
						XContract: "TON",
						AssetId:   "TON",
						From: []*xcclient.BalanceChange{
							{

								Balance:   xc.NewAmountBlockchainFromUint64(200000000),
								XAddress:  xcclient.NewAddressName(xc.TON, "0QAjflEZ_6KgKMxPlcnKN1ZoUvHdTT6hVwTW95EGVQfeSvD5"),
								AddressId: "0QAjflEZ_6KgKMxPlcnKN1ZoUvHdTT6hVwTW95EGVQfeSvD5",
							},
						},
						To: []*xcclient.BalanceChange{
							{
								Balance:   xc.NewAmountBlockchainFromUint64(200000000),
								XAddress:  xcclient.NewAddressName(xc.TON, "kQA9NAsVDqwDkvX0KKv_3zwzvtaEaKf39gfEoT2AE-9Wse1G"),
								AddressId: "kQA9NAsVDqwDkvX0KKv_3zwzvtaEaKf39gfEoT2AE-9Wse1G",
							},
						},
					},
					// token movement
					{
						XAsset:    "chains/TON/assets/EQAiboDEv_qRrcEdrYdwbVLNOXBHwShFbtKGbQVJ2OKxY0to",
						XContract: "EQAiboDEv_qRrcEdrYdwbVLNOXBHwShFbtKGbQVJ2OKxY0to",
						AssetId:   "EQAiboDEv_qRrcEdrYdwbVLNOXBHwShFbtKGbQVJ2OKxY0to",
						From: []*xcclient.BalanceChange{
							{

								Balance:   xc.NewAmountBlockchainFromUint64(22000000),
								XAddress:  xcclient.NewAddressName(xc.TON, "0QAjflEZ_6KgKMxPlcnKN1ZoUvHdTT6hVwTW95EGVQfeSvD5"),
								AddressId: "0QAjflEZ_6KgKMxPlcnKN1ZoUvHdTT6hVwTW95EGVQfeSvD5",
							},
						},
						To: []*xcclient.BalanceChange{
							{
								Balance:   xc.NewAmountBlockchainFromUint64(22000000),
								XAddress:  xcclient.NewAddressName(xc.TON, "EQChotyiAtSPqs0BbPD851Mys9_LdMVM7N-atsFYvUMc4yQp"),
								AddressId: "EQChotyiAtSPqs0BbPD851Mys9_LdMVM7N-atsFYvUMc4yQp",
							},
						},
						Memo: "hii",
					},
					// fee
					{
						XAsset:    "chains/TON/assets/TON",
						XContract: "TON",
						AssetId:   "TON",
						To:        []*xcclient.BalanceChange{},
						From: []*xcclient.BalanceChange{
							{

								Balance:   xc.NewAmountBlockchainFromUint64(2432322),
								XAddress:  xcclient.NewAddressName(xc.TON, "0QAjflEZ_6KgKMxPlcnKN1ZoUvHdTT6hVwTW95EGVQfeSvD5"),
								AddressId: "0QAjflEZ_6KgKMxPlcnKN1ZoUvHdTT6hVwTW95EGVQfeSvD5",
							},
						},
					},
				},
				Confirmations: 168,
			},
		},
		{
			desc: "reports_error",
			resp: []string{
				// get account
				`{"error":"bad stuff"}`,
			},
			httpStatus: 400,
			err:        "bad stuff",
		},
	}
	for i, v := range vectors {
		t.Run(fmt.Sprintf("testcase_%d_%s", i, v.desc), func(t *testing.T) {
			httpStatus := 200
			if v.httpStatus > 0 {
				httpStatus = v.httpStatus
			}
			server, close := testtypes.MockHTTP(t, v.resp, httpStatus)
			defer close()
			chain.URL = server.URL
			chain.Limiter = rate.NewLimiter(rate.Inf, 1)

			client, err := ton.NewClient(chain)
			require.NoError(t, err)
			info, err := client.FetchTxInfo(context.Background(), xc.TxHash(v.hash))

			if v.err != "" {
				require.ErrorContains(t, err, v.err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, info)
				// fees are calculated, skip
				info.Fees = nil

				require.EqualValues(t, reserialize(v.tx), reserialize(&info))
			}
		})
	}
}

func TestNewBlockId(t *testing.T) {
	id := ton.NewBlockId(0, "a000000000000000", 10)
	require.Equal(t, "0:a000000000000000:10", id)

	id = ton.NewBlockId(0, "-2305843009213693952", 10)
	require.Equal(t, "0:2000000000000000:10", id)

	id = ton.NewBlockId(0, "2000000000000000", 10)
	require.Equal(t, "0:2000000000000000:10", id)
}
