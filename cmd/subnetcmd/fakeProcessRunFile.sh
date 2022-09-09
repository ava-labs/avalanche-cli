#!/bin/sh


# This file runs as part of TestFindByRunningProcess
# to simulate a running process.
# The test starts this file with expected cli args
# which then will be matched by the test.
# We just need to have something running for some
# (short) time - the test kills it right away anyway.
sleep 20
