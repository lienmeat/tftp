#!/usr/bin/env bash

#script is for quickly testing how concurrent requests scale using a known-good tftp client implementation on macos/linux
#should be used as a sanity check or system check

IP=$1
PORT=$2
N=500

rm -rf /tmp/dl
mkdir /tmp/dl
rm -rf /tmp/up
mkdir /tmp/up

for (( i = 0; i < $N; ++i )); do
    cp test.txt "/tmp/up/$i.txt"
done

for (( i = 0; i < $N; ++i )); do
	./tftp.sh $IP $PORT "$i.txt" &
done

wait
