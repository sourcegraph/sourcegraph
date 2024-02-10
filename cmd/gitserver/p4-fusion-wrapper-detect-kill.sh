#!/usr/bin/env bash
# shellcheck disable=SC2064,SC2207

# TODO: What does this do?

# create a file to hold the output of p4-fusion
# TODO: consider recording/storing/capturing the file for logs display in the UI if there's a problem
fusionout=$(mktemp || mktemp -t fusionout_XXXXXXXX)

# create a pipe to use for capturing output of p4-fusion
# so that it can be sent to stdout and also to a file for analyzing later
fusionpipe=$(mktemp || mktemp -t fusionpipe_XXXXXXXX)
rm -f "${fusionpipe}"
mknod "${fusionpipe}" p
tee <"${fusionpipe}" "${fusionout}" &

# create a file to hold the output of `wait`
waitout=$(mktemp || mktemp -t waitout_XXXXXXXX)

# create a file to hold the resource usage of the child process
stats=$(mktemp || mktemp -t resource_XXXXXXXX)

# make sure to clean up on exit
trap "rm -f \"${fusionout}\" \"${fusionpipe}\" \"${waitout}\" \"${stats}\"" EXIT

# launch p4-fusion in the background, sending all output to the pipe for capture and re-echoing
# depends on the p4-fusion binary executable being copied to p4-fusion-binary in the gitserver Dockerfile
p4-fusion-binary "${@}" >"${fusionpipe}" 2>&1 &

# capture the pid of the child process
fpid=$!

# start up a "sidecar" process to capture resource usage.
# it will terminate when the p4-fusion process terminates.
process-stats-watcher.sh "${fpid}" "p4-fusion-binary" >"${stats}" &
spid=$!

# Wait for the child process to finish
wait ${fpid} >"${waitout}" 2>&1

# capture the result of the wait, which is the result of the child process
# or the result of external action on the child process, like SIGKILL
waitcode=$?

# the sidecar process should have exited by now, but just in case, wait for it
wait "${spid}" >/dev/null 2>&1

[ ${waitcode} -eq 0 ] || {
  # if the wait exit code indicates a problem,
  # check to see if the child process was killed
  killed=""
  # if the process was killed with SIGKILL, the `wait` process will have generated a notification
  grep -qs "Killed" "${waitout}" && killed=y
  [ -z "${killed}" ] && {
    # If the wait process did not generate an error message, check the process output.
    # The process traps SIGINT and SIGTERM; uncaught signals will be displayed as "uncaught"
    tail -5 "${fusionout}" | grep -Eqs "Signal Received:|uncaught target signal" && killed=y
  }
  [ -z "${killed}" ] || {
    # include the signal if it's SIGINT, SIGTERM, or SIGKILL
    # not gauranteed to work, but nice if we can include the info
    signal="$(kill -l ${waitcode})"
    [ -z "${signal}" ] || signal=" (SIG${signal})"
    # get info if available from the sidecar process
    rusage=""
    [ -s "${stats}" ] && {
      # expect the last (maybe only) line to be four fields:
      # RSS VSZ ETIME TIME
      x=($(tail -1 "${stats}"))
      # NOTE: bash indexes from 0; zsh indexes from 1
      [ ${#x[@]} -eq 4 ] && rusage=" At the time of its demise, it had been running for ${x[2]}, had used ${x[3]} CPU time, reserved ${x[1]} RAM and was using ${x[0]}."
    }
    echo "p4-fusion was killed by an external signal${signal}.${rusage}"
  }
}

exit ${waitcode}
