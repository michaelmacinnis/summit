# summit

## Quick Start

Build client, server and mux.
Put client and mux somewhere in your `$PATH` as summit-client and summit-mux.
Run the server (pass -t and the path to the terminal emulator if not using kitty).

Install summit-mux on any machine/container where you want to launch terminals.
Instead of launching `$SHELL` on remote machines or in containers, run,

    summit-mux $SHELL

Instead of launching `$SHELL` locally, run,

    summit-client $SHELL

On a remote machine or in a container, when you want a new terminal, type,

    summit-client -n

or,

	summit-client -n command
