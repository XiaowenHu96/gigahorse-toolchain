import sys
import os
import subprocess
from time import gmtime, strftime, perf_counter
import csv
from typing import Tuple, List, Union, Mapping, Set
import argparse

'''
This file is used to generate tac from contracts bytecode using gigahorse.
'''

class bcolors:
    RED = '\033[31m'
    FAIL = '\033[91m'
    ENDC = '\033[0m'

time="/usr/bin/time -f \"{},%e\" "
contract_dir="/home/xiaowen/big-contracts/contracts/"

class Task:
    def __init__(self, cmd):
        self.cmd = cmd

    def __decode(self, input):
        '''
        Dump stderr only
        '''
        return input.stderr.decode('utf-8');

    def run(self, silent=False, stdout="", stderr=""):

        print(self.cmd)
        ret = subprocess.run(self.cmd, capture_output=True, shell=True)
        
        if (stderr != ""):
            result = ret.stderr.decode('utf-8')
            f = open(stderr, "a")
            f.write(result)
            f.close()
        if (stdout != ""):
            result = ret.stdout.decode('utf-8')
            f = open(stdout, "a")
            f.write(result)
            f.close()
        if (not silent and ret.returncode != 0):
            print(f"{bcolors.FAIL}{self.__decode(ret)}{bcolors.ENDC}")
        return ret.returncode

def load_csv(path: str, seperator: str='\t') -> List[Union[str, List[str]]]:
    with open(path) as f:
        return [line.split(seperator)[0:2] for line in f.read().splitlines()]

def load_csv_map(path: str, seperator: str='\t', reverse: bool=False) -> Mapping[str, str]:
    return {y: x for x, y in load_csv(path, seperator)} if reverse else {x: y for x, y in load_csv(path, seperator)}

def gigahorse(inputfile):
    '''
        Runs Gigahorse toolchain (parser)

        inputfile contains log history of what contract has been run and their status

        by default it only runs contract that has not been run yet.
        with retry = True, it also retry failed contract before
    '''
    logs=load_csv_map(inputfile)
    for _, c in enumerate(os.listdir(contract_dir), start = 1):
        if c.endswith(".hex") and c not in logs:
            contract = os.path.join(contract_dir, c)
            
            ret = Task("./generatefacts " + contract +  " facts").run(silent=True)
            ret |= Task("bash -c \'echo \\\"8\\\" > facts/MaxContextDepth.csv\'").run(silent=True)
            start = perf_counter()
            ret |= Task("LD_LIBRARY_PATH=/home/xiaowen/gigahorse-toolchain/souffle-addon \
                    ./prog  -F facts -D out").run()
            analysis_time = perf_counter() - start
            start = perf_counter()
            ret |= Task("cd out && ../clients/gen_bytecode.py").run()
            generation_time = perf_counter() - start
            print("time spend on analysis {}, generation {}"\
                    .format(str(analysis_time), str(generation_time)))
            # Update log
            f = open(inputfile, "a")
            if (ret == 0) :
                f.write(c + "\t" + "PASSED" + "\t" + str(analysis_time) \
                        + "\t" + str(generation_time) + "\n")
                # copy program into GVM_CFG
                Task("cp ./out/contract.tac.in ./GVM_CFG/" + c.replace("hex", "tac")).run()
            else:
                f.write(c + "\t" + "FAILED" + "\t" + str(analysis_time) \
                        + "\t" + str(generation_time) + "\n")
            f.close()


parser = argparse.ArgumentParser()
parser.add_argument("-i",
                    "--input",
                    nargs="?",
                    metavar="FILE",
                    required=True,
                    help="the log file.")
args = parser.parse_args()
gigahorse(args.input)
