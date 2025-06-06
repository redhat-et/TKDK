"""Debugging utilities for Triton kernels.

Includes conditional breakpoints and printing based on thread identifiers,
plus tensor readiness checks for CUDA or interpreted environments.
"""
# pylint: disable=multiple-statements,line-too-long,import-outside-toplevel,eval-used,fixme,unused-variable
import os
import triton
import triton.language as tl

def _test_pid_conds(conds, pid0=0, pid1=0, pid2=0):
    '''Test if condition on pids are fulfilled
    E.g.:
        '=0'    checks that pid_0 == 0
        ',>1'   checks that pid_1 > 1
        '>1,=0' checks that pid_0 > 1 and pid_1 == 0
    '''
    pids = pid0, pid1, pid2
    conds = conds.replace(' ','').split(',')
    for i, (cond, pid) in enumerate(zip(conds, pids)):
        if cond=='': continue
        if   cond[:2] in ['<=', '>=', '!=']: op, threshold = cond[:2], int(cond[2:])
        elif cond[:1] in ['<',  '>',  '=' ]: op, threshold = cond[:1], int(cond[1:])
        else: raise ValueError(f"Rules may only use these ops: '<','>','>=','<=','=', '!='. Invalid rule in '{cond}'.")
        op = '==' if op == '=' else op
        if not eval(f'{pid} {op} {threshold}'): return False
    return True

@triton.jit
def test_pid_conds(conds):
    '''Test if condition on pids are fulfilled
    E.g.:
        '=0'    checks that pid_0 == 0
        ',>1'   checks that pid_1 > 1
        '>1,=0' checks that pid_0 > 1 and pid_1 == 0
    '''
    return _test_pid_conds(conds, tl.program_id(0).handle.data[0], tl.program_id(1).handle.data[0], tl.program_id(2).handle.data[0])

@triton.jit
def breakpoint_if(conds):
    '''Stop kernel, if condition on pids is fulfilled'''
    from IPython.core.debugger import set_trace
    if test_pid_conds(conds): set_trace()

@triton.jit
def print_if(*txt, conds):
    '''Print txt, if condition on pids is fulfilled'''
    if test_pid_conds(conds): print(*txt)

@triton.jit
def breakpoint_once():
    """Trigger a breakpoint."""
    breakpoint_if('=0,=0,=0')

@triton.jit
def print_once(*txt):
    """Print a message."""
    print_if(*txt,conds='=0,=0,=0')

def assert_tensors_gpu_ready(*tensors):
    """Assert that each tensor is contiguous and on the GPU (unless TRITON_INTERPRET=1)."""
    for t in tensors:
        assert t.is_contiguous(), "A tensor is not contiguous"
        if not os.environ.get('TRITON_INTERPRET') == '1': assert t.is_cuda, "A tensor is not on cuda"

@triton.jit
def offsets_from_base(ptrs, base_ptr):
    '''Return offsets for which ptrs = base_ptr + offsets''' # todo umer: write test
    return ptrs.to(tl.uint64) - base_ptr.to(tl.uint64)
