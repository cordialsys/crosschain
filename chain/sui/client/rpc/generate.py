#!/usr/bin/env python3
"""Generate Sui RPC bindings from a checkout of the pinned sui-apis revision.

Usage: PATH=<protoc plugins>:$PATH python3 generate.py /path/to/sui-apis
Requires protoc, protoc-gen-go v1.36.11 and protoc-gen-go-grpc v1.6.1.
"""
import pathlib
import re
import subprocess
import sys

REVISION = 'aa9ba2c13b2a9348059b38110e64af39849a0377'
source = pathlib.Path(sys.argv[1]).resolve()
revision = subprocess.check_output(['git', '-C', str(source), 'rev-parse', 'HEAD'], text=True).strip()
if revision != REVISION:
    raise SystemExit(f'Expected sui-apis revision {REVISION}, got {revision}')
root = source / 'proto'
out = pathlib.Path(__file__).resolve().parent
services = ['sui/rpc/v2/' + name + '.proto' for name in
            ['state_service', 'ledger_service', 'transaction_execution_service']]
files = set()
def visit(name):
    if not name.startswith('sui/') or name in files:
        return
    files.add(name)
    for dependency in re.findall(r'import "([^"]+)";', (root / name).read_text()):
        visit(dependency)
for name in services:
    visit(name)
package = 'github.com/cordialsys/crosschain/chain/sui/client/rpc;rpc'
options = []
for name in sorted(files):
    options.extend(['--go_opt=M' + name + '=' + package,
                    '--go-grpc_opt=M' + name + '=' + package])
subprocess.run(['protoc', '-I' + str(root), '--go_out=' + str(out),
                '--go_opt=module=github.com/cordialsys/crosschain/chain/sui/client/rpc',
                '--go-grpc_out=' + str(out),
                '--go-grpc_opt=module=github.com/cordialsys/crosschain/chain/sui/client/rpc',
                *options, *sorted(files)], check=True)
