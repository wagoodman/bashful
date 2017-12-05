#!/usr/bin/env bash
# set -u

count=$1
for i in $(seq 1 $count) ;do
    sleep .$RANDOM
    echo "worker ($i/$1): $(cat /dev/urandom | tr -dc 'a-zA-Z0-9' | fold -w $((RANDOM%80)) | head -n 1)"
done
