## 2016.03.11

### Features

- Moves the working path to default to `.wercker` and removes the flags
  for configuring the other paths
- Adds a symlink `.wercker/latest` for referring to your latest build, and
  a `.wercker/latest_deploy` for referring to your latest deploy
- Make the --artifacts work better locally, making your build's artifacts
  easily available under .wercker/latest/output
- Automatically use the contents of `.wercker/latest/output` when running a
  `wercker deploy` without specifying a target
- When running `wercker deploy` if the specified target does not container a
  wercker.yml file, attempt to use the one in the current directory.
- Allow settings multiple tags at a time when doing `internal/docker-push`
- Check for and allow unix:///var/run/docker.sock on non-linux hosts


### Bug Fixes

- Deal with symlinks significantly better
- Respect --docker-local when using `internal/docker-push` (don't push)
- Allow images to be pulled by nested local services (removes
  implicit --docker-local)
- Workaround a docker issue related to not fully consuming the result of a
  CopyFromContainer API call (when we exported a cache that was more than our
  limit of 1GB we'd just drop it, and docker would hang)
- Remove pipeline ID tag set by `internal/docker-push`


## 2016.02.10

### Features

- Allow users to mount local volumes to their wercker build containers, specified by a list of `volumes` underneath box in the werker.yml file. Must have `--enable-volumes` flag set in order to run.
- Check to see if config from wercker.yml is empty
- Adds changelog

### Bug fixes

- Fixes to the shellstep implementation
