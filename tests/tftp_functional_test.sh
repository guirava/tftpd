#!/bin/bash
HOST="127.0.0.1"
REST="http://$HOST:8069"

function failed() {
  echo -e "\t\t\t\t*** FAILED ***"
}

function run_expect () {
  expect -f ./putputget.expect $HOST $1 $2
  diff  $1 $1.copy || failed
  rm $1*
}

# empty file list at server:
curl $REST/clear

# a small file:
echo -n "abcdefghijklmnopqrstuvwxyz" > abc.testfile
run_expect abc.testfile 26

# empty file:
touch empty.testfile
run_expect empty.testfile 0

# a block size minus 1 byte:
head -c 511 < /dev/urandom > 511.testfile
run_expect 511.testfile 511

# exactly a block size:
head -c 512 < /dev/urandom > 512.testfile
run_expect 512.testfile 512

# a bigger file:
head -c 20k < /dev/urandom > 20k.testfile
run_expect 20k.testfile 20480

# 40MB will create a rollover in the TFTP packet block numbers
head -c 40m < /dev/urandom > 40m.testfile
run_expect 40m.testfile 41943040

curl $REST/clear

