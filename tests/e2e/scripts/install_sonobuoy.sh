#!/bin/bash
git clone https://github.com/phillipsj/my-sonobuoy-plugins.git
wget -q https://github.com/vmware-tanzu/sonobuoy/releases/download/v0.56.0/sonobuoy_0.56.0_linux_amd64.tar.gz
tar -xvf sonobuoy_0.56.0_linux_amd64.tar.gz
chmod +x sonobuoy && mv sonobuoy /usr/local/bin/sonobuoy