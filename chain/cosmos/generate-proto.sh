rm -rf github.com
buf generate
cp -r github.com/cordialsys/crosschain/chain/cosmos/types .
rm -r github.com/cordialsys/crosschain/chain/cosmos/types
cp -r github.com/* types/
rm -rf github.com
