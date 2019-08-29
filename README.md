# pokemem

`pokemem` is a simple program to find the address of a variable in a process and change its value.
One use case is to increase money or another resource in a game.

`pokemem` uses the ptrace system call to inspect the memory of a process and replace a value when a match is found.

## Usage

The strategy used by `pokemem` is to find all memory addresses holding a particular value (currently 32 bits) and keep checking those addresses as the target value changes.
There may be many initial matches, but in most cases the actual address can be found in one or two iterations, and then the value can be replaced.

**Note:** `pokemem` must be run as root or with `CAP_SYS_PTRACE` set on the binary.

Here is a simple example where there's only one instance of the desired value in the process memory.
In other cases, you may have to change the value in the target process (through some input or action the program allows) to narrow the list of matches.

```bash
# Run with the the pid of the target process as the only argument
$ ./pokemem 1234
value to find: 42  <-- the initial value of the variable in the target process
successfully attached to 1234
found a single match!  <-- we found just one instance of the value, so we have the right address
replacement value: 5  <-- new value to set in target process
replacing with value: 5
detaching from 1234
detached from 1234
```
