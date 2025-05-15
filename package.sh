#!/bin/bash
set -e

echo "Packaging Flowctl Processor SDK..."

# Create a tarball of the SDK
tar -czvf flowctl-sdk.tar.gz \
  pkg/ \
  examples/ \
  go.mod \
  README.md \
  Makefile \
  .github/ \
  LICENSE

echo "SDK packaged as flowctl-sdk.tar.gz"
echo ""
echo "To use this SDK:"
echo "1. Extract it to your project or upload to your GitHub repo"
echo "2. Import it in your Go code: import \"github.com/withObsrvr/flowctl-sdk/pkg/processor\""
echo "3. See the README.md for usage examples"