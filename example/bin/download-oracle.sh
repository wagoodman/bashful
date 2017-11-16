#!/usr/bin/env bash
set -eux

ORACLE_FILES=(instantclient-basiclite-linux.x64-12.1.0.2.0.zip instantclient-sqlplus-linux.x64-12.1.0.2.0.zip instantclient-sdk-linux.x64-12.1.0.2.0.zip)

if [[ -d oracle ]]; then
    mkdir -p downloads/oracle 
    mv oracle/* downloads/oracle/
    rm -rf oracle
elif [[ ! -d downloads/oracle ]]; then
    mkdir -p downloads/oracle 
fi
pushd downloads/oracle > /dev/null
    for FILE in ${ORACLE_FILES[@]}; do
        if [[ ! -e $FILE ]]; then
        echo "Downloading $FILE"
        curl --location "https://github.com/wagoodman/docker-ruby-node-oracle/raw/master/oracle/$FILE" -o $FILE
        else
        echo "Skipping download $FILE"
        fi
    done
popd > /dev/null