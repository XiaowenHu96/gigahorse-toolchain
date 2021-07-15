package gvm

import (
	"fmt"
	"github.com/holiman/uint256"
)

// Transformer encode GProgram into an executable format GVMProgram
// Each GVMProgram contains several GVMFunctions
// Each GVMFunctions contains a linear bytecode

type Uint256 = uint256.Int

type ConstSym struct {
	idx int
	val Uint256
}

type GVMFunction struct {
	// execution info
	insts      []int
	const_syms []ConstSym // const values (idx, val)

	// debuging & auxiliary info
	variable_encoding map[string]int      // map variable name into an idx
	blocks_start      map[string]int      // map block address into block start idx
	blocks_id         map[string]int      // map block address into an id
	jumps             map[int]*GStatement // bookkeeping all jump insturctions
	num_vars          int
	num_blocks        int
	name              string
}

type GVMProgram struct {
	entry     *GVMFunction
	functions []GVMFunction
}

type Transformer struct {
	function_encoding map[string]int // map function into an idx
	num_funcs         int
}

func (this *GVMFunction) add_var(name string) int {
	if id, ok := this.variable_encoding[name]; ok {
		return id
	}
	this.variable_encoding[name] = this.num_vars
	this.num_vars++
	return this.num_vars - 1
}

func (this *GVMFunction) get_var(name string) int {
	if idx, ok := this.variable_encoding[name]; !ok {
		return -1
	} else {
		return idx
	}
}

func (this *GVMFunction) encode_block(in *GBlock) int {
	if id, ok := this.blocks_id[in.address]; ok {
		return id
	}
	this.blocks_id[in.address] = this.num_blocks
	this.num_blocks++
	return this.num_blocks - 1
}

func (this *GVMFunction) decode_block(in *GBlock) int {
	if id, ok := this.blocks_id[in.address]; !ok {
		panic("Cannot find decode block")
	} else {
		return id
	}
}

func (this *GVMFunction) encode_var(in *GVariable) int {
	var idx int
	if typ := in.typ; typ == CONSTANT {
		// add constant into constant symbol
		idx = this.get_var(in.name)
		if idx == -1 {
			panic(fmt.Sprintf("Constant defined more than once %s", in.name))
		}
		this.add_var(in.name)
		val, err := uint256.FromHex(in.val)
		if err != nil {
			panic(err)
		}
		this.const_syms = append(this.const_syms, ConstSym{idx, *val})
	} else {
		// for variable, simply encode it
		idx = this.add_var(in.name)
	}
	return idx
}

func (this *Transformer) encode_function(name string) {
	if _, ok := this.function_encoding[name]; ok {
		return
	}
	this.function_encoding[name] = this.num_funcs
	this.num_funcs++
}

func (this *Transformer) decode_function(name string) int {
	if idx, ok := this.function_encoding[name]; !ok {
		panic(fmt.Sprintf("Error in decode_function, cannot find function %s", name))
	} else {
		return idx
	}
}

func (this *Transformer) init_transform(in *GProgram) {
	// Find entry function, always placed at 0
	for _, fun := range in.functions {
		if fun.name == "__function_selector__" {
			this.encode_function("__function_selector__")
		}
	}

	// Encode others
	for _, fun := range in.functions {
		this.encode_function(fun.name)
	}

	// transform each function
	for _, fun := range in.functions {
		this.transform_function(&fun)
	}
}

// transform_function flatten the graph layout into a linear one.
func (this *Transformer) transform_function(in *GFunction) {
	var function GVMFunction
	visited := make(map[*GBlock]bool)
	to_visited := make([]*GBlock, 0)

	// decode the graph in a DFS fashion
	to_visited = append(to_visited, &in.blocks[0])
	for len(to_visited) != 0 {
		cur := to_visited[len(to_visited)-1]
		visited[cur] = true
		function.add_block(cur)
		// append branched location first (So it is visited later)
		last_statment := cur.statements[len(cur.statements)-1]
		var branch_target *GBlock
		if op := last_statment.operation; op == JUMP || op == JUMPI {
			branch_target = in.get_block(last_statment.args[0].val)
		}
		if _, ok := visited[branch_target]; !ok {
			to_visited = append(to_visited, branch_target)
		}
		// append fallthough location second
		for _, succ := range cur.successor {
			if _, ok := visited[in.get_block(succ)]; branch_target == nil || (succ != branch_target.address && !ok) {
				to_visited = append(to_visited, branch_target)
			}
		}
	}

    // TODO: Fillup jump dest
}

func (this *GVMFunction) add_block(in *GBlock) {
	// log block start
	this.blocks_start[in.address] = len(this.insts) - 1
	this.encode_block(in)

	// find all PHI
	var phis []*GStatement

	for _, stmt := range in.statements {
		if stmt.operation == PHI {
			phis = append(phis, &stmt)
		}
	}

	// insert phis at the start of the block
	this.insts = append(this.insts, GVM_PHI_START)
	for _, phi := range phis {
		trans_phi(this, phi)
	}
	this.insts = append(this.insts, GVM_PHI_END)

	// translate all other statements
	for _, stmt := range in.statements {
		if stmt.operation != PHI {
            transformer_dispatcher(this, &stmt)
		}
	}

	// mark block end to update flag
	this.insts = append(this.insts, GVM_BLOCK_END)
	this.insts = append(this.insts, this.decode_block(in))
}

type TransFunc func(in *GVMFunction, stmt *GStatement)

func trans_phi(in *GVMFunction, stmt *GStatement) {
    // TODO
}

func trans_const(in *GVMFunction, stmt *GStatement) {
	// simply encode the constant, we won't have constant opcode during execution.
	in.encode_var(&stmt.args[0])
}

func trans_throw(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_THROW)
}

func trans_stop(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_STOP)
}

func trans_address(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_ADDRESS)
	in.push_arguments(1, stmt.args)
}

func trans_origin(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_ORIGIN)
	in.push_arguments(1, stmt.args)
}

func trans_caller(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_CALLER)
	in.push_arguments(1, stmt.args)
}

func trans_CALLVALUE(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_CALLVALUE)
	in.push_arguments(1, stmt.args)
}

func trans_CALLDATASIZE(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_CALLDATASIZE)
	in.push_arguments(1, stmt.args)
}

func trans_CODESIZE(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_CODESIZE)
	in.push_arguments(1, stmt.args)
}

func trans_GASPRICE(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_GASPRICE)
	in.push_arguments(1, stmt.args)
}

func trans_RETURNDATASIZE(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_RETURNDATASIZE)
	in.push_arguments(1, stmt.args)
}

func trans_COINBASE(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_COINBASE)
	in.push_arguments(1, stmt.args)
}

func trans_TIMESTAMP(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_TIMESTAMP)
	in.push_arguments(1, stmt.args)
}

func trans_NUMBER(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_NUMBER)
	in.push_arguments(1, stmt.args)
}

func trans_DIFFICULTY(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_DIFFICULTY)
	in.push_arguments(1, stmt.args)
}

func trans_GASLIMIT(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_GASLIMIT)
	in.push_arguments(1, stmt.args)
}

func trans_MSIZE(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_MSIZE)
	in.push_arguments(1, stmt.args)
}

func trans_GAS(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_GAS)
	in.push_arguments(1, stmt.args)
}

func trans_ISZERO(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_ISZERO)
	in.push_arguments(2, stmt.args)
}

func trans_BALANCE(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_BALANCE)
	in.push_arguments(2, stmt.args)
}

func trans_CALLDATALOAD(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_CALLDATALOAD)
	in.push_arguments(2, stmt.args)
}

func trans_EXTCODESIZE(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_EXTCODESIZE)
	in.push_arguments(2, stmt.args)
}

func trans_EXTCODEHASH(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_EXTCODEHASH)
	in.push_arguments(2, stmt.args)
}

func trans_BLOCKHASH(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_BLOCKHASH)
	in.push_arguments(2, stmt.args)
}

func trans_MLOAD(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_MLOAD)
	in.push_arguments(2, stmt.args)
}

func trans_SLOAD(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_SLOAD)
	in.push_arguments(2, stmt.args)
}

func trans_JUMP(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_JUMP)
	in.jumps[len(in.insts)-1] = stmt
	in.push_arguments(1, stmt.args)
}

func trans_SELFDESTRUCT(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_SELFDESTRUCT)
	in.push_arguments(1, stmt.args)
}

func trans_NOT(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_NOT)
	in.push_arguments(2, stmt.args)
}

func trans_ADD(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_ADD)
	in.push_arguments(3, stmt.args)
}

func trans_MUL(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_MUL)
	in.push_arguments(3, stmt.args)
}

func trans_SUB(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_SUB)
	in.push_arguments(3, stmt.args)
}

func trans_DIV(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_DIV)
	in.push_arguments(3, stmt.args)
}

func trans_SDIV(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_SDIV)
	in.push_arguments(3, stmt.args)
}

func trans_MOD(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_MOD)
	in.push_arguments(3, stmt.args)
}

func trans_SMOD(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_SMOD)
	in.push_arguments(3, stmt.args)
}

func trans_ADDMOD(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_ADDMOD)
	in.push_arguments(4, stmt.args)
}

func trans_EXP(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_EXP)
	in.push_arguments(3, stmt.args)
}

func trans_SIGNEXTEND(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_SIGNEXTEND)
	in.push_arguments(3, stmt.args)
}

func trans_LT(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_LT)
	in.push_arguments(3, stmt.args)
}

func trans_GT(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_GT)
	in.push_arguments(3, stmt.args)
}

func trans_SLT(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_SLT)
	in.push_arguments(3, stmt.args)
}

func trans_SGT(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_SGT)
	in.push_arguments(3, stmt.args)
}

func trans_EQ(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_EQ)
	in.push_arguments(3, stmt.args)
}

func trans_AND(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_ADD)
	in.push_arguments(3, stmt.args)
}

func trans_OR(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_OR)
	in.push_arguments(3, stmt.args)
}

func trans_XOR(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_XOR)
	in.push_arguments(3, stmt.args)
}

func trans_BYTE(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_BYTE)
	in.push_arguments(3, stmt.args)
}

func trans_SHL(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_SHL)
	in.push_arguments(3, stmt.args)
}

func trans_SHR(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_SHR)
	in.push_arguments(3, stmt.args)
}

func trans_SAR(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_SAR)
	in.push_arguments(3, stmt.args)
}

func trans_SHA3(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_SHA3)
	in.push_arguments(3, stmt.args)
}

func trans_MSTORE(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_MSTORE)
	in.push_arguments(3, stmt.args)
}

func trans_MSTORE8(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_MSTORE8)
	in.push_arguments(3, stmt.args)
}

func trans_SSTORE(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_SSTORE)
	in.push_arguments(3, stmt.args)
}

func trans_JUMPI(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_JUMPI)
	in.jumps[len(in.insts)-1] = stmt
	in.push_arguments(2, stmt.args)
}

func trans_REVERT(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_REVERT)
	in.push_arguments(2, stmt.args)
}

func trans_RETURN(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_RETURN)
	in.push_arguments(2, stmt.args)
}

func trans_LOG0(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_LOG0)
	in.push_arguments(2, stmt.args)
}

func trans_CALLDATACOPY(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_CALLDATACOPY)
	in.push_arguments(3, stmt.args)
}

func trans_CODECOPY(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_CODECOPY)
	in.push_arguments(3, stmt.args)
}

func trans_RETURNDATACOPY(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_RETURNDATACOPY)
	in.push_arguments(3, stmt.args)
}

func trans_LOG1(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_LOG1)
	in.push_arguments(3, stmt.args)
}

func trans_CREATE(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_CREATE)
	in.push_arguments(4, stmt.args)
}

func trans_EXTCODECOPY(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_EXTCODECOPY)
	in.push_arguments(4, stmt.args)
}

func trans_LOG2(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_LOG2)
	in.push_arguments(4, stmt.args)
}

func trans_LOG3(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_LOG3)
	in.push_arguments(5, stmt.args)
}

func trans_LOG4(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_LOG4)
	in.push_arguments(6, stmt.args)
}

func trans_CALL(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_CALL)
	panic("CALL: Don't know what to do")
}

func trans_CALLCODE(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_CALLCODE)
	panic("CALLCODE: Don't know what to do")
}

func trans_DELEGATECALL(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_DELEGATECALL)
	panic("DELEGATECALL: Don't know what to do")
}

func trans_CREATE2(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_CREATE2)
	in.push_arguments(5, stmt.args)
}

func trans_STATICCALL(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_STATICCALL)
	panic("STATICCALL: Don't know what to do")
}

func trans_CALLPRIVATE(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_CALLPRIVATE)
	in.push_arguments(len(stmt.args), stmt.args)
}

func trans_RETURNPRIVATE(in *GVMFunction, stmt *GStatement) {
	in.insts = append(in.insts, GVM_RETURNPRIVATE)
	in.push_arguments(len(stmt.args), stmt.args)
}

func (this *GVMFunction) push_arguments(num_args int, vars []GVariable) {
	i := 0
	for ; i < num_args; i++ {
		this.insts = append(this.insts, this.encode_var(&vars[i]))
	}
	// san-check
	if i != len(vars) {
		panic("Unmatch number of arguments")
	}
}

// TODO: this is a disaster....
// Golang does not allow struct as constant, 
// otherwise this can be much cleaner with a jump table defined in lexer
// Later we can improve it with a jumptable with a map
func transformer_dispatcher(in *GVMFunction, stmt *GStatement) {
	switch stmt.operation {
	case PHI:
		panic("PHI should be skipped during transformation")
	case CONST:
		trans_const(in, stmt)
	case THROW:
		trans_throw(in, stmt)
	case STOP:
		trans_stop(in, stmt)
	case ADDRESS:
		trans_address(in, stmt)
	case ORIGIN:
		trans_origin(in, stmt)
	case CALLER:
		trans_caller(in, stmt)
	case CALLVALUE:
		trans_CALLVALUE(in, stmt)
	case CALLDATASIZE:
		trans_CALLDATASIZE(in, stmt)
	case CODESIZE:
		trans_CODESIZE(in, stmt)
	case GASPRICE:
		trans_GASPRICE(in, stmt)
	case RETURNDATASIZE:
		trans_RETURNDATASIZE(in, stmt)
	case COINBASE:
		trans_COINBASE(in, stmt)
	case TIMESTAMP:
		trans_TIMESTAMP(in, stmt)
	case NUMBER:
		trans_NUMBER(in, stmt)
	case DIFFICULTY:
		trans_DIFFICULTY(in, stmt)
	case GASLIMIT:
		trans_GASLIMIT(in, stmt)
	case MSIZE:
		trans_MSIZE(in, stmt)
	case GAS:
		trans_GAS(in, stmt)
	case ISZERO:
		trans_ISZERO(in, stmt)
	case BALANCE:
		trans_BALANCE(in, stmt)
	case CALLDATALOAD:
		trans_CALLDATALOAD(in, stmt)
	case EXTCODESIZE:
		trans_EXTCODESIZE(in, stmt)
	case EXTCODEHASH:
		trans_EXTCODEHASH(in, stmt)
	case BLOCKHASH:
		trans_BLOCKHASH(in, stmt)
	case MLOAD:
		trans_MLOAD(in, stmt)
	case SLOAD:
		trans_SLOAD(in, stmt)
	case JUMP:
		trans_JUMP(in, stmt)
	case SELFDESTRUCT:
		trans_SELFDESTRUCT(in, stmt)
	case NOT:
		trans_NOT(in, stmt)
	case ADD:
		trans_ADD(in, stmt)
	case MUL:
		trans_MUL(in, stmt)
	case SUB:
		trans_SUB(in, stmt)
	case DIV:
		trans_DIV(in, stmt)
	case SDIV:
		trans_SDIV(in, stmt)
	case MOD:
		trans_MOD(in, stmt)
	case SMOD:
		trans_SMOD(in, stmt)
	case ADDMOD:
		trans_ADDMOD(in, stmt)
	case EXP:
		trans_EXP(in, stmt)
	case SIGNEXTEND:
		trans_SIGNEXTEND(in, stmt)
	case LT:
		trans_LT(in, stmt)
	case GT:
		trans_GT(in, stmt)
	case SLT:
		trans_SLT(in, stmt)
	case SGT:
		trans_SGT(in, stmt)
	case EQ:
		trans_EQ(in, stmt)
	case AND:
		trans_AND(in, stmt)
	case OR:
		trans_OR(in, stmt)
	case XOR:
		trans_XOR(in, stmt)
	case BYTE:
		trans_BYTE(in, stmt)
	case SHL:
		trans_SHL(in, stmt)
	case SHR:
		trans_SHR(in, stmt)
	case SAR:
		trans_SAR(in, stmt)
	case SHA3:
		trans_SHA3(in, stmt)
	case MSTORE:
		trans_MSTORE(in, stmt)
	case MSTORE8:
		trans_MSTORE8(in, stmt)
	case SSTORE:
		trans_SSTORE(in, stmt)
	case JUMPI:
		trans_JUMPI(in, stmt)
	case REVERT:
		trans_REVERT(in, stmt)
	case RETURN:
		trans_RETURN(in, stmt)
	case LOG0:
		trans_LOG0(in, stmt)
	case CALLDATACOPY:
		trans_CALLDATACOPY(in, stmt)
	case CODECOPY:
		trans_CODECOPY(in, stmt)
	case RETURNDATACOPY:
		trans_RETURNDATACOPY(in, stmt)
	case LOG1:
		trans_LOG1(in, stmt)
	case CREATE:
		trans_CREATE(in, stmt)
	case EXTCODECOPY:
		trans_EXTCODECOPY(in, stmt)
	case LOG2:
		trans_LOG2(in, stmt)
	case LOG3:
		trans_LOG3(in, stmt)
	case LOG4:
		trans_LOG4(in, stmt)
	case CALL:
		trans_CALL(in, stmt)
	case CALLCODE:
		trans_CALLCODE(in, stmt)
	case DELEGATECALL:
		trans_DELEGATECALL(in, stmt)
	case CREATE2:
		trans_CREATE2(in, stmt)
	case STATICCALL:
		trans_STATICCALL(in, stmt)
	case CALLPRIVATE:
		trans_CALLPRIVATE(in, stmt)
	case RETURNPRIVATE:
		trans_RETURNPRIVATE(in, stmt)
	default:
		panic(fmt.Sprintf("Unkown operation %d", stmt.operation))
	}
}

const (
	/** nullary */
	GVM_THROW = iota
	GVM_STOP
	GVM_ADDRESS
	GVM_ORIGIN
	GVM_CALLER
	GVM_CALLVALUE
	GVM_CALLDATASIZE
	GVM_CODESIZE
	GVM_GASPRICE
	GVM_RETURNDATASIZE
	GVM_COINBASE
	GVM_TIMESTAMP
	GVM_NUMBER
	GVM_DIFFICULTY
	GVM_GASLIMIT
	GVM_MSIZE
	GVM_GAS
	// CHAINID
	// SELFBALANCE

	/** unary */
	GVM_ISZERO
	GVM_BALANCE
	GVM_CALLDATALOAD
	GVM_EXTCODESIZE
	GVM_EXTCODEHASH
	GVM_BLOCKHASH
	GVM_MLOAD
	GVM_SLOAD
	GVM_JUMP
	GVM_SELFDESTRUCT
	GVM_NOT

	/** Binary */
	GVM_ADD
	GVM_MUL
	GVM_SUB
	GVM_DIV
	GVM_SDIV
	GVM_MOD
	GVM_SMOD
	GVM_ADDMOD
	GVM_EXP
	GVM_SIGNEXTEND
	GVM_LT
	GVM_GT
	GVM_SLT
	GVM_SGT
	GVM_EQ
	GVM_AND
	GVM_OR
	GVM_XOR
	GVM_BYTE
	GVM_SHL
	GVM_SHR
	GVM_SAR
	GVM_SHA3
	GVM_MSTORE
	GVM_MSTORE8
	GVM_SSTORE
	GVM_JUMPI
	GVM_REVERT
	GVM_RETURN
	GVM_LOG0

	/** Ternary operator */
	GVM_CALLDATACOPY
	GVM_CODECOPY
	GVM_RETURNDATACOPY
	GVM_LOG1
	GVM_CREATE

	/** n-ary operator*/
	GVM_PHI_START
	GVM_PHI
	GVM_PHI_END
	GVM_BLOCK_END
	GVM_EXTCODECOPY
	GVM_LOG2
	GVM_LOG3
	GVM_LOG4
	GVM_CALL
	GVM_CALLCODE
	GVM_DELEGATECALL
	GVM_CREATE2
	GVM_STATICCALL
	GVM_CALLPRIVATE
	GVM_RETURNPRIVATE
)
