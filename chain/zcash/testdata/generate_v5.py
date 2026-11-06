"""Generate transparent fixtures using the independent Zcash reference code.

Usage: python3 generate_v5.py /path/to/zcash-test-vectors > transparent_v5.json
Upstream: https://github.com/zcash/zcash-test-vectors
Pinned revision: 78321beacb0e0477e33cd002b56585a107c2708c
No Crosschain implementation is imported. The upstream published ZIP 244
vectors include shielded bundles, so these fixtures use its unmodified hashing
and serialization functions with explicitly constructed transparent bundles.
"""

import json
import subprocess
import sys
from types import SimpleNamespace

REVISION = "78321beacb0e0477e33cd002b56585a107c2708c"
assert subprocess.check_output(
    ["git", "-C", sys.argv[1], "rev-parse", "HEAD"], text=True
).strip() == REVISION
sys.path.insert(0, sys.argv[1])

from zcash_test_vectors.transaction import TransactionV5, OutPoint, TxIn, TxOut, Script
from zcash_test_vectors.zip_0244 import txid_digest, signature_digest, SIGHASH_ALL


def fixture(name, branch, input_count, output_count, long_scripts=False):
    tx = TransactionV5.__new__(TransactionV5)
    tx.nVersionGroupId = 0x26A7270A
    tx.nConsensusBranchId = branch
    tx.nLockTime = 123456 if input_count > 1 else 0
    tx.nExpiryHeight = 4567890 if input_count > 1 else 0
    tx.vin, tx.vout = [], []
    tx.vSpendsSapling, tx.vOutputsSapling, tx.vActionsOrchard = [], [], []
    tx.valueBalanceSapling = tx.valueBalanceOrchard = 0
    inputs, outputs, signing_inputs = [], [], []
    for i in range(input_count):
        prevout = OutPoint.from_components(bytes((j + i + 1) % 256 for j in range(32)), i + 2)
        sig_script = bytes([0x51]) * (253 if long_scripts else i)
        pk_script = bytes.fromhex("76a914" + f"{i + 1:02x}" * 20 + "88ac")
        if long_scripts:
            pk_script = bytes([0x51]) * 257
        amount = 100_000_000 + i * 12345
        sequence = 0xFFFFFFFF - i
        tx.vin.append(TxIn.from_components(prevout, Script.from_bytes(sig_script), sequence))
        signing_inputs.append(SimpleNamespace(nIn=i, amount=amount, scriptPubKey=Script.from_bytes(pk_script)))
        inputs.append(dict(txid=prevout.txid.hex(), index=prevout.n, sequence=sequence,
                           amount=amount, script_pubkey=pk_script.hex(), script_sig=sig_script.hex()))
    for i in range(output_count):
        out = TxOut.__new__(TxOut)
        out.nValue = 1234567 + i
        script = bytes.fromhex("76a914" + f"{i + 7:02x}" * 20 + "88ac")
        if i % 2:
            script = bytes.fromhex("a914" + f"{i + 7:02x}" * 20 + "87")
        if long_scripts:
            script = bytes([0x51]) * 253
        out.scriptPubKey = Script.from_bytes(script)
        tx.vout.append(out)
        outputs.append(dict(amount=out.nValue, script_pubkey=script.hex()))
    return dict(name=name, branch_id=branch, lock_time=tx.nLockTime, expiry_height=tx.nExpiryHeight,
                inputs=inputs, outputs=outputs, serialized=bytes(tx).hex(),
                txid=txid_digest(tx)[::-1].hex(),
                sighashes=[signature_digest(tx, signing_inputs, SIGHASH_ALL, inp).hex()
                           for inp in signing_inputs])


print(json.dumps(dict(reference_revision=REVISION, vectors=[
    fixture("nu5", 0xC2D6D0B4, 1, 2),
    fixture("nu7", 0x77190AD9, 1, 2),
    fixture("nu7_multiple_inputs_outputs", 0x77190AD9, 3, 3),
    fixture("nu7_compact_size_scripts", 0x77190AD9, 2, 2, True),
    fixture("empty_transparent_components", 0x77190AD9, 0, 0),
    fixture("outputs_only_digest", 0x77190AD9, 0, 1),
]), indent=2))
