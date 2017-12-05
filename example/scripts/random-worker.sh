#!/usr/bin/env bash

count=$1
for i in $(seq 1 $count) ;do
    sleep .$RANDOM
    echo "$2 worker ($i/$1): $(cat /dev/urandom | tr -dc 'a-zA-Z0-9' | fold -w $((RANDOM%80 + 1)) | head -n 1)"
done
