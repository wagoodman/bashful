#!/usr/bin/env bash
# set -u

for((i=0;i<=$1;++i)) do
    printf \\$(printf '%o' $[47+45*(RANDOM%2)]);
done