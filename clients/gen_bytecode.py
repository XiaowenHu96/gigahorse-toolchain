#!/usr/bin/env python3
from typing import Mapping, Set, TextIO, List
from collections import defaultdict
import networkx as nx
from networkx.algorithms.dominance import immediate_dominators
import matplotlib.pyplot as plt

import os
import sys

# IT: Ugly hack; this can be avoided if we pull the script at the top level
sys.path.append(os.path.join(os.path.dirname(__file__), '..'))
from clientlib.facts_to_cfg import Statement, Block, Function, construct_cfg, load_csv_map # type: ignore

def render_var(var: str):
    if var in tac_variable_value:
        return f"v{var.replace('0x', '')}({tac_variable_value[var]})"
    else:
        return f"v{var.replace('0x', '')}"

def emit(s: str, out: TextIO, indent: int=0):
    # 4 spaces
    INDENT_BASE = '    '

    print(f'{indent*INDENT_BASE}{s}', file=out)


def emit_stmt(stmt: Statement, out: TextIO):
    defs = [render_var(v) for v in stmt.defs]
    uses = [render_var(v) for v in stmt.operands]

    if defs:
        emit(f"{stmt.ident}: {', '.join(defs)} = {stmt.op} {', '.join(uses)}", out, 1)
    else:
        emit(f"{stmt.ident}: {stmt.op} {', '.join(uses)}", out, 1)


def pretty_print_block(block: Block, visited: Set[str], out: TextIO):
    emit(f"Begin block {block.ident}", out, 1)

    prev = [p.ident for p in block.predecessors]
    succ = [s.ident for s in block.successors]

    emit(f"prev=[{', '.join(prev)}], succ=[{', '.join(succ)}]", out, 1)
    # emit(f"=================================", out, 1)

    for stmt in block.statements:
        emit_stmt(stmt, out)

    emit('', out)

    for block in block.successors:
        if block.ident not in visited:
            visited.add(block.ident)
            pretty_print_block(block, visited, out)


def pretty_print_tac(functions: Mapping[str, Function], out: TextIO):
    for function in sorted(functions.values(), key=lambda x: x.ident):
        visibility = 'public' if function.is_public else 'private'
        emit(f"function {function.name}({', '.join(function.formals)}) {visibility} {{", out)
        pretty_print_block(function.head_block, set(), out)

        emit("}", out)
        emit("", out)

def find_phi_mapping(function : Function, preds: List[Block],\
        vars : Set[str], dom_tree : Mapping[Block, Block]) -> Mapping[str, str]:

    '''
    Produce a phi mapping by looking at their immediate dominators
    Return a mapping from variable_name -> block_name
    Throw error if SSA is ill-formed

    Def must be found in either the immediate predecessors or 
    one of the predecessors' immediate_dominators
    '''
    def var_in_block(vars: Set[str], block: Block):
        '''
        vars: The set of variables def we are looking for
        block: Current block we are searching
        '''
        defs = {render_var(v) for stmt in block.statements for v in stmt.defs}
        # edge case where variable comes from the arguments
        # Add those arguments into defs
        if not block.predecessors:
            defs |= {render_var(v) for v in function.formals}
        ret =  vars & defs
        if (len(ret) > 1):
            raise RuntimeError("More than one defs")
        if ret:
            return ret.pop()
        return None

    mapping = defaultdict()
    for pred in preds:
        cur = pred
        while True:
            v =  var_in_block(vars, cur)
            if v:
                if v in mapping:
                    raise RuntimeError("Double encoding: {}".format(v))
                mapping[v] = pred.ident
                break
            if not cur in dom_tree: # reach root
                # each pred should lead to one encoding
                raise RuntimeError("Cannot find encoding {}".format(vars))
            cur = dom_tree[cur]
    # San-check all definitions found
    for v in vars:
        if v not in mapping:
            raise RuntimeError("Cannot find encoding {}".format(v))
    return mapping

def analysis_phi_in_block(function : Function, block: Block,\
        dom_tree : Mapping[Block, Block]):

    for i in range(len(block.statements)):
        stmt = block.statements[i]
        # only care about def statement
        if stmt.defs and stmt.op == "PHI":
            # Collect variable names
            uses = {render_var(v) for v in stmt.operands}
            try:
                phi = find_phi_mapping(function, block.predecessors, uses, dom_tree)
                # Rewrite statement
                block.statements[i] = Statement(
                        stmt.ident,
                        "PHI {{{}}}".format(', '.join([phi[k] + " -> " + k for k in phi])),
                        stmt.operands,
                        stmt.defs
                    ) 
            except RuntimeError as err:
                print(err, "in function {}".format(function.name, block.ident), file=sys.stderr)
                emit_stmt(stmt, sys.stdout)
                exit(-1)

def analysis_phi(functions: Mapping[str, Function]):
    for function in functions.values():
        all_blocks = []
        to_visit = set({function.head_block})
        visited = set({})
        # collect all blocks
        while to_visit:
            node = to_visit.pop()
            all_blocks.append(node)
            visited.add(node)
            for succ in node.successors:
                if succ not in visited:
                    to_visit.add(succ)
        # constructing immediate dominators tree
        graph = [(a, b) for a in all_blocks for b in a.successors]
        # if function.ident == "getTransactionCount(bool,bool)":
        #     G = [(node[0].ident, node[1].ident) for node in graph]
        #     nx.draw(nx.DiGraph(G), with_labels=True, node_size=500)
        #     plt.savefig("g.png", format="PNG", dpi=240)
        #     plt.show()

        if not graph: # skip single-block function
            continue
        dom_tree = nx.immediate_dominators(nx.DiGraph(graph), function.head_block)
        del dom_tree[function.head_block]
        for block in all_blocks:
            analysis_phi_in_block(function, block, dom_tree)

def main():
    global tac_variable_value
    tac_variable_value = load_csv_map('TAC_Variable_Value.csv')

    _, functions,  = construct_cfg()
    analysis_phi(functions)

    with open('contract.tac.in', 'w') as f:
        pretty_print_tac(functions, f)
    


if __name__ == "__main__":
    main()
