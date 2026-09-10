// Regenerate with ox@0.14.0 installed in the module search path:
// node fee_token_vector.mjs > fee_token_vector.json
import { AbiFunction, Secp256k1 } from 'ox'
import { TxEnvelopeTempo } from 'ox/tempo'

const privateKey = `0x${'0'.repeat(63)}1` // public test key only
const recipient = '0x1111111111111111111111111111111111111111'
const transferToken = '0x20c0000000000000000000000000000000000001'
const feeToken = '0x20c0000000000000000000000000000000000000'
const data = AbiFunction.encodeData(
  AbiFunction.from('function transfer(address to, uint256 amount)'),
  [recipient, 1000000n],
)
const envelope = TxEnvelopeTempo.from({
  chainId: 42431,
  nonce: 7n,
  gas: 1000000n,
  maxPriorityFeePerGas: 0n,
  maxFeePerGas: 20000000000n,
  calls: [{ to: transferToken, value: 0n, data }],
  feeToken,
})
const sighash = TxEnvelopeTempo.getSignPayload(envelope)
const signature = Secp256k1.sign({ payload: sighash, privateKey })
const signed = TxEnvelopeTempo.from(envelope, {
  signature: { type: 'secp256k1', signature },
})
const sender = '0x7E5F4552091A69125d5DfCb7b8C2659029395Bdf'
const payer = '0x2B5AD5c4795c026514f8317c7a215E218DcCD6cF'
const sponsored = TxEnvelopeTempo.from({ ...envelope, feePayerSignature: null })
// Tempo's sender preimage clears feeToken and uses the 0x00 sponsorship marker.
// Ox 0.14.0 requires this normalization explicitly before getSignPayload.
const senderEnvelope = TxEnvelopeTempo.from({ ...sponsored, feeToken: undefined })
const senderSighash = TxEnvelopeTempo.getSignPayload(senderEnvelope)
const senderSignature = Secp256k1.sign({ payload: senderSighash, privateKey })
const payerSighash = TxEnvelopeTempo.getFeePayerSignPayload(sponsored, { sender })
const payerSignature = Secp256k1.sign({
  payload: payerSighash, privateKey: `0x${'0'.repeat(63)}2`,
})
const sponsoredSigned = TxEnvelopeTempo.from({ ...sponsored, feePayerSignature: payerSignature }, {
  signature: { type: 'secp256k1', signature: senderSignature },
})
console.log(JSON.stringify({
  source: 'ox@0.14.0 TxEnvelopeTempo',
  unsigned: TxEnvelopeTempo.serialize(envelope),
  sighash,
  signed: TxEnvelopeTempo.serialize(signed),
  hash: TxEnvelopeTempo.hash(signed),
  sponsored: {
    sender, payer, senderSighash, payerSighash,
    unsigned: TxEnvelopeTempo.serialize(senderEnvelope),
    signed: TxEnvelopeTempo.serialize(sponsoredSigned),
    hash: TxEnvelopeTempo.hash(sponsoredSigned),
  },
}, null, 2))
