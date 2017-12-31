# bashful
Because your bash script should be quiet and shy-like (...and not such a loud mouth). 

**This is beta quality!** Use at your own risk.

![Image](demo.gif)

Use a yaml file to stitch together commands and bash snippits and run them with style!

*"But why would you make this monstrosity?"* you ask...
because I could. And because ` &>/dev/null` or ` | tee -a some.log` or `set -e; do something; set +e` is getting annoying.

**Features:**
- [x] Optionally run commands in parallel
- [x] Summary of the last line from stdout/stderr printed inline with commands
- [x] A shiny vertical progress bar
- [x] Optionally stop when a single command fails
- [x] Configuration yaml block to control the behavior/look & feel
- [x] Show detailed error reports when commands fail
- [x] Log all actions taken with all stdout/stderr
- [x] See an ETA for tasks that have already been run
- [ ] Interact with the mouse to see more/less tasks (https://godoc.org/github.com/nsf/termbox-go#Event)

## Installation & Usage
```
go get github.com/wagoodman/bashful
bashful <path-to-yaml-file>
```

The contents of the yaml file are detailed in the next sections, but here is a hello world for you:
```yaml
tasks:
    - cmd: echo "Hello, World!"
```
**There are a ton of examples in the `examples/` dir.** Go check them out!


## Options
Here is an exhaustive list of all of the config options (in the `config` yaml block). These options
are global options that apply to all tasks within the yaml file:
```yaml

# this block is used to configure the look, feel, and behavior of all tasks
config:
    # which character used to delimintate the task list
    bullet-char: "-"

    # hide all subtasks after section completion
    collapse-on-completion: false

    # by default the screen is updated when an event occurs (when stdout from
    # a running process is read). This can be changed to only allow the 
    # screen to be updated on an interval (to accomodate slower devices).
    event-driven: false

    # the number of tasks that can run simultaneously
    max-parallel-commands: 4

    # log all task output and events to the given logfile
    log-path: path/to/file.log

    # show/hide the detailed summary of all task failures after completion
    show-failure-report: true

    # show/hide the last summary line (showing % complete, number of tasks ran, eta, etc)
    show-summary-footer: true

    # show/hide the number of tasks that have failed in the summary footer line
    show-summary-errors: false

    # show/hide the number of tasks completed thus far on the summary footer line
    show-summary-steps: true

    # show/hide the eta and runtime figures on the summary footer line
    show-summary-times: false

    # globally enable/disable showing the stdout/stderr of each task
    show-task-output: true

    # Show an eta for each task on the screen (being shown on every line with a command running)
    show-task-times: true

    # globally enable/disable haulting further execution when any one task fails
    stop-on-failure: true

    # This is the character/string that is replaced with items listed in the 'for-each' block
    replica-replace-pattern: '<replace>'

    # time in milliseconds to update each task on the screen (polling interval)
    update-interval: 250
```

The `tasks` block is an ordered list of processes to run. Each task has several options that can be configured:
```yaml
tasks:
    - name: my awesome command      # a title for the task
      cmd: echo "woot"              # the command to be ran (required)
      collapse-on-completion: false # hide all defined 'parallel-tasks' after completion
      event-driven: true            # use a event driven or polling mechanism for displaying task stdout
      ignore-failure: false         # do not register any non-zero return code as a failure (this task will appear to never fail)
      show-output: true             # show task stdout to the screen
      stop-on-failure: true         # indicate if the application should continue if this cmd fails 
      parallel-tasks: ...           # a list of tasks that should be performed concurrently
      for-each: ...                 # a list of parameters used to duplicate this task
```

**There are a ton of examples in the `examples/` dir.** Go check them out!