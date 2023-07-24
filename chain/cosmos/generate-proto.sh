rm -rf github.com
buf generate
cp -r github.com/* types/
rm -rf github.com
