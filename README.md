# Parallel

Run multiple variations of a command in parallel.

While GNU parallel is optimised to run as part of a pipeline of commands, this tool
is optimised for interactive use - for example, when operating on a large number of cloud objects.

## Features

- captures STDOUT and STDERR of each job separately, for both successful and unsuccessful jobs
- allows job arguments to be described in a variety of formats (one per line, JSON per line, or CSV)
- sensible console status messages providing a progress summary
- repeated CTRL-Cs are used to progressively increase jobs' termination priority

Optionally:

- skips and/or deprioritises jobs which have already been run (unless instructed otherwise, based on time since last successful execution)
- define timeouts
- abort on job failure
- inject STDIN to each job

## Installation

```bash
go install github.com/nicois/parallel/parallel@latest
```

The binary will be installed into `~/go/bin/`

## Usage

```
Application Options:
      --debug

preparation:
      --csv             interpret STDIN as a CSV
      --debounce=       re-run jobs outside the debounce period, even if they would normally be skipped
      --defer-reruns    give priority to jobs which have not previously been run
      --json-line       interpret STDIN as JSON objects, one per line
      --skip-failures   skip jobs which have already been run unsuccessfully
      --skip-successes  skip jobs which have already been run successfully

execution:
      --abort-on-error  stop running (as though CTRL-C were pressed) if a job fails
      --concurrency=    run this many jobs in parallel (default: 10)
      --dry-run         simulate what would be run
      --hide-failures   do not display a message each time a job fails
      --hide-successes  do not display a message each time a job succeeds
      --input=          send the input string (plus newline) forever as STDIN to each job
      --timeout=        cancel each job after this much time
```

## Examples

#### Basic operation

Run three variations of `echo`, substituting `{{.value}}` with each input line in turn

```bash
$ echo -e 'one\ntwo\nthree' \
    | parallel -- echo {{.value}}
Dec  5 11:51:29.914 INF Success command="{command:[echo three] input:}" "combined output"="three\n"
Dec  5 11:51:29.915 INF Success command="{command:[echo two] input:}" "combined output"="two\n"
Dec  5 11:51:29.915 INF Success command="{command:[echo one] input:}" "combined output"="one\n"
Dec  5 11:51:29.915 INF Queued: 0; Skipped: 0; In progress: 0; Succeeded: 3; Failed: 0; Aborted: 0; Total: 3; Estimated time remaining: 0s
```

In fact, if `parallel` is run without supplying a command, it does the same thing:

```bash
$ echo -e 'one\ntwo\nthree' | parallel
Dec  5 11:51:05.255 INF no command was provided, so just echoing the input commandline="[echo value is {{.value}}]"
Dec  5 11:51:05.257 INF Success command="{command:[echo value is one] input:}" "combined output"="value is one\n"
Dec  5 11:51:05.258 INF Success command="{command:[echo value is three] input:}" "combined output"="value is three\n"
Dec  5 11:51:05.258 INF Success command="{command:[echo value is two] input:}" "combined output"="value is two\n"
Dec  5 11:51:05.258 INF Queued: 0; Skipped: 0; In progress: 0; Succeeded: 3; Failed: 0; Aborted: 0; Total: 3; Estimated time remaining: 0s
```

#### JSON parsing

Parse each input line as a JSON object

```bash
$ echo -e '{"animal": "cat", "name": "Scarface Claw"}\n{"animal": "dog", "name": "Bitzer Maloney"}' \
    | parallel --json-line -- echo the {{.animal}} is called {{.name}}
Dec  4 12:51:10.763 INF Success command="{command:[echo the cat is called Scarface Claw] input:}" "combined output"="the cat is called Scarface Claw\n"
Dec  4 12:51:10.763 INF Success command="{command:[echo the dog is called Bitzer Maloney] input:}" "combined output"="the dog is called Bitzer Maloney\n"
Dec  4 12:51:10.763 INF Queued: 0; Skipped: 0; In progress: 0; Succeeded: 2; Failed: 0; Total: 2; Elapsed time: 0s

```

#### CSV parsing

```bash
$ echo -e 'animal,name\ncat,Scarface Claw\ndog,Bitzer Maloney' \
    | parallel --csv -- echo the {{.animal}} is called {{.name}}
Dec  4 12:51:40.897 INF Success command="{command:[echo the cat is called Scarface Claw] input:}" "combined output"="the cat is called Scarface Claw\n"
Dec  4 12:51:40.897 INF Success command="{command:[echo the dog is called Bitzer Maloney] input:}" "combined output"="the dog is called Bitzer Maloney\n"
Dec  4 12:51:40.897 INF Queued: 0; Skipped: 0; In progress: 0; Succeeded: 2; Failed: 0; Total: 2; Elapsed time: 0s
```

#### Status logging

Every 10 seconds an interim status is generated, as well as at completion. An estimate of the remaining time will be shown,
based solely on the rate of completion of earlier jobs.
Duplicate status messages, where nothing has changed, will be suppressed for up to a minute.

```bash
$ seq 1 10 \
    | parallel --concurrency 4 -- bash -c 'echo {{.value}} ; sleep 4'
Dec  4 12:52:20.534 INF Success command="{command:[bash -c echo 1 ; sleep 4] input:}" "combined output"="1\n"
Dec  4 12:52:20.533 INF Success command="{command:[bash -c echo 2 ; sleep 4] input:}" "combined output"="2\n"
Dec  4 12:52:20.534 INF Success command="{command:[bash -c echo 3 ; sleep 4] input:}" "combined output"="3\n"
Dec  4 12:52:20.534 INF Success command="{command:[bash -c echo 4 ; sleep 4] input:}" "combined output"="4\n"
Dec  4 12:52:24.539 INF Success command="{command:[bash -c echo 5 ; sleep 4] input:}" "combined output"="5\n"
Dec  4 12:52:24.539 INF Success command="{command:[bash -c echo 8 ; sleep 4] input:}" "combined output"="8\n"
Dec  4 12:52:24.539 INF Success command="{command:[bash -c echo 6 ; sleep 4] input:}" "combined output"="6\n"
Dec  4 12:52:24.539 INF Success command="{command:[bash -c echo 7 ; sleep 4] input:}" "combined output"="7\n"
Dec  4 12:52:26.529 INF Queued: 0; Skipped: 0; In progress: 2; Succeeded: 8; Failed: 0; Total: 10; Estimated time remaining: 0s
Dec  4 12:52:28.544 INF Success command="{command:[bash -c echo 9 ; sleep 4] input:}" "combined output"="9\n"
Dec  4 12:52:28.544 INF Success command="{command:[bash -c echo 10 ; sleep 4] input:}" "combined output"="10\n"
Dec  4 12:52:28.544 INF Queued: 0; Skipped: 0; In progress: 0; Succeeded: 10; Failed: 0; Total: 10; Estimated time remaining: 0s
```

#### Skipping previously-run jobs

If a job has already been attempted, and should not be re-attempted, use `--skip-successes` and/or `--skip-failures` as applicable:

```bash
$ seq 2 | parallel --skip-successes
Dec  5 11:26:59.787 INF no command was provided, so just echoing the input commandline="[echo value is {{.value}}]"
Dec  5 11:26:59.790 INF Success command="{command:[echo value is 1] input:}" "combined output"="value is 1\n"
Dec  5 11:26:59.790 INF Success command="{command:[echo value is 2] input:}" "combined output"="value is 2\n"
Dec  5 11:26:59.791 INF Queued: 0; Skipped: 0; In progress: 0; Succeeded: 2; Failed: 0; Aborted: 0; Total: 2; Elapsed time: 0s


$ seq 5 | parallel --skip-successes
Dec  5 11:27:02.607 INF no command was provided, so just echoing the input commandline="[echo value is {{.value}}]"
Dec  5 11:27:02.610 INF Success command="{command:[echo value is 4] input:}" "combined output"="value is 4\n"
Dec  5 11:27:02.610 INF Success command="{command:[echo value is 5] input:}" "combined output"="value is 5\n"
Dec  5 11:27:02.610 INF Success command="{command:[echo value is 3] input:}" "combined output"="value is 3\n"
Dec  5 11:27:02.610 INF Queued: 0; Skipped: 2; In progress: 0; Succeeded: 3; Failed: 0; Aborted: 0; Total: 3; Estimated time remaining: 0s

```

Notice the `skipped` value in the stats line is now nonzero.

#### Debounce period

If you only want to skip jobs which haven't succeeded/failed recently, you can provide a `--debounce` period.
Be aware that this period is assessed when the STDIN record is parsed, not when the job is about to start.

Below, 2 jobs are run, then 3 more 10 seconds later. With a debounce of 10s, this means the third execution skips the 3 recent jobs:

```bash
$ seq 2 | parallel --skip-successes ; sleep 10; seq 5 | parallel --skip-successes ; seq 5 | parallel --skip-successes --debounce 10s
Dec  5 11:30:35.053 INF no command was provided, so just echoing the input commandline="[echo value is {{.value}}]"
Dec  5 11:30:35.054 INF Success command="{command:[echo value is 1] input:}" "combined output"="value is 1\n"
Dec  5 11:30:35.054 INF Success command="{command:[echo value is 2] input:}" "combined output"="value is 2\n"
Dec  5 11:30:35.054 INF Queued: 0; Skipped: 0; In progress: 0; Succeeded: 2; Failed: 0; Aborted: 0; Total: 2; Elapsed time: 0s

Dec  5 11:30:45.061 INF no command was provided, so just echoing the input commandline="[echo value is {{.value}}]"
Dec  5 11:30:45.062 INF Success command="{command:[echo value is 5] input:}" "combined output"="value is 5\n"
Dec  5 11:30:45.063 INF Success command="{command:[echo value is 4] input:}" "combined output"="value is 4\n"
Dec  5 11:30:45.063 INF Success command="{command:[echo value is 3] input:}" "combined output"="value is 3\n"
Dec  5 11:30:45.063 INF Queued: 0; Skipped: 2; In progress: 0; Succeeded: 3; Failed: 0; Aborted: 0; Total: 3; Estimated time remaining: 0s

Dec  5 11:30:45.064 INF no command was provided, so just echoing the input commandline="[echo value is {{.value}}]"
Dec  5 11:30:45.065 INF Success command="{command:[echo value is 1] input:}" "combined output"="value is 1\n"
Dec  5 11:30:45.065 INF Success command="{command:[echo value is 2] input:}" "combined output"="value is 2\n"
Dec  5 11:30:45.065 INF Queued: 0; Skipped: 3; In progress: 0; Succeeded: 2; Failed: 0; Aborted: 0; Total: 2; Elapsed time: 0s

```

#### Deprioritising recently-run jobs

By default, jobs are started in the order they are provided via STDIN.
If desired `--defer-reruns` will notice if a job has been run previously (whether successful or not), and will run other jobs first.
Where multiple jobs are reruns, priority is given to least recently-run jobs.
Where jobs have never been run before, the order provided in STDIN is respected.

It is possible for some rerun jobs to be started sooner than they should, due to the way incoming jobs are immediately dispatched
to the worker processes. In the example below, jobs 1 and 2 "sneak in" early, but the remaining reruns are correctly deferred

```bash
$ seq 5 | parallel ; seq 10 | parallel --defer-reruns
Dec  5 11:44:52.732 INF no command was provided, so just echoing the input commandline="[echo value is {{.value}}]"
Dec  5 11:44:52.735 INF Success command="{command:[echo value is 3] input:}" "combined output"="value is 3\n"
Dec  5 11:44:52.736 INF Success command="{command:[echo value is 5] input:}" "combined output"="value is 5\n"
Dec  5 11:44:52.736 INF Success command="{command:[echo value is 2] input:}" "combined output"="value is 2\n"
Dec  5 11:44:52.736 INF Success command="{command:[echo value is 4] input:}" "combined output"="value is 4\n"
Dec  5 11:44:52.736 INF Success command="{command:[echo value is 1] input:}" "combined output"="value is 1\n"
Dec  5 11:44:52.736 INF Queued: 0; Skipped: 0; In progress: 0; Succeeded: 5; Failed: 0; Aborted: 0; Total: 5; Estimated time remaining: 0s

Dec  5 11:44:52.741 INF no command was provided, so just echoing the input commandline="[echo value is {{.value}}]"
Dec  5 11:44:52.744 INF Success command="{command:[echo value is 1] input:}" "combined output"="value is 1\n"
Dec  5 11:44:52.744 INF Success command="{command:[echo value is 9] input:}" "combined output"="value is 9\n"
Dec  5 11:44:52.744 INF Success command="{command:[echo value is 8] input:}" "combined output"="value is 8\n"
Dec  5 11:44:52.744 INF Success command="{command:[echo value is 2] input:}" "combined output"="value is 2\n"
Dec  5 11:44:52.745 INF Success command="{command:[echo value is 10] input:}" "combined output"="value is 10\n"
Dec  5 11:44:52.745 INF Success command="{command:[echo value is 7] input:}" "combined output"="value is 7\n"
Dec  5 11:44:52.745 INF Success command="{command:[echo value is 5] input:}" "combined output"="value is 5\n"
Dec  5 11:44:52.745 INF Success command="{command:[echo value is 4] input:}" "combined output"="value is 4\n"
Dec  5 11:44:52.746 INF Success command="{command:[echo value is 6] input:}" "combined output"="value is 6\n"
Dec  5 11:44:52.746 INF Success command="{command:[echo value is 3] input:}" "combined output"="value is 3\n"
Dec  5 11:44:52.746 INF Queued: 0; Skipped: 0; In progress: 0; Succeeded: 10; Failed: 0; Aborted: 0; Total: 10; Estimated time remaining: 0s

```

#### Suppressing success and/or failure messages

If you want a less noisy output, you can suppress success and/or failure messages. STDOUT and STDERR are still logged to
the filesystem as normal:

```bash
$ seq 1 254 | parallel --hide-failures --concurrency 100 --debounce 10s --timeout 10s -- nc -vz 192.168.4.{{.value}} 443
Dec  4 21:51:17.386 INF Success command="{command:[nc -vz 192.168.4.53 443] input:}" "combined output"="Ncat: Version 7.92 ( https://nmap.org/ncat )\nNcat: Connected to 192.168.4.53:443.\nNcat: 0 bytes sent, 0 bytes received in 0.06 seconds.\n"
Dec  4 21:51:20.001 INF Queued: 142; Skipped: 0; In progress: 100; Succeeded: 1; Failed: 11; Total: 254; Estimated time remaining: 11s
Dec  4 21:51:23.681 INF Success command="{command:[nc -vz 192.168.4.222 443] input:}" "combined output"="Ncat: Version 7.92 ( https://nmap.org/ncat )\nNcat: Connected to 192.168.4.222:443.\nNcat: 0 bytes sent, 0 bytes received in 0.14 seconds.\n"
Dec  4 21:51:26.735 INF Queued: 0; Skipped: 0; In progress: 0; Succeeded: 2; Failed: 252; Total: 254; Estimated time remaining: 0s
```

### Dry-run

Want to ensure the right command will be run with the correct inputs? `--dry-run` will do this. Nothing will actually be executed.
An implicit 1 second sleep will be substituted for the actual execution of each command:

```bash
$ seq 8 | parallel --dry-run --debounce 5s --concurrency 1 --input y -- rm -f foo.{{.value}}
Dec  4 20:44:56.622 INF Success command="{command:[rm -f foo.1] input:y}" "combined output"="(dry run)"
Dec  4 20:44:57.623 INF Success command="{command:[rm -f foo.2] input:y}" "combined output"="(dry run)"
Dec  4 20:44:58.624 INF Success command="{command:[rm -f foo.3] input:y}" "combined output"="(dry run)"
Dec  4 20:44:59.625 INF Success command="{command:[rm -f foo.4] input:y}" "combined output"="(dry run)"
Dec  4 20:45:00.000 INF Queued: 3; Skipped: 0; In progress: 1; Succeeded: 4; Failed: 0; Total: 8; Estimated time remaining: 4s
Dec  4 20:45:00.627 INF Success command="{command:[rm -f foo.5] input:y}" "combined output"="(dry run)"
Dec  4 20:45:01.627 INF Success command="{command:[rm -f foo.6] input:y}" "combined output"="(dry run)"
Dec  4 20:45:02.628 INF Success command="{command:[rm -f foo.7] input:y}" "combined output"="(dry run)"
Dec  4 20:45:03.628 INF Success command="{command:[rm -f foo.8] input:y}" "combined output"="(dry run)"
Dec  4 20:45:03.628 INF Queued: 0; Skipped: 0; In progress: 0; Succeeded: 8; Failed: 0; Total: 8; Estimated time remaining: 0s
```

### Job cancellations and timeouts

Defining a timeout will cause jobs to be terminated if it is reached:

```bash
$ seq 1 7 \
    | parallel --concurrency 2 --timeout 5s -- bash -c 'echo {{.value}} ; sleep {{.value}}'
Dec  4 12:53:08.995 INF Success command="{command:[bash -c echo 1 ; sleep 1] input:}" "combined output"="1\n"
Dec  4 12:53:09.995 INF Success command="{command:[bash -c echo 2 ; sleep 2] input:}" "combined output"="2\n"
Dec  4 12:53:12.000 INF Success command="{command:[bash -c echo 3 ; sleep 3] input:}" "combined output"="3\n"
Dec  4 12:53:14.000 INF Success command="{command:[bash -c echo 4 ; sleep 4] input:}" "combined output"="4\n"
Dec  4 12:53:17.004 WRN job was aborted due to context cancellation command="{command:[bash -c echo 5 ; sleep 5] input:}"
Dec  4 12:53:17.004 WRN Failure command="{command:[bash -c echo 5 ; sleep 5] input:}" "combined output"="5\n" error="signal: killed"
Dec  4 12:53:17.991 INF Queued: 0; Skipped: 0; In progress: 2; Succeeded: 4; Failed: 1; Total: 7; Estimated time remaining: 3s
Dec  4 12:53:19.002 WRN job was aborted due to context cancellation command="{command:[bash -c echo 6 ; sleep 6] input:}"
Dec  4 12:53:19.002 WRN Failure command="{command:[bash -c echo 6 ; sleep 6] input:}" "combined output"="6\n" error="signal: killed"
Dec  4 12:53:22.007 WRN job was aborted due to context cancellation command="{command:[bash -c echo 7 ; sleep 7] input:}"
Dec  4 12:53:22.007 WRN Failure command="{command:[bash -c echo 7 ; sleep 7] input:}" "combined output"="7\n" error="signal: killed"
Dec  4 12:53:22.008 INF Queued: 0; Skipped: 0; In progress: 0; Succeeded: 4; Failed: 3; Total: 7; Estimated time remaining: 0s
```

Cancelling (e.g. with CTRL-C) while running will stop any further jobs from being started, and will exit
when all currently-running jobs have completed.
Pressing CTRL-C a second time will send SIGTERM to all running jobs.
A third CTRL-C will send SIGKILL to all remaining running jobs.
A fourth and final CTRL-C will send SIGKILL to all remaining running jobs, as well as other processes in
their process groups.

```bash
$ seq 80 | parallel --concurrency 5 --defer-reruns  -- bash -c 'trap noop SIGTERM ; sleep {{.value}}'
^CDec  7 13:02:16.012 WRN received cancellation signal. Waiting for current jobs to finish before exiting. Hit CTRL-C again to exit sooner
Dec  7 13:02:16.012 INF Queued: 75; Skipped: 0; In progress: 5; Succeeded: 0; Failed: 0; Aborted: 0; Total: 80; Elapsed time: 2s
Dec  7 13:02:17.000 INF Queued: 0; Skipped: 0; In progress: 5; Succeeded: 0; Failed: 0; Aborted: 0; Total: 5; Elapsed time: 3s
^CDec  7 13:02:17.749 WRN second CTRL-C received. Sending SIGTERM to running jobs. Hit CTRL-C again to use SIGKILL instead
^CDec  7 13:02:20.234 WRN third CTRL-C received. Sending SIGKILL to running jobs. Hit CTRL-C again to kill all subprocesses too
^CDec  7 13:02:22.158 WRN fourth CTRL-C received. Sending SIGKILL to running jobs and their subprocesses
Dec  7 13:02:22.158 WRN Failure command="{command:[bash -c trap noop SIGTERM ; sleep 48] input:}" "combined output"="" error="signal: killed"
Dec  7 13:02:22.158 WRN Failure command="{command:[bash -c trap noop SIGTERM ; sleep 44] input:}" "combined output"="" error="signal: killed"
Dec  7 13:02:22.158 WRN Failure command="{command:[bash -c trap noop SIGTERM ; sleep 46] input:}" "combined output"="" error="signal: killed"
Dec  7 13:02:22.159 WRN Failure command="{command:[bash -c trap noop SIGTERM ; sleep 45] input:}" "combined output"="" error="signal: killed"
Dec  7 13:02:22.159 WRN Failure command="{command:[bash -c trap noop SIGTERM ; sleep 47] input:}" "combined output"="" error="signal: killed"
Dec  7 13:02:22.159 INF Queued: 0; Skipped: 0; In progress: 0; Succeeded: 0; Failed: 5; Aborted: 0; Total: 5; Elapsed time: 8s
Dec  7 13:02:22.159 ERR user-initiated shutdown
```

If you want to stop processing if a job fails, use `--abort-on-error`:

```bash
$ seq 1 10 \
    | parallel --abort-on-error --concurrency 2 --timeout 5s -- bash -c 'echo {{.value}} ; sleep {{.value}}'
Dec  4 12:54:51.606 INF Success command="{command:[bash -c echo 1 ; sleep 1] input:}" "combined output"="1\n"
Dec  4 12:54:52.606 INF Success command="{command:[bash -c echo 2 ; sleep 2] input:}" "combined output"="2\n"
Dec  4 12:54:54.610 INF Success command="{command:[bash -c echo 3 ; sleep 3] input:}" "combined output"="3\n"
Dec  4 12:54:56.611 INF Success command="{command:[bash -c echo 4 ; sleep 4] input:}" "combined output"="4\n"
Dec  4 12:54:59.614 WRN job was aborted due to context cancellation command="{command:[bash -c echo 5 ; sleep 5] input:}"
Dec  4 12:54:59.614 WRN Failure command="{command:[bash -c echo 5 ; sleep 5] input:}" "combined output"="5\n" error="signal: killed"
Dec  4 12:54:59.614 WRN job was aborted due to context cancellation command="{command:[bash -c echo 6 ; sleep 6] input:}"
Dec  4 12:54:59.614 WRN Failure command="{command:[bash -c echo 6 ; sleep 6] input:}" "combined output"="6\n" error="signal: killed"
Dec  4 12:54:59.615 INF Queued: 4; Skipped: 0; In progress: 0; Succeeded: 4; Failed: 2; Total: 10; Estimated time remaining: 6s
Dec  4 12:54:59.615 ERR nonzero exit code
```

### Simulating STDIN

If each job expects input from STDIN, this can be supplied with `--input` (similar to the `yes` command).
Note that the input text can be the same for each job, or can be parameterised using the same inputs as the command itself:

```bash
$ echo -e 'animal,name,emotion\ncat,Scarface Claw,hungry' \
    | parallel --input '{{.emotion}}' --csv -- /bin/bash -c 'read emotion; echo the {{.animal}} is called {{.name}} and is $emotion'
Dec  4 11:42:38.289 INF Success command="{command:[/bin/bash -c read emotion; echo the cat is called Scarface Claw and is $emotion] input:hungry}" "combined output"="the cat is called Scarface Claw and is hungry\n"
Dec  4 11:42:38.289 INF Submitted: 1; Skipped: 0; In progress: 0; Succeeded: 1; Failed: 0; Total: 1; Elapsed time: 0s
```

## Notes

- if no command is provided, a placeholder command is used which simply echoes the inputs. This is mostly
  intended for experimentation purposes.
- each time a variation completes successfully (ie: with a zero exit code), a file is created in ~/.cache/parallel/success which
  contains the STDOUT/STDERR. Similarly, failed output is stored in ~/.cache/parallel/failure. The MTIME of this file is used by the debouncer to determine whether it is appropriate to rerun the variation. These files will never be cleaned up by `parallel`. If desired, something like this can be run to remove cache files older than a week: `find ~/.cache/parallel/ -type f -mtime +1 -delete`
- jobs will be processed as soon as they are received, without waiting for STDIN to be closed.
