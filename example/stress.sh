#!/usr/bin/env bash
# set -u
for i in $(seq 1 $1) ;do
    #  sleep .$RANDOM       #  ... process something here
    #echo "The start ($1)..." .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM
    echo "The start at $1 $2... $i: $(date)" .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM .$RANDOM  | tee -a compare.log    #echo "Something $i Another (from $1)"
done
