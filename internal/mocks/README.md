# Mocks

The `mocks` package contains mock objects used for testing.
They have been generated with `mockery`.

There is a `scripts/regenerate_mocks.sh` script which can be executed to regenerate **most** mocks when required (interface change).

However, there is an exception:
Interfaces in another repository (e.g. `avalanchego`) seem to pose problems to `mockery`. We get 

`outside main module or its selected dependencies`

trying to generate them via script.

Those should probably need to be regenerated and pasted here manually when required.

At this point this affects:
* InfoClient
* PClient
