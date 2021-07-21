#!/bin/bash

set -e
set -x
./generatefacts $1 facts
bash -c "echo \"8\" > facts/MaxContextDepth.csv"
LD_LIBRARY_PATH=/home/xiaowen/gigahorse-toolchain/souffle-addon ./prog  -F facts -D out
cd out && ../clients/gen_bytecode.py
