#!/bin/bash
version=$1
git clone https://github.com/phillipsj/my-sonobuoy-plugins.git
wait
wget -q https://github.com/vmware-tanzu/sonobuoy/releases/download/v${version}/sonobuoy_${version}_linux_amd64.tar.gz
wait
tar -xvf sonobuoy_${version}_linux_amd64.tar.gz
chmod +x sonobuoy && mv sonobuoy /usr/local/bin/sonobuoy