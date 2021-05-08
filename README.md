buffered-command runner can be used as a wrapper when running other commands.

In case of the command succeeding, output will be suppressed.
However, if the command is taking longer than 5 seconds to execute, all output from the time the command launched will be shown.

In case of the command failing, output can be either displayed or suppressed, depending on the value of the first flag.

Example usage:
`buffered-command-runner --[no]-allow-silent-failure docker build .`
