# Parallel

Run multiple variations of a command in parallel.

## Features

- captures STDOUT and STDERR of each job separately
- skips jobs which have already been run (unless instructed otherwise)
- allows job arguments to be described in a variety of formats
- each job may modify multiple parts of the original command
- optionally define timeouts and perform graceful job cancellation

## Usage

```
Application Options:
      --debug

preparation:
      --csv            interpret STDIN as a CSV
      --json-line      interpret STDIN as JSON objects, one per line
      --shuffle        run the jobs in a random order

execution:
      --concurrency=   number of jobs to run in parallel (default: 10)
      --debounce=      also re-run successful jobs unless within the debounce period
      --graceful-exit  wait for current jobs to finish before exiting due to an interrupt
      --input=         send the input string (plus newline) forever as STDIN to each job
      --timeout=       maximum time a job may run for before being cancelled

```

## Examples

#### Basic operation

Run three variations of `echo`, substituting `{{.value}}` with each input line in turn

```bash
$ echo -e 'one\ntwo\nthree' | parallel -- echo {{.value}}
Nov 29 20:11:43.802 INF Success command="[echo three]" "combined output"="three\n"
Nov 29 20:11:43.802 INF Success command="[echo two]" "combined output"="two\n"
Nov 29 20:11:43.802 INF Success command="[echo one]" "combined output"="one\n"
Nov 29 20:11:43.802 INF Submitted: 3; Skipped: 0; In progress: 0; Succeeded: 3; Failed: 0

```

#### JSON parsing

Parse each input line as a JSON object

```bash
$ echo -e '{"animal": "cat", "name": "Scarface Claw"}\n{"animal": "dog", "name": "Bitzer Maloney"}' | ./parallel --json-line -- echo the {{.animal}} is called {{.name}}Dec  1 15:00:44.150 INF Success command="[echo the cat is called Scarface Claw]" "combined output"="the cat is called Scarface Claw\n"
Dec  1 15:00:44.150 INF Success command="[echo the dog is called Bitzer Maloney]" "combined output"="the dog is called Bitzer Maloney\n"
Dec  1 15:00:44.151 INF Submitted: 2; Skipped: 0; In progress: 0; Succeeded: 2; Failed: 0; Total: 2; Estimated time remaining: ?

```

#### CSV parsing

```bash
$ echo -e 'animal,name\ncat,Scarface Claw\ndog,Bitzer Maloney' | ./parallel --csv -- echo the {{.animal}} is called {{.name}}
Dec  1 14:59:40.820 INF Success command="[echo the cat is called Scarface Claw]" "combined output"="the cat is called Scarface Claw\n"
Dec  1 14:59:40.820 INF Success command="[echo the dog is called Bitzer Maloney]" "combined output"="the dog is called Bitzer Maloney\n"
Dec  1 14:59:40.821 INF Submitted: 2; Skipped: 0; In progress: 0; Succeeded: 2; Failed: 0; Total: 2; Estimated time remaining: ?
```

#### shuffle 

In some situations, you may wish to run the jobs in a random order. If this is desired, add `--shuffle`

#### Status logging

Every 10 seconds an interim status is generated, as well as at completion

```bash
$ seq 1 10 | ./parallel --concurrency 4 -- bash -c 'echo {{.value}} ; sleep 4'
Dec  1 15:05:48.282 INF Success command="[bash -c echo 1 ; sleep 4]" "combined output"="1\n"
Dec  1 15:05:48.282 INF Success command="[bash -c echo 3 ; sleep 4]" "combined output"="3\n"
Dec  1 15:05:48.282 INF Success command="[bash -c echo 2 ; sleep 4]" "combined output"="2\n"
Dec  1 15:05:48.282 INF Success command="[bash -c echo 4 ; sleep 4]" "combined output"="4\n"
Dec  1 15:05:52.286 INF Success command="[bash -c echo 7 ; sleep 4]" "combined output"="7\n"
Dec  1 15:05:52.286 INF Success command="[bash -c echo 6 ; sleep 4]" "combined output"="6\n"
Dec  1 15:05:52.286 INF Success command="[bash -c echo 8 ; sleep 4]" "combined output"="8\n"
Dec  1 15:05:52.286 INF Success command="[bash -c echo 5 ; sleep 4]" "combined output"="5\n"
Dec  1 15:05:54.279 INF Submitted: 10; Skipped: 0; In progress: 2; Succeeded: 8; Failed: 0; Total: 10; Estimated time remaining: 0s
Dec  1 15:05:56.291 INF Success command="[bash -c echo 10 ; sleep 4]" "combined output"="10\n"
Dec  1 15:05:56.291 INF Success command="[bash -c echo 9 ; sleep 4]" "combined output"="9\n"
Dec  1 15:05:56.291 INF Submitted: 10; Skipped: 0; In progress: 0; Succeeded: 10; Failed: 0; Total: 10; Estimated time remaining: 0s
```

### Job cancellations and timeouts

Defining a timeout will also cause jobs to be terminated when it is reached:

```bash
$ seq 1 7 | ./parallel --concurrency 2 --timeout 5s -- bash -c 'echo {{.value}} ; sleep {{.value}}'
Dec  1 15:10:03.858 INF Success command="[bash -c echo 1 ; sleep 1]" "combined output"="1\n"
Dec  1 15:10:04.858 INF Success command="[bash -c echo 2 ; sleep 2]" "combined output"="2\n"
Dec  1 15:10:06.860 INF Success command="[bash -c echo 3 ; sleep 3]" "combined output"="3\n"
Dec  1 15:10:08.862 INF Success command="[bash -c echo 4 ; sleep 4]" "combined output"="4\n"
Dec  1 15:10:11.863 WRN job was aborted due to context cancellation command="[bash -c echo 5 ; sleep 5]"
Dec  1 15:10:11.863 INF Failure command="[bash -c echo 5 ; sleep 5]" "combined output"="5\n" error="signal: killed"
Dec  1 15:10:12.853 INF Submitted: 7; Skipped: 0; In progress: 2; Succeeded: 4; Failed: 1; Total: 7; Estimated time remaining: 3s
Dec  1 15:10:13.864 WRN job was aborted due to context cancellation command="[bash -c echo 6 ; sleep 6]"
Dec  1 15:10:13.864 INF Failure command="[bash -c echo 6 ; sleep 6]" "combined output"="6\n" error="signal: killed"
Dec  1 15:10:16.865 WRN job was aborted due to context cancellation command="[bash -c echo 7 ; sleep 7]"
Dec  1 15:10:16.865 INF Failure command="[bash -c echo 7 ; sleep 7]" "combined output"="7\n" error="signal: killed"
Dec  1 15:10:16.866 INF Submitted: 7; Skipped: 0; In progress: 0; Succeeded: 4; Failed: 3; Total: 7; Estimated time remaining: 0s
```

Cancelling (e.g. with CTRL-C) while running will by default stop all running jobs immediately.

```bash
$ seq 1 10 | ./parallel --concurrency 2 -- bash -c 'echo {{.value}} ; sleep {{.value}}'
Dec  1 15:10:52.188 INF Success command="[bash -c echo 1 ; sleep 1]" "combined output"="1\n"
Dec  1 15:10:53.188 INF Success command="[bash -c echo 2 ; sleep 2]" "combined output"="2\n"
^CDec  1 15:10:54.509 WRN job was aborted due to context cancellation command="[bash -c echo 4 ; sleep 4]"
Dec  1 15:10:54.509 INF Failure command="[bash -c echo 4 ; sleep 4]" "combined output"="4\n" error="signal: killed"
Dec  1 15:10:54.509 WRN job was aborted due to context cancellation command="[bash -c echo 3 ; sleep 3]"
Dec  1 15:10:54.510 INF Failure command="[bash -c echo 3 ; sleep 3]" "combined output"="3\n" error="signal: killed"
Dec  1 15:10:54.510 INF Submitted: 4; Skipped: 0; In progress: 0; Succeeded: 2; Failed: 2; Total: 10; Estimated time remaining: 5s
Dec  1 15:10:54.510 ERR context canceled
```

If the currently-running jobs should be allowed to finish before exiting in response to CTRL-C, use `--graceful-exit`:

```bash
$ seq 1 10 | ./parallel --concurrency 2 --graceful-exit -- bash -c 'echo {{.value}} ; sleep {{.value}}'
Dec  1 15:11:33.626 INF Success command="[bash -c echo 1 ; sleep 1]" "combined output"="1\n"
Dec  1 15:11:34.626 INF Success command="[bash -c echo 2 ; sleep 2]" "combined output"="2\n"
^CDec  1 15:11:36.631 INF Success command="[bash -c echo 3 ; sleep 3]" "combined output"="3\n"
Dec  1 15:11:38.631 INF Success command="[bash -c echo 4 ; sleep 4]" "combined output"="4\n"
Dec  1 15:11:38.631 INF Submitted: 4; Skipped: 0; In progress: 0; Succeeded: 4; Failed: 0; Total: 10; Estimated time remaining: 9s
Dec  1 15:11:38.631 ERR context canceled
```

If each job reads from STDIN, this can be supplied with `--input` (similar to the `yes` command):
```bash
$ echo -e 'animal,name\ncat,Scarface Claw' | ./parallel --input happy --csv -- /bin/bash -c 'read emotion; echo the {{.animal}} is called {{.name}} and is $emotion'
Dec  1 15:18:34.578 INF Success command="[/bin/bash -c read emotion; echo the cat is called Scarface Claw and is $emotion]" "combined output"="the cat is called Scarface Claw and is happy\n"
Dec  1 15:18:34.578 INF Submitted: 1; Skipped: 0; In progress: 0; Succeeded: 1; Failed: 0; Total: 1; Estimated time remaining: ?

```

## Notes

- if no command is provided, a placeholder command is used which simply echoes the inputs. This is mostly
  intended for experimentation purposes.
- each time a variation completes successfully (ie: with a zero exit code), a file is created in ~/.cache/parallel/success which
  contains the STDOUT/STDERR. Similarly, failed output is stored in ~/.cache/parallel/failure. The MTIME of this file is used by the debouncer to determine whether it is appropriate to rerun the variation. These files will never be cleaned up by `parallel`. If desired, something like this can be run to remove cache files older than a week: `find ~/.cache/parallel/ -type f -mtime +1 -delete`