#!/usr/bin/env bash

IP=$1
PORT=$2
FILE=$3

cd /tmp/up/

tftp $IP $PORT << EOF
  mode octet
  verbose
  trace
  put $FILE
  quit
EOF

cd /tmp/dl/

tftp $IP $PORT << EOF
  mode octet
  verbose
  trace
  get $FILE
  quit
EOF
