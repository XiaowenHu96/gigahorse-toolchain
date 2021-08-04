#!/bin/bash
generatefacts=/home/xihu5895/gigahorse-toolchain/generatefacts
souffleaddon=/home/xihu5895/gigahorse-toolchain/souffle-addon
gen_bytecode=/home/xihu5895/gigahorse-toolchain/clients/gen_bytecode.py
souffle=/home/xihu5895/gigahorse-toolchain/decompiler_compiled

input=$1
facts=$2
out=$3

set -e
set -x
$generatefacts $input $facts
bash -c "echo \"8\" > $facts/MaxContextDepth.csv"
mkdir -p $out
if [[ $# -eq 4 ]];
then
    LD_LIBRARY_PATH=$souffleaddon timeout $4 $souffle  -F $facts -D $out
else
    LD_LIBRARY_PATH=$souffleaddon $souffle  -F $facts -D $out
fi
cd $out && $gen_bytecode
