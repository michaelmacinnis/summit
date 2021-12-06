# summit

## Quick Start

- Build client, server and mux.
- Put client and mux (locally) somewhere on your `$PATH` as summit-client and summit-mux.
- Run the server (locally, passing -t and the path to the terminal emulator if not using kitty).
- Install summit-mux on any machine and in any container where you want to launch terminals.

Instead of launching `$SHELL` locally (from a terminal emulator, for example), launch,

    summit-client $SHELL

Instead of `$SHELL` being the "entry point" on remote machines or in containers, use,

    summit-mux $SHELL

On a remote machine or in a container, when you want a new terminal, type,

    summit-mux -n

or,

	summit-mux -n command
