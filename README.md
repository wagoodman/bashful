# bashful
...because your bash script should be quiet and shy-like (and not such a loud mouth). 

**Super alpha quality!** Use at your own risk.

![Image](demo.gif)

Use a yaml file to stich together commands and bash snippits and run them with style!

*"But why would you make this monstrosity?"* you ask...
because I could. And because ` &>/dev/null` or ` | tee -a some.log` or `set -e; do something; set +e` is getting annoying.

To run the example:
`docker-compose run app make`

**Features:**
- [x] Optionally run commands in parallel
- [x] Summary of the last line from stdout/stderr printed inline with commands
- [x] A shiny vertical progress bar
- [x] Optionally stop when a single command fails
- [ ] Configuration yaml block to control the behavior/look & feel
- [ ] Show detailed error reports when commands fail
- [ ] Bypass bashful all together and simply run each script/command in series
- [ ] Log all actions taken with all stdout/stderr