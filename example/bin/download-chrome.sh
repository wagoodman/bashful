#!/usr/bin/env bash
set -eux

CHROME_FILES=(google-chrome-stable_61.0.3163.100-1_amd64.deb)
if [[ -d chrome ]]; then
    mkdir -p downloads/chrome 
    mv chrome/* downloads/chrome/
    rm -rf chrome
elif [[ ! -d downloads/chrome ]]; then
    mkdir -p downloads/chrome 
fi
pushd downloads/chrome > /dev/null
    for FILE in ${CHROME_FILES[@]}; do
        if [[ ! -e $FILE ]]; then
        echo "Downloading $FILE"
        curl --location "https://github.com/wagoodman/docker-ruby-node-oracle/raw/master/chrome/$FILE" -o $FILE
        else
        echo "Skipping download $FILE"
        fi
    done
popd > /dev/null