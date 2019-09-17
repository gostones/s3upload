#!/usr/bin/env bash

SIZES=(
500M
1G
2G
3G
5G
10G
20G
30G
50G
)

for i in ${SIZES[@]}; do
    echo "/tmp/${i}.raw"
    time mkfile $i /tmp/${i}.raw
    echo
done