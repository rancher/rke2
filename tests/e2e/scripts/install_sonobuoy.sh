#!/bin/bash
set -e

SONOBUOY_VERSION="0.56.0"
SONOBUOY_CHECKSUM="01e35fbe2c402a31a766b3f46fe3280136f67d39e153c0b517d035b61608a28d"

SONOBUOY_PLUGINS_COMMIT="9359729a5a5948250aae78cb8b4b0eda87c0caaf"

git clone https://github.com/phillipsj/my-sonobuoy-plugins.git
cd my-sonobuoy-plugins && git checkout "${SONOBUOY_PLUGINS_COMMIT}" && cd ..

curl -fsSL "https://github.com/vmware-tanzu/sonobuoy/releases/download/v${SONOBUOY_VERSION}/sonobuoy_${SONOBUOY_VERSION}_linux_amd64.tar.gz" -o sonobuoy.tar.gz
echo "${SONOBUOY_CHECKSUM}  sonobuoy.tar.gz" | sha256sum -c
tar -xvf sonobuoy.tar.gz sonobuoy
chmod +x sonobuoy && mv sonobuoy /usr/local/bin/sonobuoy
rm -f sonobuoy.tar.gz
